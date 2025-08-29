// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

// TenantClusterValidator provides validation utilities for tenant cluster resources
type TenantClusterValidator struct {
	tenantClient kubernetes.Interface
	tcp          *kamajiv1alpha1.TenantControlPlane
}

// NewTenantClusterValidator creates a new validator for the given TenantControlPlane
func NewTenantClusterValidator(tcp *kamajiv1alpha1.TenantControlPlane) (*TenantClusterValidator, error) {
	GinkgoHelper()

	// Get tenant kubeconfig
	kubeconfigSecret := &corev1.Secret{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{
		Name:      fmt.Sprintf("%s-admin-kubeconfig", tcp.GetName()),
		Namespace: tcp.GetNamespace(),
	}, kubeconfigSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant kubeconfig secret: %w", err)
	}

	kubeconfigData, exists := kubeconfigSecret.Data["admin.conf"]
	if !exists {
		return nil, fmt.Errorf("admin.conf not found in kubeconfig secret")
	}

	// Create temporary kubeconfig file
	kubeconfigFile, err := os.CreateTemp("", "tenant-kubeconfig-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary kubeconfig file: %w", err)
	}
	defer kubeconfigFile.Close()

	if _, err := kubeconfigFile.Write(kubeconfigData); err != nil {
		return nil, fmt.Errorf("failed to write kubeconfig data: %w", err)
	}

	// Build client config from kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	// Create tenant client
	tenantClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create tenant client: %w", err)
	}

	// Clean up temporary file
	os.Remove(kubeconfigFile.Name())

	return &TenantClusterValidator{
		tenantClient: tenantClient,
		tcp:          tcp,
	}, nil
}

// ValidateClusterAdminRBAC validates that cluster-admin RBAC bindings exist
func (v *TenantClusterValidator) ValidateClusterAdminRBAC() {
	GinkgoHelper()

	ctx := context.Background()

	By("validating cluster-admin RBAC bindings exist", func() {
		// Check if RBAC bootstrap is enabled (default: true)
		if v.tcp.Spec.Bootstrap != nil && v.tcp.Spec.Bootstrap.RBAC != nil && !v.tcp.Spec.Bootstrap.RBAC.Enabled {
			Skip("RBAC bootstrap is disabled")
		}

		// Validate admin users ClusterRoleBinding
		Eventually(func() error {
			_, err := v.tenantClient.RbacV1().ClusterRoleBindings().Get(ctx, "kamaji-bootstrap-admin-users", metav1.GetOptions{})
			return err
		}, 2*time.Minute, 5*time.Second).Should(Succeed(), "kamaji-bootstrap-admin-users ClusterRoleBinding should exist")

		// Validate admin groups ClusterRoleBinding if admin groups are specified
		if v.tcp.Spec.Bootstrap != nil && v.tcp.Spec.Bootstrap.RBAC != nil && len(v.tcp.Spec.Bootstrap.RBAC.AdminGroups) > 0 {
			Eventually(func() error {
				_, err := v.tenantClient.RbacV1().ClusterRoleBindings().Get(ctx, "kamaji-bootstrap-admin-groups", metav1.GetOptions{})
				return err
			}, 2*time.Minute, 5*time.Second).Should(Succeed(), "kamaji-bootstrap-admin-groups ClusterRoleBinding should exist")
		}
	})

	By("validating cluster-admin ClusterRole exists", func() {
		Eventually(func() error {
			_, err := v.tenantClient.RbacV1().ClusterRoles().Get(ctx, "cluster-admin", metav1.GetOptions{})
			return err
		}, 2*time.Minute, 5*time.Second).Should(Succeed(), "cluster-admin ClusterRole should exist")
	})

	By("validating RBAC bindings reference cluster-admin role", func() {
		Eventually(func() bool {
			crb, err := v.tenantClient.RbacV1().ClusterRoleBindings().Get(ctx, "kamaji-bootstrap-admin-users", metav1.GetOptions{})
			if err != nil {
				return false
			}
			return crb.RoleRef.Name == "cluster-admin"
		}, 2*time.Minute, 5*time.Second).Should(BeTrue(), "ClusterRoleBinding should reference cluster-admin role")
	})
}

// ValidateCoreDNS validates that CoreDNS addon resources exist when enabled
func (v *TenantClusterValidator) ValidateCoreDNS() {
	GinkgoHelper()

	ctx := context.Background()

	if v.tcp.Spec.Addons.CoreDNS == nil {
		Skip("CoreDNS addon is not enabled")
	}

	By("validating kube-dns service exists", func() {
		Eventually(func() error {
			_, err := v.tenantClient.CoreV1().Services("kube-system").Get(ctx, "kube-dns", metav1.GetOptions{})
			return err
		}, 3*time.Minute, 5*time.Second).Should(Succeed(), "kube-dns service should exist in kube-system namespace")
	})

	By("validating CoreDNS deployment exists and is ready", func() {
		Eventually(func() bool {
			deployment, err := v.tenantClient.AppsV1().Deployments("kube-system").Get(ctx, "coredns", metav1.GetOptions{})
			if err != nil {
				return false
			}
			return deployment.Status.ReadyReplicas > 0
		}, 5*time.Minute, 10*time.Second).Should(BeTrue(), "CoreDNS deployment should exist and have ready replicas")
	})

	By("validating CoreDNS ConfigMap exists", func() {
		Eventually(func() error {
			_, err := v.tenantClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "coredns", metav1.GetOptions{})
			return err
		}, 2*time.Minute, 5*time.Second).Should(Succeed(), "coredns ConfigMap should exist")
	})

	By("validating CoreDNS ServiceAccount exists", func() {
		Eventually(func() error {
			_, err := v.tenantClient.CoreV1().ServiceAccounts("kube-system").Get(ctx, "coredns", metav1.GetOptions{})
			return err
		}, 2*time.Minute, 5*time.Second).Should(Succeed(), "coredns ServiceAccount should exist")
	})

	By("validating CoreDNS RBAC resources exist", func() {
		Eventually(func() error {
			_, err := v.tenantClient.RbacV1().ClusterRoles().Get(ctx, "system:coredns", metav1.GetOptions{})
			return err
		}, 2*time.Minute, 5*time.Second).Should(Succeed(), "system:coredns ClusterRole should exist")

		Eventually(func() error {
			_, err := v.tenantClient.RbacV1().ClusterRoleBindings().Get(ctx, "system:coredns", metav1.GetOptions{})
			return err
		}, 2*time.Minute, 5*time.Second).Should(Succeed(), "system:coredns ClusterRoleBinding should exist")
	})
}

