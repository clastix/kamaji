// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestRBACBootstrap(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bootstrap RBAC Suite")
}

var _ = Describe("RBACBootstrap mutate", func() {
	Context("When building ClusterRoleBinding subjects", func() {
		It("should include users from AdminUsers", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-tcp",
					Namespace: "test-namespace",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:     ptr.To(true),
							AdminUsers:  []string{"user1", "user2"},
							AdminGroups: []string{},
						},
					},
				},
			}

			r := &RBACBootstrap{
				resource: &rbacv1.ClusterRoleBinding{},
			}

			mutateFn := r.mutate(tcp)

			err := mutateFn()
			Expect(err).NotTo(HaveOccurred())

			// Find user subjects
			var userSubjects []rbacv1.Subject
			for _, subject := range r.resource.Subjects {
				if subject.Kind == rbacv1.UserKind {
					userSubjects = append(userSubjects, subject)
				}
			}

			Expect(userSubjects).To(HaveLen(2))
			Expect(userSubjects[0].Name).To(Equal("user1"))
			Expect(userSubjects[1].Name).To(Equal("user2"))
		})

		It("should include groups from AdminGroups", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-tcp",
					Namespace: "test-namespace",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:     ptr.To(true),
							AdminUsers:  []string{},
							AdminGroups: []string{"group1", "group2"},
						},
					},
				},
			}

			r := &RBACBootstrap{
				resource: &rbacv1.ClusterRoleBinding{},
			}

			mutateFn := r.mutate(tcp)

			err := mutateFn()
			Expect(err).NotTo(HaveOccurred())

			// Find group subjects
			var groupSubjects []rbacv1.Subject
			for _, subject := range r.resource.Subjects {
				if subject.Kind == rbacv1.GroupKind {
					groupSubjects = append(groupSubjects, subject)
				}
			}

			Expect(groupSubjects).To(HaveLen(2))
			Expect(groupSubjects[0].Name).To(Equal("group1"))
			Expect(groupSubjects[1].Name).To(Equal("group2"))
		})

		It("should include both users and groups", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-tcp",
					Namespace: "test-namespace",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:     ptr.To(true),
							AdminUsers:  []string{"admin-user"},
							AdminGroups: []string{"system:masters"},
						},
					},
				},
			}

			r := &RBACBootstrap{
				resource: &rbacv1.ClusterRoleBinding{},
			}

			mutateFn := r.mutate(tcp)

			err := mutateFn()
			Expect(err).NotTo(HaveOccurred())

			Expect(r.resource.Subjects).To(HaveLen(2))
			Expect(r.resource.RoleRef.Name).To(Equal("cluster-admin"))
		})

		It("should not touch the object name, which CreateOrUpdate forbids", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster",
					Namespace: "kamaji-system",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:     ptr.To(true),
							AdminUsers:  []string{"admin-user"},
							AdminGroups: []string{},
						},
					},
				},
			}

			r := &RBACBootstrap{
				resource: &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName(tcp)},
				},
			}

			mutateFn := r.mutate(tcp)

			err := mutateFn()
			Expect(err).NotTo(HaveOccurred())

			Expect(r.resource.Name).To(Equal("kamaji-my-cluster-admin"))
		})

		It("should derive a name independent of the configured users and groups", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster",
					Namespace: "kamaji-system",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:     ptr.To(true),
							AdminUsers:  []string{},
							AdminGroups: []string{"system:masters"},
						},
					},
				},
			}

			Expect(clusterRoleBindingName(tcp)).To(Equal("kamaji-my-cluster-admin"))

			tcp.Spec.Bootstrap.RBAC.AdminUsers = []string{"admin-user"}
			tcp.Spec.Bootstrap.RBAC.AdminGroups = nil

			Expect(clusterRoleBindingName(tcp)).To(Equal("kamaji-my-cluster-admin"))
		})

		It("should set kamaji labels", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-ns",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:     ptr.To(true),
							AdminUsers:  []string{"admin"},
							AdminGroups: []string{},
						},
					},
				},
			}

			r := &RBACBootstrap{
				resource: &rbacv1.ClusterRoleBinding{},
			}

			mutateFn := r.mutate(tcp)

			err := mutateFn()
			Expect(err).NotTo(HaveOccurred())

			labels := r.resource.GetLabels()
			Expect(labels["kamaji.clastix.io/project"]).To(Equal("kamaji"))
			Expect(labels["kamaji.clastix.io/name"]).To(Equal("test-cluster"))
		})

		It("should set cluster-admin RoleRef", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "ns",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:     ptr.To(true),
							AdminUsers:  []string{"admin"},
							AdminGroups: []string{},
						},
					},
				},
			}

			r := &RBACBootstrap{
				resource: &rbacv1.ClusterRoleBinding{},
			}

			mutateFn := r.mutate(tcp)

			err := mutateFn()
			Expect(err).NotTo(HaveOccurred())

			Expect(r.resource.RoleRef.APIGroup).To(Equal(rbacv1.GroupName))
			Expect(r.resource.RoleRef.Kind).To(Equal("ClusterRole"))
			Expect(r.resource.RoleRef.Name).To(Equal("cluster-admin"))
		})
	})

	Context("ShouldCleanup", func() {
		It("should return true when Bootstrap is nil", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: nil,
				},
			}

			r := &RBACBootstrap{}
			Expect(r.ShouldCleanup(tcp)).To(BeTrue())
		})

		It("should return true when RBAC is nil", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: nil,
					},
				},
			}

			r := &RBACBootstrap{}
			Expect(r.ShouldCleanup(tcp)).To(BeTrue())
		})

		It("should return true when RBAC is disabled", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled: ptr.To(false),
						},
					},
				},
			}

			r := &RBACBootstrap{}
			Expect(r.ShouldCleanup(tcp)).To(BeTrue())
		})

		It("should return false when RBAC is enabled", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled: ptr.To(true),
						},
					},
				},
			}

			r := &RBACBootstrap{}
			Expect(r.ShouldCleanup(tcp)).To(BeFalse())
		})
	})
})

