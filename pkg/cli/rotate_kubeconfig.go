// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

type RotateKubeconfigOptions map[string]string

func (opts RotateKubeconfigOptions) Keys() []string {
	keys := make([]string, 0, len(opts))

	for k := range opts {
		keys = append(keys, k)
	}

	return keys
}

var RotateKubeconfigMap = RotateKubeconfigOptions{
	"Admin":             "admin-kubeconfig",
	"ControllerManager": "controller-manager-kubeconfig",
	"Konnectivity":      "konnectivity-kubeconfig",
	"Scheduler":         "scheduler-kubeconfig",
}

// RotateKubeconfig allows rotating the components' kubeconfig resources,
// despite this is already done by Kamaji via its CertificateLifeCycle controller.
// Except for the Admin one, all other components will trigger a Tenant Control Plane reload.
func (h *Helper) RotateKubeconfig(ctx context.Context, namespace, name string, kubeconfig RotateKubeconfigOptions) error {
	for component, suffix := range kubeconfig {
		var secret corev1.Secret
		secret.Name = fmt.Sprintf("%s-%s", name, suffix)
		secret.Namespace = namespace

		if err := h.Client.Delete(ctx, &secret); err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}

			return fmt.Errorf("unable to rotate %s kubeconfig: %w", component, err)
		}
	}

	return nil
}
