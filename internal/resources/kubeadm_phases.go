// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	bootstraptokenv1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/bootstraptoken/v1"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
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
)

func (d kubeadmPhase) String() string {
	return [...]string{"PhaseUploadConfigKubeadm", "PhaseUploadConfigKubelet", "PhaseBootstrapToken"}[d]
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
	default:
		panic("shouldn't happen")
	}
}

func (r *KubeadmPhase) GetPredicateFunc() func(obj client.Object) bool {
	switch r.Phase {
	case PhaseUploadConfigKubeadm:
		return func(obj client.Object) bool {
			return obj.GetName() == constants.KubeadmConfigConfigMap && obj.GetNamespace() == metav1.NamespaceSystem
		}
	case PhaseUploadConfigKubelet:
		return func(obj client.Object) bool {
			return obj.GetName() == constants.KubeletBaseConfigurationConfigMap && obj.GetNamespace() == metav1.NamespaceSystem
		}
	case PhaseBootstrapToken:
		return func(obj client.Object) bool {
			secret := obj.(*corev1.Secret) //nolint:forcetypeassert

			return secret.Type == "bootstrap.kubernetes.io/token" && secret.GetNamespace() == metav1.NamespaceSystem
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

func (r *KubeadmPhase) GetKubeadmFunction() (func(clientset.Interface, *kubeadm.Configuration) ([]byte, error), error) {
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
	case PhaseUploadConfigKubeadm, PhaseUploadConfigKubelet:
		return nil, nil
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
