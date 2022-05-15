// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	bootstraptokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	kubeconfigutil "github.com/clastix/kamaji/internal/kubeconfig"
)

const kubeadmPhaseTimeout = 10 // seconds

type KubeadmPhase int

const (
	PhaseUploadConfigKubeadm KubeadmPhase = iota
	PhaseUploadConfigKubelet
	PhaseAddonCoreDNS
	PhaseAddonKubeProxy
	PhaseBootstrapToken
)

func (d KubeadmPhase) String() string {
	return [...]string{"PhaseUploadConfigKubeadm", "PhaseUploadConfigKubelet", "PhaseAddonCoreDNS", "PhaseAddonKubeProxy", "PhaseBootstrapToken"}[d]
}

const (
	kubeconfigAdminKeyName = "admin.conf"
)

type KubeadmPhaseResource struct {
	Client                       client.Client
	Log                          logr.Logger
	Name                         string
	KubeadmPhase                 KubeadmPhase
	kubeadmConfigResourceVersion string
}

func (r *KubeadmPhaseResource) isStatusEqual(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	status, err := r.getStatus(tenantControlPlane)
	if err != nil {
		return true
	}

	return status.KubeadmConfigResourceVersion == r.kubeadmConfigResourceVersion
}

func (r *KubeadmPhaseResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return !r.isStatusEqual(tenantControlPlane)
}

func (r *KubeadmPhaseResource) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubeadmPhaseResource) CleanUp(ctx context.Context) (bool, error) {
	return false, nil
}

func (r *KubeadmPhaseResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	return nil
}

func (r *KubeadmPhaseResource) getKubeadmPhaseFunction() (func(clientset.Interface, *kubeadm.Configuration) error, error) {
	switch r.KubeadmPhase {
	case PhaseUploadConfigKubeadm:
		return kubeadm.UploadKubeadmConfig, nil
	case PhaseUploadConfigKubelet:
		return kubeadm.UploadKubeletConfig, nil
	case PhaseAddonCoreDNS:
		return kubeadm.CoreDNSAddon, nil
	case PhaseAddonKubeProxy:
		return kubeadm.KubeProxyAddon, nil
	case PhaseBootstrapToken:
		return func(client clientset.Interface, config *kubeadm.Configuration) error {
			bootstrapTokensEnrichment(config.InitConfiguration.BootstrapTokens)

			return kubeadm.BootstrapToken(client, config)
		}, nil
	default:
		return nil, fmt.Errorf("no available functionality for phase %s", r.KubeadmPhase)
	}
}

func bootstrapTokensEnrichment(bootstrapTokens []bootstraptokenv1.BootstrapToken) {
	var bootstrapToken bootstraptokenv1.BootstrapToken
	if len(bootstrapTokens) > 0 {
		bootstrapToken = bootstrapTokens[0]
	}

	enrichBootstrapToken(&bootstrapToken)
	bootstrapTokens[0] = bootstrapToken
}

func enrichBootstrapToken(bootstrapToken *bootstraptokenv1.BootstrapToken) {
	if bootstrapToken.Token == nil {
		bootstrapToken.Token = &bootstraptokenv1.BootstrapTokenString{}
	}

	if bootstrapToken.Token.ID == "" {
		bootstrapToken.Token.ID = fmt.Sprintf("%s.%s", randomString(6), randomString(16))
	}
}

func (r *KubeadmPhaseResource) GetClient() client.Client {
	return r.Client
}

func (r *KubeadmPhaseResource) GetTmpDirectory() string {
	return ""
}

func (r *KubeadmPhaseResource) GetName() string {
	return r.Name
}

func (r *KubeadmPhaseResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	status, err := r.getStatus(tenantControlPlane)
	if err != nil {
		return err
	}

	status.LastUpdate = metav1.Now()
	status.KubeadmConfigResourceVersion = r.kubeadmConfigResourceVersion

	return nil
}

func (r *KubeadmPhaseResource) getStatus(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kamajiv1alpha1.KubeadmPhaseStatus, error) {
	switch r.KubeadmPhase {
	case PhaseUploadConfigKubeadm:
		return &tenantControlPlane.Status.KubeadmPhase.UploadConfigKubeadm, nil
	case PhaseUploadConfigKubelet:
		return &tenantControlPlane.Status.KubeadmPhase.UploadConfigKubelet, nil
	case PhaseAddonCoreDNS:
		return &tenantControlPlane.Status.KubeadmPhase.AddonCoreDNS, nil
	case PhaseAddonKubeProxy:
		return &tenantControlPlane.Status.KubeadmPhase.AddonKubeProxy, nil
	case PhaseBootstrapToken:
		return &tenantControlPlane.Status.KubeadmPhase.BootstrapToken, nil
	default:
		return nil, fmt.Errorf("%s is not a right kubeadm phase", r.KubeadmPhase)
	}
}

func (r *KubeadmPhaseResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return r.reconcile(ctx, tenantControlPlane)
}

func (r *KubeadmPhaseResource) reconcile(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	config, resourceVersion, err := getKubeadmConfiguration(ctx, r, tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	kubeconfig, err := r.getKubeconfig(ctx, tenantControlPlane)
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

	status, err := r.getStatus(tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	if resourceVersion == status.KubeadmConfigResourceVersion {
		r.kubeadmConfigResourceVersion = resourceVersion

		return controllerutil.OperationResultNone, nil
	}

	client, err := r.getRESTClient(ctx, tenantControlPlane)
	if err != nil {
		return controllerutil.OperationResultNone, err
	}

	fun, err := r.getKubeadmPhaseFunction()
	if err != nil {
		return controllerutil.OperationResultNone, err
	}
	if err = fun(client, config); err != nil {
		return controllerutil.OperationResultNone, err
	}

	r.kubeadmConfigResourceVersion = resourceVersion

	if status.LastUpdate.IsZero() {
		return controllerutil.OperationResultCreated, nil
	}

	return controllerutil.OperationResultUpdated, nil
}

func (r *KubeadmPhaseResource) getKubeconfigSecret(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*corev1.Secret, error) {
	kubeconfigSecretName := tenantControlPlane.Status.KubeConfig.Admin.SecretName
	namespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: kubeconfigSecretName}
	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, namespacedName, secret); err != nil {
		return nil, err
	}

	return secret, nil
}

func (r *KubeadmPhaseResource) getKubeconfig(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kubeconfigutil.Kubeconfig, error) {
	secretKubeconfig, err := r.getKubeconfigSecret(ctx, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	bytes, ok := secretKubeconfig.Data[kubeconfigAdminKeyName]
	if !ok {
		return nil, fmt.Errorf("%s is not into kubeconfig secret", kubeconfigAdminKeyName)
	}

	return kubeconfigutil.GetKubeconfigFromBytes(bytes)
}

func (r *KubeadmPhaseResource) getRESTClient(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*clientset.Clientset, error) {
	config, err := r.getRESTClientConfig(ctx, tenantControlPlane)
	if err != nil {
		return nil, err
	}

	return clientset.NewForConfig(config)
}

func (r *KubeadmPhaseResource) getRESTClientConfig(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*restclient.Config, error) {
	kubeconfig, err := r.getKubeconfig(ctx, tenantControlPlane)
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
