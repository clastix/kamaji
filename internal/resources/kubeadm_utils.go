package resources

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	kubeconfigutil "github.com/clastix/kamaji/internal/kubeconfig"
)

func KubeadmPhaseCreate(ctx context.Context, r KubeadmPhaseResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	config, resourceVersion, err := getKubeadmConfiguration(ctx, r, tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	kubeconfig, err := getKubeconfig(ctx, r, tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	config.Kubeconfig = *kubeconfig
	config.Parameters = kubeadm.Parameters{
		TenantControlPlaneName:         tenantControlPlane.GetName(),
		TenantDNSServiceIPs:            tenantControlPlane.Spec.NetworkProfile.DNSServiceIPs,
		TenantControlPlaneVersion:      tenantControlPlane.Spec.Kubernetes.Version,
		TenantControlPlanePodCIDR:      tenantControlPlane.Spec.NetworkProfile.PodCIDR,
		TenantControlPlaneAddress:      tenantControlPlane.Spec.NetworkProfile.Address,
		TenantControlPlanePort:         tenantControlPlane.Spec.NetworkProfile.Port,
		TenantControlPlaneCGroupDriver: tenantControlPlane.Spec.Kubernetes.Kubelet.CGroupFS.String(),
	}

	status, err := r.GetStatus(tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	storedResourceVersion := status.GetKubeadmConfigResourceVersion()
	if resourceVersion == storedResourceVersion {
		r.SetKubeadmConfigResourceVersion(resourceVersion)

		return controllerutil.OperationResultNone, nil
	}

	client, err := GetRESTClient(ctx, r, tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	fun, err := r.GetKubeadmFunction()
	if err != nil {
		return controllerutil.OperationResultNone, err
	}
	if err = fun(client, config); err != nil {
		return controllerutil.OperationResultNone, err
	}

	r.SetKubeadmConfigResourceVersion(resourceVersion)

	if storedResourceVersion == "" {
		return controllerutil.OperationResultCreated, nil
	}

	return controllerutil.OperationResultUpdated, nil
}

func getKubeconfigSecret(ctx context.Context, r KubeadmPhaseResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*corev1.Secret, error) {
	kubeconfigSecretName := tenantControlPlane.Status.KubeConfig.Admin.SecretName
	namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: kubeconfigSecretName}
	secret := &corev1.Secret{}
	if err := r.GetClient().Get(ctx, namespacedName, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

func getKubeconfig(ctx context.Context, r KubeadmPhaseResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kubeconfigutil.Kubeconfig, error) {
	secretKubeconfig, err := getKubeconfigSecret(ctx, r, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	bytes, ok := secretKubeconfig.Data[kubeconfigAdminKeyName]
	if !ok {
		return nil, fmt.Errorf("%s is not into kubeconfig secret", kubeconfigAdminKeyName)
	}

	return kubeconfigutil.GetKubeconfigFromBytes(bytes)
}

func GetRESTClient(ctx context.Context, r KubeadmPhaseResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*clientset.Clientset, error) {
	config, err := getRESTClientConfig(ctx, r, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	return clientset.NewForConfig(config)
}

func getRESTClientConfig(ctx context.Context, r KubeadmPhaseResource, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*restclient.Config, error) {
	kubeconfig, err := getKubeconfig(ctx, r, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	config := &restclient.Config{
		Host: fmt.Sprintf("https://%s:%d", getTenantControllerInternalFQDN(*tenantControlPlane), tenantControlPlane.Spec.NetworkProfile.Port),
		TLSClientConfig: restclient.TLSClientConfig{
			CAData:   kubeconfig.Clusters[0].Cluster.CertificateAuthorityData,
			CertData: kubeconfig.AuthInfos[0].AuthInfo.ClientCertificateData,
			KeyData:  kubeconfig.AuthInfos[0].AuthInfo.ClientKeyData,
		},
		Timeout: time.Second * kubeadmPhaseTimeout,
	}

	return config, nil
}
