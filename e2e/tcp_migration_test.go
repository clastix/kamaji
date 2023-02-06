// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

var _ = Describe("When migrating a Tenant Control Plane to another datastore", func() {
	var tcp *kamajiv1alpha1.TenantControlPlane
	// Create a TenantControlPlane resource into the cluster
	JustBeforeEach(func() {
		// Fill TenantControlPlane object
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("migrating-%s-etcd", rand.String(5)),
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				DataStore: "etcd-bronze",
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Deployment: kamajiv1alpha1.DeploymentSpec{
						Replicas: 1,
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
					Version: "v1.23.6",
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

		By("start migration to a new DataStore")
		Eventually(func() error {
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.GetName()}, tcp); err != nil {
				return err
			}

			tcp.Spec.DataStore = "etcd-silver"

			return k8sClient.Update(context.Background(), tcp)
		}, time.Minute, time.Second).ShouldNot(HaveOccurred())

		By("waiting for the migrating status")
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionMigrating)

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
		}, time.Minute, time.Second).Should(BeEquivalentTo("etcd-silver"))

		By("checking the presence of the previous Namespace")
		Eventually(func() error {
			return tcpClient.Get(context.Background(), types.NamespacedName{Name: ns.GetName()}, &corev1.Namespace{})
		}).ShouldNot(HaveOccurred())
	})
})
