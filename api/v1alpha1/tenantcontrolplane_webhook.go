// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clastix/kamaji/internal/upgrade"
)

//+kubebuilder:webhook:path=/mutate-kamaji-clastix-io-v1alpha1-tenantcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=create;update,versions=v1alpha1,name=mtenantcontrolplane.kb.io,admissionReviewVersions=v1
//+kubebuilder:webhook:path=/validate-kamaji-clastix-io-v1alpha1-tenantcontrolplane,mutating=false,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=create;update,versions=v1alpha1,name=vtenantcontrolplane.kb.io,admissionReviewVersions=v1

func (in *TenantControlPlane) SetupWebhookWithManager(mgr ctrl.Manager, datastore string) error {
	validator := &tenantControlPlaneValidator{
		client:           mgr.GetClient(),
		defaultDatastore: datastore,
		log:              mgr.GetLogger().WithName("tenantcontrolplane-webhook"),
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(in).
		WithValidator(validator).
		WithDefaulter(validator).
		Complete()
}

type tenantControlPlaneValidator struct {
	client           client.Client
	defaultDatastore string
	log              logr.Logger
}

func (t *tenantControlPlaneValidator) Default(_ context.Context, obj runtime.Object) error {
	tcp, ok := obj.(*TenantControlPlane)
	if !ok {
		return fmt.Errorf("expected *kamajiv1alpha1.TenantControlPlane")
	}

	if len(tcp.Spec.DataStore) == 0 {
		tcp.Spec.DataStore = t.defaultDatastore
	}

	return nil
}

func (t *tenantControlPlaneValidator) ValidateCreate(_ context.Context, obj runtime.Object) error {
	tcp, ok := obj.(*TenantControlPlane)
	if !ok {
		return fmt.Errorf("expected *kamajiv1alpha1.TenantControlPlane")
	}

	t.log.Info("validate create", "name", tcp.Name, "namespace", tcp.Namespace)

	ver, err := semver.New(t.normalizeKubernetesVersion(tcp.Spec.Kubernetes.Version))
	if err != nil {
		return errors.Wrap(err, "unable to parse the desired Kubernetes version")
	}

	supportedVer, supportedErr := semver.Make(t.normalizeKubernetesVersion(upgrade.KubeadmVersion))
	if supportedErr != nil {
		return errors.Wrap(supportedErr, "unable to parse the Kamaji supported Kubernetes version")
	}

	if ver.GT(supportedVer) {
		return fmt.Errorf("unable to create a TenantControlPlane with a Kubernetes version greater than the supported one, actually %s", supportedVer.String())
	}

	if err = t.validatePreferredKubeletAddressTypes(tcp.Spec.Kubernetes.Kubelet.PreferredAddressTypes); err != nil {
		return err
	}

	return nil
}

func (t *tenantControlPlaneValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	old, ok := oldObj.(*TenantControlPlane)
	if !ok {
		return fmt.Errorf("expected *kamajiv1alpha1.TenantControlPlane")
	}

	tcp, ok := newObj.(*TenantControlPlane)
	if !ok {
		return fmt.Errorf("expected *kamajiv1alpha1.TenantControlPlane")
	}

	t.log.Info("validate update", "name", tcp.Name, "namespace", tcp.Namespace)

	if err := t.validateVersionUpdate(old, tcp); err != nil {
		return err
	}
	if err := t.validateDataStore(ctx, old, tcp); err != nil {
		return err
	}
	if err := t.validatePreferredKubeletAddressTypes(tcp.Spec.Kubernetes.Kubelet.PreferredAddressTypes); err != nil {
		return err
	}

	return nil
}

func (t *tenantControlPlaneValidator) ValidateDelete(context.Context, runtime.Object) error {
	return nil
}

func (t *tenantControlPlaneValidator) validatePreferredKubeletAddressTypes(addressTypes []KubeletPreferredAddressType) error {
	s := sets.NewString()

	for _, at := range addressTypes {
		if s.Has(string(at)) {
			return fmt.Errorf("preferred kubelet address types is stated multiple times: %s", at)
		}

		s.Insert(string(at))
	}

	return nil
}

func (t *tenantControlPlaneValidator) validateVersionUpdate(oldObj, newObj *TenantControlPlane) error {
	oldVer, oldErr := semver.Make(t.normalizeKubernetesVersion(oldObj.Spec.Kubernetes.Version))
	if oldErr != nil {
		return errors.Wrap(oldErr, "unable to parse the previous Kubernetes version")
	}

	newVer, newErr := semver.New(t.normalizeKubernetesVersion(newObj.Spec.Kubernetes.Version))
	if newErr != nil {
		return errors.Wrap(newErr, "unable to parse the desired Kubernetes version")
	}

	supportedVer, supportedErr := semver.Make(t.normalizeKubernetesVersion(upgrade.KubeadmVersion))
	if supportedErr != nil {
		return errors.Wrap(supportedErr, "unable to parse the Kamaji supported Kubernetes version")
	}

	switch {
	case newVer.GT(supportedVer):
		return fmt.Errorf("unable to upgrade to a version greater than the supported one, actually %s", supportedVer.String())
	case newVer.LT(oldVer):
		return fmt.Errorf("unable to downgrade a TenantControlPlane from %s to %s", oldVer.String(), newVer.String())
	case newVer.Minor-oldVer.Minor > 1:
		return fmt.Errorf("unable to upgrade to a minor version in a non-sequential mode")
	}

	return nil
}

func (t *tenantControlPlaneValidator) validateDataStore(ctx context.Context, oldObj, tcp *TenantControlPlane) error {
	if oldObj.Spec.DataStore == tcp.Spec.DataStore {
		return nil
	}

	previousDatastore, desiredDatastore := &DataStore{}, &DataStore{}

	if err := t.client.Get(ctx, types.NamespacedName{Name: oldObj.Spec.DataStore}, previousDatastore); err != nil {
		return fmt.Errorf("unable to retrieve old DataStore for validation: %w", err)
	}

	if err := t.client.Get(ctx, types.NamespacedName{Name: tcp.Spec.DataStore}, desiredDatastore); err != nil {
		return fmt.Errorf("unable to retrieve old DataStore for validation: %w", err)
	}

	if previousDatastore.Spec.Driver != desiredDatastore.Spec.Driver {
		return fmt.Errorf("migration between different Datastore drivers is not supported")
	}

	return nil
}

func (t *tenantControlPlaneValidator) normalizeKubernetesVersion(input string) string {
	if strings.HasPrefix(input, "v") {
		return strings.Replace(input, "v", "", 1)
	}

	return input
}
