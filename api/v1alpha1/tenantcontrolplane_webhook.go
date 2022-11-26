// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/clastix/kamaji/internal/config"
	"github.com/clastix/kamaji/internal/upgrade"
)

// log is for logging in this package.
var tenantcontrolplanelog = logf.Log.WithName("tenantcontrolplane-resource")

func (in *TenantControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(in).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-kamaji-clastix-io-v1alpha1-tenantcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=create;update,versions=v1alpha1,name=mtenantcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &TenantControlPlane{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (in *TenantControlPlane) Default() {
	if len(in.Spec.DataStore) == 0 {
		in.Spec.DataStore = config.Config().GetString("datastore")
	}
}

//+kubebuilder:webhook:path=/validate-kamaji-clastix-io-v1alpha1-tenantcontrolplane,mutating=false,failurePolicy=fail,sideEffects=None,groups=kamaji.clastix.io,resources=tenantcontrolplanes,verbs=create;update,versions=v1alpha1,name=vtenantcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &TenantControlPlane{}

func normalizeKubernetesVersion(input string) string {
	if strings.HasPrefix(input, "v") {
		return strings.Replace(input, "v", "", 1)
	}

	return input
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (in *TenantControlPlane) ValidateCreate() error {
	tenantcontrolplanelog.Info("validate create", "name", in.Name, "namespace", in.Namespace)

	ver, err := semver.New(normalizeKubernetesVersion(in.Spec.Kubernetes.Version))
	if err != nil {
		return errors.Wrap(err, "unable to parse the desired Kubernetes version")
	}

	supportedVer, supportedErr := semver.Make(normalizeKubernetesVersion(upgrade.KubeadmVersion))
	if supportedErr != nil {
		return errors.Wrap(supportedErr, "unable to parse the Kamaji supported Kubernetes version")
	}

	if ver.GT(supportedVer) {
		return fmt.Errorf("unable to create a TenantControlPlane with a Kubernetes version greater than the supported one, actually %s", supportedVer.String())
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (in *TenantControlPlane) ValidateUpdate(old runtime.Object) error {
	tenantcontrolplanelog.Info("validate update", "name", in.Name, "namespace", in.Namespace)

	o := old.(*TenantControlPlane) //nolint:forcetypeassert

	oldVer, oldErr := semver.Make(normalizeKubernetesVersion(o.Spec.Kubernetes.Version))
	if oldErr != nil {
		return errors.Wrap(oldErr, "unable to parse the previous Kubernetes version")
	}

	newVer, newErr := semver.New(normalizeKubernetesVersion(in.Spec.Kubernetes.Version))
	if newErr != nil {
		return errors.Wrap(newErr, "unable to parse the desired Kubernetes version")
	}

	supportedVer, supportedErr := semver.Make(normalizeKubernetesVersion(upgrade.KubeadmVersion))
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

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (in *TenantControlPlane) ValidateDelete() error {
	return nil
}
