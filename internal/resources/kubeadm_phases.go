// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	bootstraptokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/kubeconfig"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/resources/utils"
)

type kubeadmPhase int

const (
	PhaseUploadConfigKubeadm kubeadmPhase = iota
	PhaseUploadConfigKubelet
	PhaseBootstrapToken
	PhaseClusterAdminRBAC
)

func (d kubeadmPhase) String() string {
	return [...]string{"PhaseUploadConfigKubeadm", "PhaseUploadConfigKubelet", "PhaseBootstrapToken", "PhaseClusterAdminRBAC"}[d]
}

type KubeadmPhase struct {
	Client   client.Client
	Phase    kubeadmPhase
	checksum string
}

func (r *KubeadmPhase) GetWatchedObject() client.Object {
	switch r.Phase {
	case PhaseUploadConfigKubeadm:
		return &corev1.ConfigMap{}
	case PhaseUploadConfigKubelet:
		return &corev1.ConfigMap{}
	case PhaseBootstrapToken:
		return &corev1.Secret{}
	case PhaseClusterAdminRBAC:
		return &rbacv1.ClusterRoleBinding{}
	default:
		panic("shouldn't happen")
	}
}

func (r *KubeadmPhase) GetPredicateFunc() func(obj client.Object) bool {
	switch r.Phase {
	case PhaseUploadConfigKubeadm:
		return func(obj client.Object) bool {
			return obj.GetName() == kubeadmconstants.KubeadmConfigConfigMap && obj.GetNamespace() == metav1.NamespaceSystem
		}
	case PhaseUploadConfigKubelet:
		return func(obj client.Object) bool {
			return obj.GetName() == kubeadmconstants.KubeletBaseConfigurationConfigMap && obj.GetNamespace() == metav1.NamespaceSystem
		}
	case PhaseBootstrapToken:
		return func(obj client.Object) bool {
			secret := obj.(*corev1.Secret) //nolint:forcetypeassert

			return secret.Type == "bootstrap.kubernetes.io/token" && secret.GetNamespace() == metav1.NamespaceSystem
		}
	case PhaseClusterAdminRBAC:
		return func(obj client.Object) bool {
			cr := obj.(*rbacv1.ClusterRoleBinding) //nolint:forcetypeassert

			return strings.HasPrefix(cr.Name, "kubeadm:")
		}
	default:
		panic("shouldn't happen")
	}
}

func (r *KubeadmPhase) isStatusEqual(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	i, err := r.GetStatus(tenantControlPlane)
	if err != nil {
		return true
	}

	status, ok := i.(*kamajiv1alpha1.KubeadmPhaseStatus)
	if !ok {
		return true
	}

	return status.Checksum == r.checksum
}

func (r *KubeadmPhase) SetKubeadmConfigChecksum(checksum string) {
	r.checksum = checksum
}

func (r *KubeadmPhase) ShouldStatusBeUpdated(_ context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return !r.isStatusEqual(tenantControlPlane)
}

func (r *KubeadmPhase) ShouldCleanup(*kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubeadmPhase) CleanUp(context.Context, *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *KubeadmPhase) Define(context.Context, *kamajiv1alpha1.TenantControlPlane) error {
	return nil
}

func (r *KubeadmPhase) GetKubeadmFunction(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) (func(clientset.Interface, *kubeadm.Configuration) ([]byte, error), error) {
	switch r.Phase {
	case PhaseUploadConfigKubeadm:
		return kubeadm.UploadKubeadmConfig, nil
	case PhaseUploadConfigKubelet:
		return kubeadm.UploadKubeletConfig, nil
	case PhaseBootstrapToken:
		return func(client clientset.Interface, config *kubeadm.Configuration) ([]byte, error) {
			bootstrapTokensEnrichment(config.InitConfiguration.BootstrapTokens)

			return nil, kubeadm.BootstrapToken(client, config)
		}, nil
	case PhaseClusterAdminRBAC:
		return func(c clientset.Interface, configuration *kubeadm.Configuration) ([]byte, error) {
			tmp, err := os.MkdirTemp("", string(tcp.UID))
			if err != nil {
				return nil, err
			}

			defer func() { _ = os.Remove(tmp) }()

			var caSecret corev1.Secret
			if err = r.Client.Get(ctx, types.NamespacedName{Name: tcp.Status.Certificates.CA.SecretName, Namespace: tcp.Namespace}, &caSecret); err != nil {
				return nil, err
			}

			crtKeyPair := kubeadm.CertificatePrivateKeyPair{
				Certificate: caSecret.Data[kubeadmconstants.CACertName],
				PrivateKey:  caSecret.Data[kubeadmconstants.CAKeyName],
			}

			for _, i := range []string{AdminKubeConfigFileName, SuperAdminKubeConfigFileName} {
				configuration.InitConfiguration.CertificatesDir, _ = os.MkdirTemp(tmp, "")

				kubeconfigValue, err := kubeadm.CreateKubeconfig(SuperAdminKubeConfigFileName, crtKeyPair, configuration)
				if err != nil {
					return nil, err
				}

				_ = os.WriteFile(fmt.Sprintf("%s/%s", tmp, i), kubeconfigValue, os.ModePerm)
			}

			if _, err = kubeconfig.EnsureAdminClusterRoleBinding(tmp, func(_ context.Context, _ clientset.Interface, _ clientset.Interface, duration time.Duration, duration2 time.Duration) (clientset.Interface, error) {
				return kubeconfig.EnsureAdminClusterRoleBindingImpl(ctx, c, c, duration, duration2)
			}); err != nil {
				return nil, err
			}

			return nil, nil
		}, nil
	default:
		return nil, fmt.Errorf("no available functionality for phase %s", r.Phase)
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
		bootstrapToken.Token.ID = fmt.Sprintf("%s.%s", utils.RandomString(6), utils.RandomString(16))
	}
}

func (r *KubeadmPhase) GetClient() client.Client {
	return r.Client
}

func (r *KubeadmPhase) GetTmpDirectory() string {
	return ""
}

func (r *KubeadmPhase) GetName() string {
	return r.Phase.String()
}

func (r *KubeadmPhase) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	logger := log.FromContext(ctx, "resource", r.GetName(), "phase", r.Phase.String())

	status, err := r.GetStatus(tenantControlPlane)
	if err != nil {
		logger.Error(err, "unable to update the status")

		return err
	}

	if status != nil {
		status.SetChecksum(r.checksum)
	}

	return nil
}

func (r *KubeadmPhase) GetStatus(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (kamajiv1alpha1.KubeadmConfigChecksumDependant, error) {
	switch r.Phase {
	case PhaseUploadConfigKubeadm, PhaseUploadConfigKubelet, PhaseClusterAdminRBAC:
		return nil, nil //nolint:nilnil
	case PhaseBootstrapToken:
		return &tenantControlPlane.Status.KubeadmPhase.BootstrapToken, nil
	default:
		return nil, fmt.Errorf("%s is not a right kubeadm phase", r.Phase)
	}
}

func (r *KubeadmPhase) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	logger := log.FromContext(ctx, "resource", r.GetName(), "phase", r.Phase.String())

	return KubeadmPhaseCreate(ctx, r, logger, tenantControlPlane)
}
