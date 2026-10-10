// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
)

// kubeadmUtilsTestScheme returns a scheme that knows the types used by
// GetKubeadmManifestDeps.
func kubeadmUtilsTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding v1alpha1 to scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding corev1 to scheme: %v", err)
	}

	return scheme
}

// TestGetKubeadmManifestDepsCoreDNSImageTag verifies that setting only the
// CoreDNS ImageTag (leaving the repository empty) is honoured: the resulting
// kubeadm configuration must carry the tag in CoreDNSOptions.
func TestGetKubeadmManifestDepsCoreDNSImageTag(t *testing.T) {
	tcp := &v1alpha1.TenantControlPlane{}
	tcp.SetName("test-tcp")
	tcp.SetNamespace("default")
	tcp.Spec.Kubernetes.Version = "v1.28.0"
	tcp.Spec.NetworkProfile.PodCIDR = "10.244.0.0/16"
	tcp.Spec.NetworkProfile.ServiceCIDR = "10.96.0.0/12"
	tcp.Spec.NetworkProfile.DNSServiceIPs = []string{"10.96.0.10"}
	tcp.Spec.NetworkProfile.CertSANs = []string{"test-tcp.default.svc"}
	tcp.Spec.NetworkProfile.Port = 6443
	tcp.Spec.Kubernetes.Kubelet.CGroupFS = "cgroupfs"
	tcp.Status.ControlPlaneEndpoint = "1.2.3.4:6443"
	tcp.Status.KubeadmConfig.ConfigmapName = "test-tcp-kubeadm-config"
	tcp.Status.KubeConfig.Admin.SecretName = "test-tcp-admin-conf"

	// Only the tag is set: the bug silently drops it because the guard
	// wrongly checks ImageRepository.
	tcp.Spec.Addons.CoreDNS = &v1alpha1.AddonSpec{}
	tcp.Spec.Addons.CoreDNS.ImageTag = "v1.11.0"

	stored, err := kubeadm.CreateKubeadmInitConfiguration(kubeadm.Parameters{
		TenantControlPlaneName:          tcp.GetName(),
		TenantControlPlaneNamespace:     tcp.GetNamespace(),
		TenantControlPlaneEndpoint:      "1.2.3.4:6443",
		TenantControlPlaneAddress:       "1.2.3.4",
		TenantControlPlaneCertSANs:      []string{"test-tcp.default.svc"},
		TenantControlPlanePort:          6443,
		TenantControlPlaneClusterDomain: "cluster.local",
		TenantControlPlanePodCIDR:       []string{"10.244.0.0/16"},
		TenantControlPlaneServiceCIDR:   []string{"10.96.0.0/12"},
		TenantDNSServiceIPs:             []string{"10.96.0.10"},
		TenantControlPlaneVersion:       "v1.28.0",
		TenantControlPlaneCGroupDriver:  "cgroupfs",
		ETCDs:                           []string{"https://etcd.default.svc:2379"},
	})
	if err != nil {
		t.Fatalf("creating stored kubeadm configuration: %v", err)
	}
	data, err := kubeadm.GetKubeadmInitConfigurationMap(*stored)
	if err != nil {
		t.Fatalf("mapping stored kubeadm configuration: %v", err)
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tcp.GetNamespace(),
			Name:      tcp.Status.KubeadmConfig.ConfigmapName,
		},
		Data: data,
	}

	// A well-formed kubeconfig that clientcmd can parse; it is never dialled.
	kubeconfig := &clientcmdapiv1.Config{
		APIVersion:     "v1",
		Kind:           "Config",
		CurrentContext: "test",
		Clusters:       []clientcmdapiv1.NamedCluster{{Name: "test", Cluster: clientcmdapiv1.Cluster{Server: "https://127.0.0.1:6443"}}},
		Contexts:       []clientcmdapiv1.NamedContext{{Name: "test", Context: clientcmdapiv1.Context{Cluster: "test", AuthInfo: "test"}}},
		AuthInfos:      []clientcmdapiv1.NamedAuthInfo{{Name: "test", AuthInfo: clientcmdapiv1.AuthInfo{}}},
	}
	raw, err := yaml.Marshal(kubeconfig)
	if err != nil {
		t.Fatalf("marshalling kubeconfig: %v", err)
	}
	kubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: tcp.GetNamespace(),
			Name:      tcp.Status.KubeConfig.Admin.SecretName,
		},
		Data: map[string][]byte{
			kubeadmconstants.SuperAdminKubeConfigFileName: raw,
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(kubeadmUtilsTestScheme(t)).
		WithObjects(configMap, kubeconfigSecret).
		Build()

	ctx := context.Background()
	_, config, err := GetKubeadmManifestDeps(ctx, cl, tcp)
	if err != nil {
		t.Fatalf("GetKubeadmManifestDeps: %v", err)
	}
	opts := config.Parameters.CoreDNSOptions
	if opts == nil {
		t.Fatalf("expected CoreDNSOptions to be set, got nil")
	}
	if opts.Tag != "v1.11.0" {
		t.Errorf("expected CoreDNSOptions.Tag to be %q, got %q (custom ImageTag was dropped)", "v1.11.0", opts.Tag)
	}
}
