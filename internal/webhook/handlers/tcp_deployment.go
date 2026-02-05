// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"

	"github.com/google/go-cmp/cmp"
	"gomodules.xyz/jsonpatch/v2"
	appsv1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/builders/controlplane"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlaneDeployment struct {
	Client              client.Client
	DeploymentBuilder   controlplane.Deployment
	KonnectivityBuilder controlplane.Konnectivity
}

func (t TenantControlPlaneDeployment) OnCreate(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneDeployment) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneDeployment) shouldTriggerCheck(newTCP, oldTCP kamajiv1alpha1.TenantControlPlane) bool {
	if newTCP.Spec.ControlPlane.Deployment.AdditionalVolumeMounts == nil &&
		len(newTCP.Spec.ControlPlane.Deployment.AdditionalInitContainers) == 0 &&
		len(newTCP.Spec.ControlPlane.Deployment.AdditionalContainers) == 0 &&
		len(newTCP.Spec.ControlPlane.Deployment.AdditionalVolumes) == 0 {
		return false
	}

	if newTCP.Spec.ControlPlane.Deployment.AdditionalVolumeMounts != nil && oldTCP.Spec.ControlPlane.Deployment.AdditionalVolumeMounts == nil {
		return true
	}

	return !cmp.Equal(newTCP.Spec.ControlPlane.Deployment.AdditionalContainers, oldTCP.Spec.ControlPlane.Deployment.AdditionalContainers) ||
		!cmp.Equal(newTCP.Spec.ControlPlane.Deployment.AdditionalInitContainers, oldTCP.Spec.ControlPlane.Deployment.AdditionalInitContainers) ||
		!cmp.Equal(newTCP.Spec.ControlPlane.Deployment.AdditionalVolumes, oldTCP.Spec.ControlPlane.Deployment.AdditionalVolumes) ||
		!cmp.Equal(newTCP.Spec.ControlPlane.Deployment.AdditionalVolumeMounts, oldTCP.Spec.ControlPlane.Deployment.AdditionalVolumeMounts)
}

func (t TenantControlPlaneDeployment) OnUpdate(newObject runtime.Object, oldObject runtime.Object) AdmissionResponse {
	return func(ctx context.Context, _ admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, previousTCP := newObject.(*kamajiv1alpha1.TenantControlPlane), oldObject.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		if !t.shouldTriggerCheck(*tcp, *previousTCP) {
			return nil, nil
		}

		ds := kamajiv1alpha1.DataStore{}
		if err := t.Client.Get(ctx, types.NamespacedName{Name: tcp.Spec.DataStore}, &ds); err != nil {
			return nil, err
		}
		t.DeploymentBuilder.DataStore = ds

		dataStoreOverrides := make([]controlplane.DataStoreOverrides, 0, len(tcp.Spec.DataStoreOverrides))

		for _, dso := range tcp.Spec.DataStoreOverrides {
			ds := kamajiv1alpha1.DataStore{}
			if err := t.Client.Get(ctx, types.NamespacedName{Name: dso.DataStore}, &ds); err != nil {
				return nil, err
			}
			dataStoreOverrides = append(dataStoreOverrides, controlplane.DataStoreOverrides{
				Resource:  dso.Resource,
				DataStore: ds,
			})
		}
		t.DeploymentBuilder.DataStoreOverrides = dataStoreOverrides

		deployment := appsv1.Deployment{}
		deployment.Name = tcp.Name
		deployment.Namespace = tcp.Namespace

		err := t.Client.Get(ctx, types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, &deployment)
		if err != nil && !k8serrors.IsNotFound(err) {
			return nil, nil
		}

		t.DeploymentBuilder.Build(ctx, &deployment, *tcp)

		if tcp.Spec.Addons.Konnectivity != nil {
			t.KonnectivityBuilder.Build(&deployment, *tcp)
		}

		if k8serrors.IsNotFound(err) {
			err = t.Client.Create(ctx, &deployment, client.DryRunAll)
		} else {
			err = t.Client.Update(ctx, &deployment, client.DryRunAll)
		}

		if err != nil {
			return nil, fmt.Errorf("the resulting Deployment will generate a configuration error, cannot proceed: %w", err)
		}

		return nil, nil
	}
}