var _ = Describe("RBACBootstrap reconciliation", func() {
	var (
		tcp    *kamajiv1alpha1.TenantControlPlane
		tenant client.Client
		r      *RBACBootstrap
	)

	BeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(rbacv1.AddToScheme(scheme)).To(Succeed())

		tcp = &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "test-tcp", Namespace: "test-namespace"},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				Bootstrap: &kamajiv1alpha1.BootstrapSpec{
					RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
						AdminUsers:  []string{"kubernetes-admin"},
						AdminGroups: []string{"system:masters"},
					},
				},
			},
		}

		tenant = fake.NewClientBuilder().WithScheme(scheme).Build()
		// Mirrors what Define() sets up, without needing a live tenant kubeconfig.
		r = &RBACBootstrap{
			tenantClient: tenant,
			resource: &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{Name: clusterRoleBindingName(tcp)},
			},
		}
	})

	// Regression test: mutate() used to assign the object name, which
	// controllerutil.CreateOrUpdate rejects because it captures the object key
	// before invoking the MutateFn. Every reconcile failed with
	// "MutateFn cannot mutate object name and/or object namespace".
	It("should create the ClusterRoleBinding without mutating the object key", func() {
		op, err := r.CreateOrUpdate(context.Background(), tcp)
		Expect(err).NotTo(HaveOccurred())
		Expect(op).To(Equal(controllerutil.OperationResultCreated))

		var crb rbacv1.ClusterRoleBinding
		Expect(tenant.Get(context.Background(), types.NamespacedName{Name: "kamaji-test-tcp-admin"}, &crb)).To(Succeed())
		Expect(crb.RoleRef.Name).To(Equal("cluster-admin"))
		Expect(crb.Subjects).To(HaveLen(2))
	})

	It("should be idempotent across repeated reconciles", func() {
		_, err := r.CreateOrUpdate(context.Background(), tcp)
		Expect(err).NotTo(HaveOccurred())

		op, err := r.CreateOrUpdate(context.Background(), tcp)
		Expect(err).NotTo(HaveOccurred())
		Expect(op).To(Equal(controllerutil.OperationResultNone))
	})

	It("should keep a stable name when the configured admins change", func() {
		_, err := r.CreateOrUpdate(context.Background(), tcp)
		Expect(err).NotTo(HaveOccurred())

		// Dropping the users previously renamed the object to -admin-group,
		// orphaning the ClusterRoleBinding created on the first pass.
		tcp.Spec.Bootstrap.RBAC.AdminUsers = nil
		Expect(clusterRoleBindingName(tcp)).To(Equal("kamaji-test-tcp-admin"))

		op, err := r.CreateOrUpdate(context.Background(), tcp)
		Expect(err).NotTo(HaveOccurred())
		Expect(op).To(Equal(controllerutil.OperationResultUpdated))

		var list rbacv1.ClusterRoleBindingList
		Expect(tenant.List(context.Background(), &list)).To(Succeed())
		Expect(list.Items).To(HaveLen(1))
	})

	It("should treat an unset Enabled as enabled", func() {
		Expect(tcp.Spec.Bootstrap.RBAC.Enabled).To(BeNil())
		Expect(tcp.IsRBACBootstrapEnabled()).To(BeTrue())

		op, err := r.CreateOrUpdate(context.Background(), tcp)
		Expect(err).NotTo(HaveOccurred())
		Expect(op).To(Equal(controllerutil.OperationResultCreated))
	})

	It("should honour an explicit false so the feature can be disabled", func() {
		tcp.Spec.Bootstrap.RBAC.Enabled = ptr.To(false)
		Expect(tcp.IsRBACBootstrapEnabled()).To(BeFalse())

		op, err := r.CreateOrUpdate(context.Background(), tcp)
		Expect(err).NotTo(HaveOccurred())
		Expect(op).To(Equal(controllerutil.OperationResultNone))

		var list rbacv1.ClusterRoleBindingList
		Expect(tenant.List(context.Background(), &list)).To(Succeed())
		Expect(list.Items).To(BeEmpty())
	})

	// CleanUp resolved the placeholder name, so it never found the real object.
	It("should delete the ClusterRoleBinding it created", func() {
		_, err := r.CreateOrUpdate(context.Background(), tcp)
		Expect(err).NotTo(HaveOccurred())

		deleted, err := r.CleanUp(context.Background(), tcp)
		Expect(err).NotTo(HaveOccurred())
		Expect(deleted).To(BeTrue())

		var list rbacv1.ClusterRoleBindingList
		Expect(tenant.List(context.Background(), &list)).To(Succeed())
		Expect(list.Items).To(BeEmpty())
	})
})
