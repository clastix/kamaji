// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"
	"strings"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneName struct{}

func (t TenantControlPlaneName) OnCreate(object runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if errs := validation.IsDNS1035Label(tcp.Name); len(errs) > 0 {
			return nil, fmt.Errorf("the provided name is invalid, %s", strings.Join(errs, ","))
		}

		return nil, nil
	}
}

func (t TenantControlPlaneName) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneName) OnUpdate(runtime.Object, runtime.Object) AdmissionResponse {
	return utils.NilOp()
}
