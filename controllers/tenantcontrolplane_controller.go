// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources"
)

const (
	separator = ","
	finalizer = "finalizer.kamaji.clastix.io"
)

// TenantControlPlaneReconciler reconciles a TenantControlPlane object.
type TenantControlPlaneReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config TenantControlPlaneReconcilerConfig
}

// TenantControlPlaneReconcilerConfig gives the necessary configuration for TenantControlPlaneReconciler.
type TenantControlPlaneReconcilerConfig struct {
	ETCDCASecretName          string
	ETCDCASecretNamespace     string
	ETCDClientSecretName      string
	ETCDClientSecretNamespace string
	ETCDEndpoints             string
	ETCDCompactionInterval    string
	TmpBaseDirectory          string
}

//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=tenantcontrolplanes/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete

func (r *TenantControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	tenantControlPlane := &kamajiv1alpha1.TenantControlPlane{}
	isTenantControlPlane, err := r.getTenantControlPlane(ctx, req.NamespacedName, tenantControlPlane)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !isTenantControlPlane {
		return ctrl.Result{}, nil
	}

	markedToBeDeleted := tenantControlPlane.GetDeletionTimestamp() != nil
	hasFinalizer := hasFinalizer(*tenantControlPlane)

	if markedToBeDeleted && !hasFinalizer {
		return ctrl.Result{}, nil
	}

	if markedToBeDeleted {
		registeredDeleteableResources := []resources.DeleteableResource{
			&resources.ETCDSetupResource{
				Name:                  "etcd-setup",
				Client:                r.Client,
				Scheme:                r.Scheme,
				Log:                   log,
				ETCDClientCertsSecret: getNamespacedName(r.Config.ETCDClientSecretNamespace, r.Config.ETCDClientSecretName),
				ETCDCACertsSecret:     getNamespacedName(r.Config.ETCDCASecretNamespace, r.Config.ETCDCASecretName),
				Endpoints:             getArrayFromString(r.Config.ETCDEndpoints),
			},
		}

		for _, resource := range registeredDeleteableResources {
			if err := resource.Delete(ctx, tenantControlPlane); err != nil {
				return ctrl.Result{}, err
			}
		}

		if hasFinalizer {
			controllerutil.RemoveFinalizer(tenantControlPlane, finalizer)
			if err := r.Update(ctx, tenantControlPlane); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	if !hasFinalizer {
		controllerutil.AddFinalizer(tenantControlPlane, finalizer)
		if err := r.Update(ctx, tenantControlPlane); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	registeredResources := []resources.Resource{
		&resources.KubernetesUpgrade{
			Name:   "upgrade",
			Client: r.Client,
		},
		&resources.KubernetesServiceResource{
			Client: r.Client,
		},
		&resources.KubeadmConfigResource{
			Name:                   "kubeadmconfig",
			Port:                   tenantControlPlane.Spec.NetworkProfile.Port,
			KubernetesVersion:      tenantControlPlane.Spec.Kubernetes.Version,
			PodCIDR:                tenantControlPlane.Spec.NetworkProfile.PodCIDR,
			ServiceCIDR:            tenantControlPlane.Spec.NetworkProfile.ServiceCIDR,
			Domain:                 tenantControlPlane.Spec.NetworkProfile.Domain,
			ETCDs:                  getArrayFromString(r.Config.ETCDEndpoints),
			ETCDCompactionInterval: r.Config.ETCDCompactionInterval,
			Client:                 r.Client,
			Scheme:                 r.Scheme,
			Log:                    log,
			TmpDirectory:           getTmpDirectory(r.Config.TmpBaseDirectory, *tenantControlPlane),
		},
		&resources.CACertificate{
			Name:         "ca",
			Client:       r.Client,
			Log:          log,
			TmpDirectory: getTmpDirectory(r.Config.TmpBaseDirectory, *tenantControlPlane),
		},
		&resources.FrontProxyCACertificate{
			Name:         "front-proxy-ca-certificate",
			Client:       r.Client,
			Log:          log,
			TmpDirectory: getTmpDirectory(r.Config.TmpBaseDirectory, *tenantControlPlane),
		},
		&resources.SACertificate{
			Name:         "sa-certificate",
			Client:       r.Client,
			Log:          log,
			TmpDirectory: getTmpDirectory(r.Config.TmpBaseDirectory, *tenantControlPlane),
		},
		&resources.APIServerCertificate{
			Name:         "api-server-certificate",
			Client:       r.Client,
			Log:          log,
			TmpDirectory: getTmpDirectory(r.Config.TmpBaseDirectory, *tenantControlPlane),
		},
		&resources.APIServerKubeletClientCertificate{
			Name:         "api-server-kubelet-client-certificate",
			Client:       r.Client,
			Log:          log,
			TmpDirectory: getTmpDirectory(r.Config.TmpBaseDirectory, *tenantControlPlane),
		},
		&resources.FrontProxyClientCertificate{
			Name:         "front-proxy-client-certificate",
			Client:       r.Client,
			Log:          log,
			TmpDirectory: getTmpDirectory(r.Config.TmpBaseDirectory, *tenantControlPlane),
		},
		&resources.KubeconfigResource{
			Name:               "admin-kubeconfig",
			Client:             r.Client,
			Scheme:             r.Scheme,
			Log:                log,
			KubeConfigFileName: resources.AdminKubeConfigFileName,
			TmpDirectory:       getTmpDirectory(r.Config.TmpBaseDirectory, *tenantControlPlane),
		},
		&resources.KubeconfigResource{
			Name:               "controller-manager-kubeconfig",
			Client:             r.Client,
			Scheme:             r.Scheme,
			Log:                log,
			KubeConfigFileName: resources.ControllerManagerKubeConfigFileName,
			TmpDirectory:       getTmpDirectory(r.Config.TmpBaseDirectory, *tenantControlPlane),
		},
		&resources.KubeconfigResource{
			Name:               "scheduler-kubeconfig",
			Client:             r.Client,
			Scheme:             r.Scheme,
			Log:                log,
			KubeConfigFileName: resources.SchedulerKubeConfigFileName,
			TmpDirectory:       getTmpDirectory(r.Config.TmpBaseDirectory, *tenantControlPlane),
		},
		&resources.ETCDCACertificatesResource{
			Name:                  "etcd-ca-certificates",
			Client:                r.Client,
			Log:                   log,
			ETCDCASecretName:      r.Config.ETCDCASecretName,
			ETCDCASecretNamespace: r.Config.ETCDCASecretNamespace,
		},
		&resources.ETCDCertificatesResource{
			Name:   "etcd-certificates",
			Client: r.Client,
			Log:    log,
		},
		&resources.ETCDSetupResource{
			Name:                  "etcd-setup",
			Client:                r.Client,
			Scheme:                r.Scheme,
			Log:                   log,
			ETCDClientCertsSecret: getNamespacedName(r.Config.ETCDClientSecretNamespace, r.Config.ETCDClientSecretName),
			ETCDCACertsSecret:     getNamespacedName(r.Config.ETCDCASecretNamespace, r.Config.ETCDCASecretName),
			Endpoints:             getArrayFromString(r.Config.ETCDEndpoints),
		},
		&resources.KubernetesDeploymentResource{
			Client:                 r.Client,
			ETCDEndpoints:          getArrayFromString(r.Config.ETCDEndpoints),
			ETCDCompactionInterval: r.Config.ETCDCompactionInterval,
		},
		&resources.KubernetesIngressResource{
			Client: r.Client,
		},
		&resources.KubeadmPhase{
			Name:   "upload-config-kubeadm",
			Client: r.Client,
			Log:    log,
			Phase:  resources.PhaseUploadConfigKubeadm,
		},
		&resources.KubeadmPhase{
			Name:   "upload-config-kubelet",
			Client: r.Client,
			Log:    log,
			Phase:  resources.PhaseUploadConfigKubelet,
		},
		&resources.KubeadmPhase{
			Name:   "bootstrap-token",
			Client: r.Client,
			Log:    log,
			Phase:  resources.PhaseBootstrapToken,
		},
		&resources.KubeadmAddonResource{
			Name:         "coredns",
			Client:       r.Client,
			Log:          log,
			KubeadmAddon: resources.AddonCoreDNS,
		},
		&resources.KubeadmAddonResource{
			Name:         "kubeproxy",
			Client:       r.Client,
			Log:          log,
			KubeadmAddon: resources.AddonKubeProxy,
		},
	}

	for _, resource := range registeredResources {
		result, err := resources.Handle(ctx, resource, tenantControlPlane)
		if err != nil {
			return ctrl.Result{}, err
		}

		if result == controllerutil.OperationResultNone {
			continue
		}

		if err := r.updateStatus(ctx, req.NamespacedName, resource); err != nil {
			return ctrl.Result{}, err
		}

		log.Info(fmt.Sprintf("%s has been configured", resource.GetName()))

		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TenantControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kamajiv1alpha1.TenantControlPlane{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}

func (r *TenantControlPlaneReconciler) getTenantControlPlane(ctx context.Context, namespacedName k8stypes.NamespacedName, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	if err := r.Client.Get(ctx, namespacedName, tenantControlPlane); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *TenantControlPlaneReconciler) updateStatus(ctx context.Context, namespacedName k8stypes.NamespacedName, resource resources.Resource) error {
	tenantControlPlane := &kamajiv1alpha1.TenantControlPlane{}
	isTenantControlPlane, err := r.getTenantControlPlane(ctx, namespacedName, tenantControlPlane)
	if err != nil {
		return err
	}

	if !isTenantControlPlane {
		return fmt.Errorf("error updating tenantControlPlane %s: not found", namespacedName.Name)
	}

	if err := resource.UpdateTenantControlPlaneStatus(ctx, tenantControlPlane); err != nil {
		return err
	}

	if err := r.Status().Update(ctx, tenantControlPlane); err != nil {
		return fmt.Errorf("error updating tenantControlPlane status: %w", err)
	}

	return nil
}

func getArrayFromString(s string) []string {
	var a []string
	a = append(a, strings.Split(s, separator)...)

	return a
}

func getNamespacedName(namespace string, name string) k8stypes.NamespacedName {
	return k8stypes.NamespacedName{Namespace: namespace, Name: name}
}

func getTmpDirectory(base string, tenantControlPlane kamajiv1alpha1.TenantControlPlane) string {
	return fmt.Sprintf("%s/%s/%s", base, tenantControlPlane.GetName(), uuid.New())
}

func hasFinalizer(tenantControlPlane kamajiv1alpha1.TenantControlPlane) bool {
	for _, f := range tenantControlPlane.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}

	return false
}
