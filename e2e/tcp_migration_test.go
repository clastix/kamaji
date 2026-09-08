// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	pointer "k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	ds "github.com/clastix/kamaji/internal/resources/datastore"
	"github.com/clastix/kamaji/internal/utilities"
)

// Seed size for the migration: enough objects to slow etcd's per-key Put loop, and enough
// bytes to slow PostgreSQL's bulk COPY. Tunable.
const (
	migrationSeedObjects      = 400
	migrationSeedPayloadBytes = 128 * 1024
	migrationSeedConcurrency  = 10
)

// migrationSeedBackoff outlasts the multi-second API server stalls that retry.DefaultBackoff does not.
var migrationSeedBackoff = wait.Backoff{
	Duration: 100 * time.Millisecond,
	Factor:   2.0,
	Jitter:   0.1,
	Steps:    6,
}

// seedMigrationData fills the tenant cluster with ConfigMaps, returning their names.
func seedMigrationData(ctx context.Context, tcpClient ctrlclient.Client, namespace string) []string {
	GinkgoHelper()

	payload := strings.Repeat("k", migrationSeedPayloadBytes)

	names := make([]string, 0, migrationSeedObjects)
	for i := range migrationSeedObjects {
		names = append(names, fmt.Sprintf("migration-seed-%04d", i))
	}

	work := make(chan string)
	errs := make(chan error, migrationSeedObjects)

	var wg sync.WaitGroup

	for range migrationSeedConcurrency {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for name := range work {
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
					Data:       map[string]string{"name": name, "payload": payload},
				}
				// The API server is contended: a create can need more than one attempt.
				if err := retry.OnError(migrationSeedBackoff, func(error) bool { return true }, func() error {
					return ctrlclient.IgnoreAlreadyExists(tcpClient.Create(ctx, cm))
				}); err != nil {
					errs <- fmt.Errorf("unable to seed ConfigMap %s/%s: %w", namespace, name, err)
				}
			}
		}()
	}

	for _, name := range names {
		work <- name
	}

	close(work)
	wg.Wait()
	close(errs)

	for err := range errs {
		Expect(err).ToNot(HaveOccurred())
	}

	return names
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

		ns := &corev1.Namespace{}
		ns.SetName("kamaji-test")
		Expect(tcpClient.Create(context.Background(), ns)).ToNot(HaveOccurred())

		By("seeding the source DataStore so the migration takes observable time")
		seeded := seedMigrationData(context.Background(), tcpClient, ns.GetName())

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
		// The webhook only exists while the migration Job runs: once it completes the API
		// server is repointed at the target DataStore and the webhook, being tenant data,
		// is gone. So this is a window to sample, not a state to wait for, and a longer
		// timeout cannot help once it has shut. The counters separate samples that reached
		// the API server from those lost to it being starved.
		var reachable, unreachable int

		Eventually(func() error {
			err := tcpClient.Get(context.Background(), types.NamespacedName{Name: ds.FreezeWebhookName}, &admissionregistrationv1.ValidatingWebhookConfiguration{})

			switch {
			case err == nil || apierrors.IsNotFound(err):
				reachable++
			default:
				unreachable++
			}

			return err
		}, 5*time.Minute, 500*time.Millisecond).Should(Succeed(), func() string {
			return fmt.Sprintf("never observed %s in the tenant cluster: %d samples reached the API server, %d were lost to it being unreachable", ds.FreezeWebhookName, reachable, unreachable)
		})

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

		By("checking every seeded object survived the migration")
		// Makes this spec assert its own name, rather than just the one Namespace.
		Eventually(func() error {
			for _, name := range seeded {
				if err := tcpClient.Get(context.Background(), types.NamespacedName{Namespace: ns.GetName(), Name: name}, &corev1.ConfigMap{}); err != nil {
					return fmt.Errorf("seeded ConfigMap %s/%s did not survive the migration: %w", ns.GetName(), name, err)
				}
			}

			return nil
		}, 5*time.Minute, time.Second).Should(Succeed())
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
