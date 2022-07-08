// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiapi "github.com/clastix/kamaji/api"
	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/kubeadm"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubeadmAddon int

const (
	AddonCoreDNS KubeadmAddon = iota
	AddonKubeProxy
)

func (d KubeadmAddon) String() string {
	return [...]string{"PhaseAddonCoreDNS", "PhaseAddonKubeProxy"}[d]
}

type KubeadmAddonResource struct {
	Client                       client.Client
	Log                          logr.Logger
	Name                         string
	KubeadmAddon                 KubeadmAddon
	kubeadmConfigResourceVersion string
}

func (r *KubeadmAddonResource) isStatusEqual(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	i, err := r.GetStatus(tenantControlPlane)
	if err != nil {
		return false
	}

	addonStatus, ok := i.(*kamajiv1alpha1.AddonStatus)
	if !ok {
		return false
	}

	return addonStatus.KubeadmConfigResourceVersion == r.kubeadmConfigResourceVersion
}

func (r *KubeadmAddonResource) SetKubeadmConfigResourceVersion(rv string) {
	r.kubeadmConfigResourceVersion = rv
}

func (r *KubeadmAddonResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return !r.isStatusEqual(tenantControlPlane)
}

func (r *KubeadmAddonResource) ShouldCleanup(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	spec, err := r.getSpec(tenantControlPlane)
	if err != nil {
		return false
	}

	return spec == nil
}

func (r *KubeadmAddonResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	client, err := utilities.GetTenantRESTClient(ctx, r.Client, tenantControlPlane)
	if err != nil {
		return false, err
	}

	fun, err := r.getRemoveAddonFunction()
	if err != nil {
		return false, err
	}

	if err := fun(ctx, client); err != nil {
		if !k8serrors.IsNotFound(err) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func (r *KubeadmAddonResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	return nil
}

func (r *KubeadmAddonResource) GetKubeadmFunction() (func(clientset.Interface, *kubeadm.Configuration) error, error) {
	switch r.KubeadmAddon {
	case AddonCoreDNS:
		return kubeadm.AddCoreDNS, nil
	case AddonKubeProxy:
		return kubeadm.AddKubeProxy, nil

	default:
		return nil, fmt.Errorf("no available functionality for phase %s", r.KubeadmAddon)
	}
}

func (r *KubeadmAddonResource) getRemoveAddonFunction() (func(context.Context, clientset.Interface) error, error) {
	switch r.KubeadmAddon {
	case AddonCoreDNS:
		return kubeadm.RemoveCoreDNSAddon, nil
	case AddonKubeProxy:
		return kubeadm.RemoveKubeProxy, nil
	default:
		return nil, fmt.Errorf("no available functionality for removing addon %s", r.KubeadmAddon)
	}
}

func (r *KubeadmAddonResource) GetClient() client.Client {
	return r.Client
}

func (r *KubeadmAddonResource) GetTmpDirectory() string {
	return ""
}

func (r *KubeadmAddonResource) GetName() string {
	return r.Name
}

func (r *KubeadmAddonResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	i, err := r.GetStatus(tenantControlPlane)
	if err != nil {
		return err
	}

	status, ok := i.(*kamajiv1alpha1.AddonStatus)
	if !ok {
		return fmt.Errorf("error addon status")
	}

	status.LastUpdate = metav1.Now()
	status.KubeadmConfigResourceVersion = r.kubeadmConfigResourceVersion

	return nil
}

func (r *KubeadmAddonResource) GetStatus(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (kamajiapi.KubeadmConfigResourceVersionDependant, error) {
	switch r.KubeadmAddon {
	case AddonCoreDNS:
		return &tenantControlPlane.Status.Addons.CoreDNS, nil
	case AddonKubeProxy:
		return &tenantControlPlane.Status.Addons.KubeProxy, nil
	default:
		return nil, fmt.Errorf("%s has no addon status", r.KubeadmAddon)
	}
}

func (r *KubeadmAddonResource) getSpec(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (*kamajiv1alpha1.AddonSpec, error) {
	switch r.KubeadmAddon {
	case AddonCoreDNS:
		return tenantControlPlane.Spec.Addons.CoreDNS, nil
	case AddonKubeProxy:
		return tenantControlPlane.Spec.Addons.KubeProxy, nil
	default:
		return nil, fmt.Errorf("%s has no spec", r.KubeadmAddon)
	}
}

func (r *KubeadmAddonResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return KubeadmPhaseCreate(ctx, r, tenantControlPlane)
}
