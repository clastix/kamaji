// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

const (
	AdminKubeConfigFileName             = kubeadmconstants.AdminKubeConfigFileName
	ControllerManagerKubeConfigFileName = kubeadmconstants.ControllerManagerKubeConfigFileName
	SchedulerKubeConfigFileName         = kubeadmconstants.SchedulerKubeConfigFileName
	localhost                           = "127.0.0.1"
)

type KubeconfigResource struct {
	resource           *corev1.Secret
	Client             client.Client
	Log                logr.Logger
	Name               string
	KubeConfigFileName string
	TmpDirectory       string
}

func (r *KubeconfigResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubeconfigResource) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubeconfigResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return false, nil
}

func (r *KubeconfigResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.getPrefixedName(tenantControlPlane),
			Namespace: tenantControlPlane.GetNamespace(),
		},
	}

	return nil
}

func (r *KubeconfigResource) getPrefixedName(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return utilities.AddTenantPrefix(r.GetName(), tenantControlPlane)
}

func (r *KubeconfigResource) GetClient() client.Client {
	return r.Client
}

func (r *KubeconfigResource) GetTmpDirectory() string {
	return r.TmpDirectory
}

func (r *KubeconfigResource) GetName() string {
	return r.Name
}

func (r *KubeconfigResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	status, err := r.getKubeconfigStatus(tenantControlPlane)
	if err != nil {
		return err
	}

	status.LastUpdate = metav1.Now()
	status.SecretName = r.resource.GetName()
	status.Checksum = r.resource.Annotations["checksum"]

	return nil
}

func (r *KubeconfigResource) getKubeconfigStatus(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kamajiv1alpha1.KubeconfigStatus, error) {
	switch r.KubeConfigFileName {
	case kubeadmconstants.AdminKubeConfigFileName:
		return &tenantControlPlane.Status.KubeConfig.Admin, nil
	case kubeadmconstants.ControllerManagerKubeConfigFileName:
		return &tenantControlPlane.Status.KubeConfig.ControllerManager, nil
	case kubeadmconstants.SchedulerKubeConfigFileName:
		return &tenantControlPlane.Status.KubeConfig.Scheduler, nil
	default:
		return nil, fmt.Errorf("kubeconfigfilename %s is not a right name", r.KubeConfigFileName)
	}
}

func (r *KubeconfigResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubeconfigResource) checksum(apiServerCertificatesSecret *corev1.Secret, kubeadmChecksum string) string {
	return utilities.CalculateConfigMapChecksum(map[string]string{
		"ca-cert-checksum": string(apiServerCertificatesSecret.Data[kubeadmconstants.CACertName]),
		"ca-key-checksum":  string(apiServerCertificatesSecret.Data[kubeadmconstants.CAKeyName]),
		"kubeadmconfig":    kubeadmChecksum,
	})
}

func (r *KubeconfigResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	return func() error {
		config, err := getStoredKubeadmConfiguration(ctx, r, tenantControlPlane)
		if err != nil {
			return err
		}

		if err = r.customizeConfig(config); err != nil {
			return err
		}

		apiServerCertificatesSecretNamespacedName := k8stypes.NamespacedName{Namespace: tenantControlPlane.GetNamespace(), Name: tenantControlPlane.Status.Certificates.CA.SecretName}
		apiServerCertificatesSecret := &corev1.Secret{}
		if err := r.Client.Get(ctx, apiServerCertificatesSecretNamespacedName, apiServerCertificatesSecret); err != nil {
			return err
		}

		checksum := r.checksum(apiServerCertificatesSecret, config.Checksum())

		status, err := r.getKubeconfigStatus(tenantControlPlane)
		if err != nil {
			return err
		}

		if status.Checksum == checksum && kubeadm.IsKubeconfigValid(r.resource.Data[r.KubeConfigFileName]) {
			return nil
		}

		kubeconfig, err := kubeadm.CreateKubeconfig(
			r.KubeConfigFileName,

			kubeadm.CertificatePrivateKeyPair{
				Certificate: apiServerCertificatesSecret.Data[kubeadmconstants.CACertName],
				PrivateKey:  apiServerCertificatesSecret.Data[kubeadmconstants.CAKeyName],
			},
			config,
		)
		if err != nil {
			return err
		}
		r.resource.Data = map[string][]byte{
			r.KubeConfigFileName: kubeconfig,
		}

		r.resource.SetLabels(utilities.MergeMaps(
			utilities.KamajiLabels(),
			map[string]string{
				"kamaji.clastix.io/name":      tenantControlPlane.GetName(),
				"kamaji.clastix.io/component": r.GetName(),
			},
		))

		r.resource.SetAnnotations(map[string]string{
			"checksum": checksum,
		})

		return ctrl.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *KubeconfigResource) customizeConfig(config *kubeadm.Configuration) error {
	switch r.KubeConfigFileName {
	case kubeadmconstants.ControllerManagerKubeConfigFileName:
		return r.localhostAsAdvertiseAddress(config)
	case kubeadmconstants.SchedulerKubeConfigFileName:
		return r.localhostAsAdvertiseAddress(config)
	default:
		return nil
	}
}

func (r *KubeconfigResource) localhostAsAdvertiseAddress(config *kubeadm.Configuration) error {
	config.InitConfiguration.LocalAPIEndpoint.AdvertiseAddress = localhost

	return nil
}