// ValidateKubeProxy validates that kube-proxy addon resources exist when enabled
func (v *TenantClusterValidator) ValidateKubeProxy() {
	GinkgoHelper()

	ctx := context.Background()

	if v.tcp.Spec.Addons.KubeProxy == nil {
		Skip("kube-proxy addon is not enabled")
	}

	By("validating kube-proxy DaemonSet exists", func() {
		Eventually(func() error {
			_, err := v.tenantClient.AppsV1().DaemonSets("kube-system").Get(ctx, "kube-proxy", metav1.GetOptions{})
			return err
		}, 3*time.Minute, 5*time.Second).Should(Succeed(), "kube-proxy DaemonSet should exist")
	})

	By("validating kube-proxy ConfigMap exists", func() {
		Eventually(func() error {
			_, err := v.tenantClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "kube-proxy", metav1.GetOptions{})
			return err
		}, 2*time.Minute, 5*time.Second).Should(Succeed(), "kube-proxy ConfigMap should exist")
	})

	By("validating kube-proxy ServiceAccount exists", func() {
		Eventually(func() error {
			_, err := v.tenantClient.CoreV1().ServiceAccounts("kube-system").Get(ctx, "kube-proxy", metav1.GetOptions{})
			return err
		}, 2*time.Minute, 5*time.Second).Should(Succeed(), "kube-proxy ServiceAccount should exist")
	})

	By("validating kube-proxy RBAC resources exist", func() {
		Eventually(func() error {
			_, err := v.tenantClient.RbacV1().ClusterRoleBindings().Get(ctx, "kubeadm:node-proxier", metav1.GetOptions{})
			return err
		}, 2*time.Minute, 5*time.Second).Should(Succeed(), "kubeadm:node-proxier ClusterRoleBinding should exist")
	})
}

// ValidateStandardKubernetesResources validates standard Kubernetes bootstrap resources
func (v *TenantClusterValidator) ValidateStandardKubernetesResources() {
	GinkgoHelper()

	ctx := context.Background()

	By("validating kube-system namespace exists", func() {
		Eventually(func() error {
			_, err := v.tenantClient.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{})
			return err
		}, 1*time.Minute, 5*time.Second).Should(Succeed(), "kube-system namespace should exist")
	})

	By("validating default namespace exists", func() {
		Eventually(func() error {
			_, err := v.tenantClient.CoreV1().Namespaces().Get(ctx, "default", metav1.GetOptions{})
			return err
		}, 1*time.Minute, 5*time.Second).Should(Succeed(), "default namespace should exist")
	})

	By("validating kubernetes service exists", func() {
		Eventually(func() error {
			_, err := v.tenantClient.CoreV1().Services("default").Get(ctx, "kubernetes", metav1.GetOptions{})
			return err
		}, 1*time.Minute, 5*time.Second).Should(Succeed(), "kubernetes service should exist in default namespace")
	})

	By("validating system ClusterRoles exist", func() {
		systemRoles := []string{
			"cluster-admin",
			"admin",
			"edit",
			"view",
			"system:node",
		}

		for _, role := range systemRoles {
			role := role // capture for closure
			Eventually(func() error {
				_, err := v.tenantClient.RbacV1().ClusterRoles().Get(ctx, role, metav1.GetOptions{})
				return err
			}, 2*time.Minute, 5*time.Second).Should(Succeed(), fmt.Sprintf("%s ClusterRole should exist", role))
		}
	})
}

// ValidateClusterHealth performs basic cluster health checks
func (v *TenantClusterValidator) ValidateClusterHealth() {
	GinkgoHelper()

	ctx := context.Background()

	By("validating API server is responsive", func() {
		Eventually(func() error {
			_, err := v.tenantClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
			return err
		}, 2*time.Minute, 5*time.Second).Should(Succeed(), "API server should be responsive")
	})

	By("validating cluster info is accessible", func() {
		Eventually(func() error {
			_, err := v.tenantClient.Discovery().ServerVersion()
			return err
		}, 1*time.Minute, 5*time.Second).Should(Succeed(), "cluster version info should be accessible")
	})
}

// ValidateAllResources runs all validation checks
func (v *TenantClusterValidator) ValidateAllResources() {
	GinkgoHelper()

	v.ValidateClusterHealth()
	v.ValidateStandardKubernetesResources()
	v.ValidateClusterAdminRBAC()
	v.ValidateCoreDNS()
	v.ValidateKubeProxy()
}

// TenantClusterResourcesMustBeValid validates that all expected resources exist in the tenant cluster
func TenantClusterResourcesMustBeValid(tcp *kamajiv1alpha1.TenantControlPlane) {
	GinkgoHelper()

	// First ensure the TenantControlPlane is ready
	StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

	// Create validator and run all checks
	validator, err := NewTenantClusterValidator(tcp)
	Expect(err).NotTo(HaveOccurred(), "should be able to create tenant cluster validator")

	validator.ValidateAllResources()
}
