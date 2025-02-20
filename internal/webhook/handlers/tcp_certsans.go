// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/validation"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneCertSANs struct{}

func (t TenantControlPlaneCertSANs) ValidateCertSANs(tcp *kamajiv1alpha1.TenantControlPlane) error {
	if len(tcp.Spec.NetworkProfile.CertSANs) == 0 {
		return nil
	}

	if err := validation.ValidateCertSANs(tcp.Spec.NetworkProfile.CertSANs, field.NewPath("spec.networkProfile.certSANs")); err != nil {
		return err.ToAggregate()
	}

	return nil
}

func (t TenantControlPlaneCertSANs) OnCreate(obj runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := obj.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		return nil, t.ValidateCertSANs(tcp)
	}
}

func (t TenantControlPlaneCertSANs) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneCertSANs) OnUpdate(newObject runtime.Object, prevObject runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := newObject.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		return nil, t.ValidateCertSANs(tcp)
	}
}
