// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/upgrade"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneVersion struct{}

func (t TenantControlPlaneVersion) OnCreate(object runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		ver, err := semver.New(t.normalizeKubernetesVersion(tcp.Spec.Kubernetes.Version))
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse the desired Kubernetes version")
		}

		supportedVer, supportedErr := semver.Make(t.normalizeKubernetesVersion(upgrade.KubeadmVersion))
		if supportedErr != nil {
			return nil, errors.Wrap(supportedErr, "unable to parse the Kamaji supported Kubernetes version")
		}

		if ver.GT(supportedVer) {
			return nil, fmt.Errorf("unable to create a TenantControlPlane with a Kubernetes version greater than the supported one, actually %s", supportedVer.String())
		}

		return nil, nil
	}
}

func (t TenantControlPlaneVersion) normalizeKubernetesVersion(input string) string {
	if strings.HasPrefix(input, "v") {
		return strings.Replace(input, "v", "", 1)
	}

	return input
}

func (t TenantControlPlaneVersion) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneVersion) OnUpdate(object runtime.Object, oldObject runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		newTCP, oldTCP := object.(*kamajiv1alpha1.TenantControlPlane), oldObject.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		oldVer, oldErr := semver.Make(t.normalizeKubernetesVersion(oldTCP.Spec.Kubernetes.Version))
		if oldErr != nil {
			return nil, errors.Wrap(oldErr, "unable to parse the previous Kubernetes version")
		}

		newVer, newErr := semver.New(t.normalizeKubernetesVersion(newTCP.Spec.Kubernetes.Version))
		if newErr != nil {
			return nil, errors.Wrap(newErr, "unable to parse the desired Kubernetes version")
		}

		supportedVer, supportedErr := semver.Make(t.normalizeKubernetesVersion(upgrade.KubeadmVersion))
		if supportedErr != nil {
			return nil, errors.Wrap(supportedErr, "unable to parse the Kamaji supported Kubernetes version")
		}

		switch {
		case newVer.GT(supportedVer):
			return nil, fmt.Errorf("unable to upgrade to a version greater than the supported one, actually %s", supportedVer.String())
		case newVer.LT(oldVer):
			return nil, fmt.Errorf("unable to downgrade a TenantControlPlane from %s to %s", oldVer.String(), newVer.String())
		case newVer.Minor-oldVer.Minor > 1:
			return nil, fmt.Errorf("unable to upgrade to a minor version in a non-sequential mode")
		}

		return nil, nil
	}
}
