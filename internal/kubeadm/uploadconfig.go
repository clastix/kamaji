// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubelettypes "k8s.io/kubelet/config/v1beta1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/uploadconfig"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	"k8s.io/kubernetes/pkg/apis/rbac"
	kubeletv1beta1 "k8s.io/kubernetes/pkg/kubelet/apis/config/v1beta1"

	"github.com/clastix/kamaji/internal/utilities"
)

const (
	// kubeletConfigMapName defines base kubelet configuration ConfigMap name for kubeadm < 1.24.
	kubeletConfigMapName = "kubelet-config-%d.%d"
)

// minVerUnversionedKubeletConfig defines minimum version from which kubeadm uses kubelet-config as a ConfigMap name.
var minVerUnversionedKubeletConfig = semver.MustParse("1.24.0")

func UploadKubeadmConfig(client kubernetes.Interface, config *Configuration) ([]byte, error) {
	return nil, uploadconfig.UploadConfiguration(&config.InitConfiguration, client)
}

func UploadKubeletConfig(client kubernetes.Interface, config *Configuration) ([]byte, error) {
	kubeletConfiguration := KubeletConfiguration{
		TenantControlPlaneDomain:        config.InitConfiguration.Networking.DNSDomain,
		TenantControlPlaneDNSServiceIPs: config.Parameters.TenantDNSServiceIPs,
		TenantControlPlaneCgroupDriver:  config.Parameters.TenantControlPlaneCGroupDriver,
		FeatureGates:                    config.Parameters.KubeletFeatureGates,
	}
	content, err := getKubeletConfigmapContent(kubeletConfiguration)
	if err != nil {
		return nil, err
	}

	configMapName, err := generateKubeletConfigMapName(config.Parameters.TenantControlPlaneVersion)
	if err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string]string{
			kubeadmconstants.KubeletBaseConfigurationConfigMapKey: string(content),
		},
	}

	if err = apiclient.CreateOrUpdate[*corev1.ConfigMap](client.CoreV1().ConfigMaps(metav1.NamespaceSystem), configMap); err != nil {
		return nil, err
	}

	if err = createConfigMapRBACRules(client, configMapName); err != nil {
		return nil, errors.Wrap(err, "error creating kubelet configuration configmap RBAC rules")
	}

	return nil, nil
}

func getKubeletConfigmapContent(kubeletConfiguration KubeletConfiguration) ([]byte, error) {
	var kc kubelettypes.KubeletConfiguration

	kc.FeatureGates = kubeletConfiguration.FeatureGates
	kubeletv1beta1.SetDefaults_KubeletConfiguration(&kc)

	kc.APIVersion = kubeletv1beta1.SchemeGroupVersion.String()
	kc.Kind = "KubeletConfiguration"
	kc.Authentication.X509.ClientCAFile = "/etc/kubernetes/pki/ca.crt"
	kc.CgroupDriver = kubeletConfiguration.TenantControlPlaneCgroupDriver
	kc.ClusterDNS = kubeletConfiguration.TenantControlPlaneDNSServiceIPs
	kc.ClusterDomain = kubeletConfiguration.TenantControlPlaneDomain
	kc.RotateCertificates = true
	kc.StaticPodPath = "/etc/kubernetes/manifests"
	// TODO(prometherion): drop support of <= v1.27 TCP versions
	// a numeric value is required due to strict marshaling
	// kubeadm <= v1.27 has a different type for FlushFrequency
	// https://github.com/kubernetes/component-base/blob/55b3ab0db0081303695d641b9b43d560bf3f7a65/logs/api/v1/types.go#L42-L45
	kc.Logging.FlushFrequency.SerializeAsString = false
	// Restore default behaviour so Kubelet will automatically
	// determine the resolvConf location, as reported in clastix/kamaji#581.
	kc.ResolverConfig = nil

	return utilities.EncodeToYaml(&kc)
}

func createConfigMapRBACRules(client kubernetes.Interface, configMapName string) error {
	configMapRBACName := kubeadmconstants.KubeletBaseConfigMapRole

	if err := apiclient.CreateOrUpdate[*rbacv1.Role](client.RbacV1().Roles(metav1.NamespaceSystem), &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapRBACName,
			Namespace: metav1.NamespaceSystem,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get"},
				APIGroups:     []string{""},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{configMapName},
			},
		},
	}); err != nil {
		return err
	}

	return apiclient.CreateOrUpdate[*rbacv1.RoleBinding](client.RbacV1().RoleBindings(metav1.NamespaceSystem), &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapRBACName,
			Namespace: metav1.NamespaceSystem,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbac.GroupName,
			Kind:     "Role",
			Name:     configMapRBACName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: rbac.GroupKind,
				Name: kubeadmconstants.NodesGroup,
			},
			{
				Kind: rbac.GroupKind,
				Name: kubeadmconstants.NodeBootstrapTokenAuthGroup,
			},
		},
	})
}

func generateKubeletConfigMapName(version string) (string, error) {
	parsedVersion, err := semver.ParseTolerant(version)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse kubernetes version %q", version)
	}

	majorMinor := semver.Version{Major: parsedVersion.Major, Minor: parsedVersion.Minor}
	if majorMinor.GTE(minVerUnversionedKubeletConfig) {
		return kubeadmconstants.KubeletBaseConfigurationConfigMap, nil
	}

	return fmt.Sprintf(kubeletConfigMapName, parsedVersion.Major, parsedVersion.Minor), nil
}
