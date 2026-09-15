// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/clientcmd"
	pointer "k8s.io/utils/ptr"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/utilities"
)

var _ = Describe("Deploy a TenantControlPlane with RBAC bootstrap enabled", func() {
	var tcp *kamajiv1alpha1.TenantControlPlane

	// The object is filled here rather than at tree-construction time: GetKindIPAddress
	// queries the management cluster, which is not reachable until the suite has started.
	JustBeforeEach(func() {
		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tcp-rbac-bootstrap",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Deployment: kamajiv1alpha1.DeploymentSpec{
						Replicas: pointer.To(int32(1)),
					},
					Service: kamajiv1alpha1.ServiceSpec{
						ServiceType: "NodePort",
					},
				},
				// NodePort on the kind node address: the tenant API server has to be
				// reachable from the test process, and a ClusterIP on the default port
				// would resolve to the management cluster's own API server instead.
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
				Bootstrap: &kamajiv1alpha1.BootstrapSpec{
					RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
						AdminUsers:  []string{"kubernetes-admin"},
						AdminGroups: []string{"kamaji-e2e-admins"},
					},
				},
				Addons: kamajiv1alpha1.AddonsSpec{},
			},
		}

		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
	})

	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
	})

	It("Should create the admin ClusterRoleBinding in the Tenant cluster", func() {
		By("waiting for the Tenant Control Plane to be ready")
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

		By("getting a client for the Tenant cluster")
		tenantClient := tenantClientFor(tcp)

		By("waiting for the bootstrapped ClusterRoleBinding")
		crb := &rbacv1.ClusterRoleBinding{}

		Eventually(func() error {
			return tenantClient.Get(context.Background(), types.NamespacedName{Name: "kamaji-tcp-rbac-bootstrap-admin"}, crb)
		}, 5*time.Minute, time.Second).Should(Succeed())

		By("checking the binding grants cluster-admin to the declared subjects")
		Expect(crb.RoleRef.Kind).To(Equal("ClusterRole"))
		Expect(crb.RoleRef.Name).To(Equal("cluster-admin"))

		Expect(crb.Subjects).To(ContainElement(rbacv1.Subject{
			APIGroup: rbacv1.GroupName,
			Kind:     rbacv1.UserKind,
			Name:     "kubernetes-admin",
		}))
		Expect(crb.Subjects).To(ContainElement(rbacv1.Subject{
			APIGroup: rbacv1.GroupName,
			Kind:     rbacv1.GroupKind,
			Name:     "kamaji-e2e-admins",
		}))

		By("checking the Tenant Control Plane status reports the binding")
		Eventually(func() string {
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.GetName()}, tcp); err != nil {
				return ""
			}

			if tcp.Status.Bootstrap == nil || tcp.Status.Bootstrap.RBAC == nil {
				return ""
			}

			return tcp.Status.Bootstrap.RBAC.ClusterRoleBinding.Name
		}, time.Minute, time.Second).Should(Equal("kamaji-tcp-rbac-bootstrap-admin"))

		By("keeping the same binding when the declared admins change")
		// The object name is derived only from the Tenant Control Plane name: editing
		// the subjects must update the existing binding rather than create a second one.
		Eventually(func() error {
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.GetName()}, tcp); err != nil {
				return err
			}

			tcp.Spec.Bootstrap.RBAC.AdminGroups = []string{"kamaji-e2e-other-admins"}

			return k8sClient.Update(context.Background(), tcp)
		}, time.Minute, time.Second).ShouldNot(HaveOccurred())

		Eventually(func() []rbacv1.Subject {
			if err := tenantClient.Get(context.Background(), types.NamespacedName{Name: "kamaji-tcp-rbac-bootstrap-admin"}, crb); err != nil {
				return nil
			}

			return crb.Subjects
		}, 2*time.Minute, time.Second).Should(ContainElement(rbacv1.Subject{
			APIGroup: rbacv1.GroupName,
			Kind:     rbacv1.GroupKind,
			Name:     "kamaji-e2e-other-admins",
		}))

		// Scoped to the component label as well: Kamaji creates other ClusterRoleBindings
		// in the Tenant cluster (kubeadm phases, and the addons when enabled) that carry
		// the same Tenant Control Plane name label.
		list := &rbacv1.ClusterRoleBindingList{}
		Expect(tenantClient.List(context.Background(), list, ctrlclient.MatchingLabels{
			constants.ControlPlaneLabelKey:      tcp.GetName(),
			constants.ControlPlaneLabelResource: "rbac-bootstrap-clusterrolebinding",
		})).To(Succeed())
		Expect(list.Items).To(HaveLen(1))

		By("removing the binding when RBAC bootstrap is disabled")
		Eventually(func() error {
			if err := k8sClient.Get(context.Background(), types.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.GetName()}, tcp); err != nil {
				return err
			}

			tcp.Spec.Bootstrap.RBAC.Enabled = pointer.To(false)

			return k8sClient.Update(context.Background(), tcp)
		}, time.Minute, time.Second).ShouldNot(HaveOccurred())

		Eventually(func() bool {
			err := tenantClient.Get(context.Background(), types.NamespacedName{Name: "kamaji-tcp-rbac-bootstrap-admin"}, crb)

			return k8serrors.IsNotFound(err)
		}, 2*time.Minute, time.Second).Should(BeTrue())
	})
})

// tenantClientFor returns a controller-runtime client connected to the Tenant cluster
// backing the given Tenant Control Plane.
func tenantClientFor(tcp *kamajiv1alpha1.TenantControlPlane) ctrlclient.Client {
	config, err := utilities.GetTenantKubeconfig(context.Background(), k8sClient, tcp)
	Expect(err).ToNot(HaveOccurred())

	b, err := utilities.EncodeToYaml(config)
	Expect(err).ToNot(HaveOccurred())

	clientCfg, err := clientcmd.NewClientConfigFromBytes(b)
	Expect(err).ToNot(HaveOccurred())

	restConfig, err := clientCfg.ClientConfig()
	Expect(err).ToNot(HaveOccurred())

	tenantClient, err := ctrlclient.New(restConfig, ctrlclient.Options{})
	Expect(err).ToNot(HaveOccurred())

	return tenantClient
}
