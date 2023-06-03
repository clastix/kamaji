// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneKubeletAddresses struct{}

func (t TenantControlPlaneKubeletAddresses) OnCreate(object runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		return nil, t.validatePreferredKubeletAddressTypes(tcp.Spec.Kubernetes.Kubelet.PreferredAddressTypes)
	}
}

func (t TenantControlPlaneKubeletAddresses) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneKubeletAddresses) OnUpdate(object runtime.Object, _ runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		return nil, t.validatePreferredKubeletAddressTypes(tcp.Spec.Kubernetes.Kubelet.PreferredAddressTypes)
	}
}

func (t TenantControlPlaneKubeletAddresses) validatePreferredKubeletAddressTypes(addressTypes []kamajiv1alpha1.KubeletPreferredAddressType) error {
	s := sets.New[string]()

	for _, at := range addressTypes {
		if s.Has(string(at)) {
			return fmt.Errorf("preferred kubelet address types is stated multiple times: %s", at)
		}

		s.Insert(string(at))
	}

	return nil
}
