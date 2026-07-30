// Copyright 2026 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
							Enabled:     true,
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
							Enabled:     true,
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
							Enabled:     true,
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

		It("should set ClusterRoleBinding name based on users when users are present", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster",
					Namespace: "kamaji-system",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:     true,
							AdminUsers:  []string{"admin-user"},
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

			Expect(r.resource.Name).To(Equal("kamaji-my-cluster-admin-user"))
		})

		It("should set ClusterRoleBinding name based on groups when only groups are present", func() {
			tcp := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-cluster",
					Namespace: "kamaji-system",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					Bootstrap: &kamajiv1alpha1.BootstrapSpec{
						RBAC: &kamajiv1alpha1.RBACBootstrapSpec{
							Enabled:     true,
							AdminUsers:  []string{},
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

			Expect(r.resource.Name).To(Equal("kamaji-my-cluster-admin-group"))
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
							Enabled:     true,
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
							Enabled:     true,
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
							Enabled: false,
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
							Enabled: true,
						},
					},
				},
			}

			r := &RBACBootstrap{}
			Expect(r.ShouldCleanup(tcp)).To(BeFalse())
		})
	})
})
