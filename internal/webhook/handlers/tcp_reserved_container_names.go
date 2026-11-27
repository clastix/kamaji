// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"
	"slices"

	"gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/builders/controlplane"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

// TenantControlPlaneReservedContainerNames rejects additional containers and init containers
// named after the ones built by Kamaji. Without it, the collision is only caught on update,
// by the dry-run of the resulting Deployment: on creation there is no Deployment yet, so the
// TenantControlPlane is accepted and its first update reconciliation fails on a duplicate name.
type TenantControlPlaneReservedContainerNames struct{}

func (t TenantControlPlaneReservedContainerNames) OnCreate(object runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		return nil, validateReservedContainerNames(tcp, &kamajiv1alpha1.TenantControlPlane{})
	}
}

func (t TenantControlPlaneReservedContainerNames) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlaneReservedContainerNames) OnUpdate(newObject, oldObject runtime.Object) AdmissionResponse {
	return func(context.Context, admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp, previousTCP := newObject.(*kamajiv1alpha1.TenantControlPlane), oldObject.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		return nil, validateReservedContainerNames(tcp, previousTCP)
	}
}

// validateReservedContainerNames only checks the names the update introduces: a TenantControlPlane
// declaring a reserved name before this validation existed keeps accepting unrelated updates,
// such as the finalizer removal upon deletion.
func validateReservedContainerNames(tcp, previousTCP *kamajiv1alpha1.TenantControlPlane) error {
	deployment, previousDeployment := tcp.Spec.ControlPlane.Deployment, previousTCP.Spec.ControlPlane.Deployment

	if err := validateAddedContainerNames("additionalContainers", deployment.AdditionalContainers, previousDeployment.AdditionalContainers, controlplane.ReservedContainerNames()); err != nil {
		return err
	}

	return validateAddedContainerNames("additionalInitContainers", deployment.AdditionalInitContainers, previousDeployment.AdditionalInitContainers, controlplane.ReservedInitContainerNames())
}

func validateAddedContainerNames(field string, containers, previousContainers []corev1.Container, reserved []string) error {
	for i, container := range containers {
		if !slices.Contains(reserved, container.Name) {
			continue
		}

		if slices.ContainsFunc(previousContainers, func(previous corev1.Container) bool { return previous.Name == container.Name }) {
			continue
		}

		return fmt.Errorf("spec.controlPlane.deployment.%s[%d].name: %q is reserved for a container built by Kamaji", field, i, container.Name)
	}

	return nil
}
