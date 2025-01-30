// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"
	"net"

	"gomodules.xyz/jsonpatch/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneLoadBalancerSourceRanges struct{}

func (t TenantControlPlaneLoadBalancerSourceRanges) handle(tcp *kamajiv1alpha1.TenantControlPlane) error {
	for _, sourceCIDR := range tcp.Spec.NetworkProfile.LoadBalancerSourceRanges {
		_, _, err := net.ParseCIDR(sourceCIDR)
		if err != nil {
			return fmt.Errorf("invalid LoadBalancer source CIDR %s, %s", sourceCIDR, err.Error())
		}
	}

	return nil
}

func (t TenantControlPlaneLoadBalancerSourceRanges) OnCreate(object runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if err := t.handle(tcp); err != nil {
			return nil, err
		}

		return nil, nil
	}
}

func (t TenantControlPlaneLoadBalancerSourceRanges) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneLoadBalancerSourceRanges) OnUpdate(object runtime.Object, _ runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if err := t.handle(tcp); err != nil {
			return nil, err
		}

		return nil, nil
	}
}
