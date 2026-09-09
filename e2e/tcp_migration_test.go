// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	pointer "k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	ds "github.com/clastix/kamaji/internal/resources/datastore"
	"github.com/clastix/kamaji/internal/utilities"
)

// migrationFixtures is kept small on purpose: objects written here land in the Kamaji
// manager's soot cache, which shares that process's 100Mi limit.
const migrationFixtures = 5

// awaitFreezeWebhook drains freeze webhook events until the object is seen. The watch must
// already be open before the migration is triggered: the webhook exists only while the
// migration Job runs, and polling for it loses the race whenever the tenant API server is
// contended, which on a 2-vCPU runner is most of that window.
func awaitFreezeWebhook(w watch.Interface, timeout time.Duration) error {
	deadline := time.After(timeout)

	for {
		select {
		case event, open := <-w.ResultChan():
			if !open {
				return fmt.Errorf("watch closed before %s was observed", ds.FreezeWebhookName)
			}

			if event.Type == watch.Added || event.Type == watch.Modified {
				return nil
			}
		case <-deadline:
			return fmt.Errorf("timed out waiting for %s", ds.FreezeWebhookName)
		}
	}
}

func featureTestMigration(driver string) {
	var tcp *kamajiv1alpha1.TenantControlPlane
	// Create a TenantControlPlane resource into the cluster
	JustBeforeEach(func() {
		// Fill TenantControlPlane object
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("migrating-%s-%s", rand.String(5), driver),
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				DataStore: fmt.Sprintf("%s-bronze", driver),
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Deployment: kamajiv1alpha1.DeploymentSpec{
						Replicas: pointer.To(int32(1)),
					},
					Service: kamajiv1alpha1.ServiceSpec{
						ServiceType: "NodePort",
					},
				},
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
					Address: GetKindIPAddress(),
					Port:    int32(rand.Int63nRange(31000, 32000)),
				},
				Kubernetes: kamajiv1alpha1.KubernetesSpec{
					Version: kamajiv1alpha1.DefaultKubernetesVersion,
					Kubelet: kamajiv1alpha1.KubeletSpec{
						CGroupFS: "cgroupfs",
					},
				},
			},
		}
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
	})
	// Delete the TenantControlPlane resource after test is finished
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
	})
	// Check if TenantControlPlane resource has been created
	It("Should contain all the migrated data", func() {
		time.Sleep(10 * time.Second)

		By("getting TCP rest.Config")
		config, err := utilities.GetTenantKubeconfig(context.Background(), k8sClient, tcp)
		Expect(err).ToNot(HaveOccurred())

		b, err := utilities.EncodeToYaml(config)
		Expect(err).ToNot(HaveOccurred())

		clientCfg, err := clientcmd.NewClientConfigFromBytes(b)
		Expect(err).ToNot(HaveOccurred())

		restConfig, err := clientCfg.ClientConfig()
		Expect(err).ToNot(HaveOccurred())

		tcpClient, err := ctrlclient.New(restConfig, ctrlclient.Options{})
		Expect(err).ToNot(HaveOccurred())

		tcpClientset, err := kubernetes.NewForConfig(restConfig)
		Expect(err).ToNot(HaveOccurred())

		ns := &corev1.Namespace{}
		ns.SetName("kamaji-test")
		Expect(tcpClient.Create(context.Background(), ns)).ToNot(HaveOccurred())

		By("writing fixtures the migration has to carry across")
		fixtures := make([]string, 0, migrationFixtures)

		for i := range migrationFixtures {
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("migration-fixture-%d", i), Namespace: ns.GetName()},
				Data:       map[string]string{"payload": rand.String(256)},
			}
			Expect(tcpClient.Create(context.Background(), cm)).ToNot(HaveOccurred())

			fixtures = append(fixtures, cm.GetName())
		}

		By("opening a watch on the freeze webhook before the migration can install it")
		freezeWatch, err := tcpClientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Watch(context.Background(), metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", ds.FreezeWebhookName).String(),
		})
		Expect(err).ToNot(HaveOccurred())

		defer freezeWatch.Stop()

		By("start migration to a new DataStore")
		Eventually(func() error {
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.GetName()}, tcp); err != nil {
				return err
			}

			tcp.Spec.DataStore = fmt.Sprintf("%s-silver", driver)

			return k8sClient.Update(context.Background(), tcp)
		}, time.Minute, time.Second).ShouldNot(HaveOccurred())

		By("waiting for the migrating status")
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionMigrating)

		By("waiting for the webhook installation")
		// The webhook is pushed to the watch opened above the instant Kamaji installs it,
		// so a contended API server delays the event rather than hiding it. If the watch
		// drops - the API server is restarted at the end of the migration - fall back to
		// a direct read, which still succeeds while the window is open.
		if err := awaitFreezeWebhook(freezeWatch, 5*time.Minute); err != nil {
			Eventually(func() error {
				return tcpClient.Get(context.Background(), types.NamespacedName{Name: ds.FreezeWebhookName}, &admissionregistrationv1.ValidatingWebhookConfiguration{})
			}, time.Minute, time.Second).Should(Succeed(), "%s never observed: %s", ds.FreezeWebhookName, err)
		}

		By("ensuring changes are not allowed")
		Consistently(func() error {
			return tcpClient.Delete(context.Background(), ns)
		}, 10*time.Second, time.Second).Should(HaveOccurred())

		By("waiting for completion of migration")
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

		By("checking the DataStore of the TCP")
		Eventually(func() string {
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: tcp.GetName(), Namespace: tcp.GetNamespace()}, tcp); err != nil {
				return ""
			}

			return tcp.Status.Storage.DataStoreName
		}, time.Minute, time.Second).Should(BeEquivalentTo(fmt.Sprintf("%s-silver", driver)))

		By("checking the presence of the previous Namespace")
		Eventually(func() error {
			return tcpClient.Get(context.Background(), types.NamespacedName{Name: ns.GetName()}, &corev1.Namespace{})
		}).ShouldNot(HaveOccurred())

		By("checking every fixture survived the migration")
		// Makes this spec assert its own name, rather than just the one Namespace.
		Eventually(func() error {
			for _, name := range fixtures {
				if err := tcpClient.Get(context.Background(), types.NamespacedName{Namespace: ns.GetName(), Name: name}, &corev1.ConfigMap{}); err != nil {
					return err
				}
			}

			return nil
		}, time.Minute, time.Second).Should(Succeed())
		// The Freeze ValidatingWebhookConfiguration should have been removed successfully:
		// we're checking write operations are allowed.
		By("checking the changes are newly allowed")
		Eventually(func() error {
			var writeNamespace corev1.Namespace
			writeNamespace.Name = fmt.Sprintf("write-%s-%s", rand.String(5), driver)

			return tcpClient.Create(context.Background(), &writeNamespace)
		}).ShouldNot(HaveOccurred())
	})
}

var _ = Describe("When migrating a Tenant Control Plane to another datastore (etcd)", func() {
	featureTestMigration("etcd")
})

var _ = Describe("When migrating a Tenant Control Plane to another datastore (postgresql)", func() {
	featureTestMigration("postgresql")
})
