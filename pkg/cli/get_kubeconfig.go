// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clastix/kamaji/api/v1alpha1"
)

// GetKubeconfig is the proper function that performs the retrieval of the decoded kubeconfig:
// it's used by the kubectl-kamaji plugin, but it can be used at convenience of any other implementation.
// It returns the decoded content of the administrator kubeconfig, or any other error.
func (h *Helper) GetKubeconfig(ctx context.Context, namespace, name, secretKey string) (*clientcmdapi.Config, []byte, error) {
	var tcp v1alpha1.TenantControlPlane
	if err := h.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &tcp); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("instance %s/%s does not exist", namespace, name)
		}

		return nil, nil, fmt.Errorf("cannot retrieve instance, %w", err)
	}

	namespacedName := client.ObjectKeyFromObject(&tcp)

	if tcp.Status.KubeConfig.Admin.SecretName == "" {
		return nil, nil, fmt.Errorf("kubeconfig for %s is not yet generated", namespacedName.String())
	}

	var secret corev1.Secret
	if err := h.Client.Get(ctx, client.ObjectKey{Namespace: tcp.Namespace, Name: tcp.Status.KubeConfig.Admin.SecretName}, &secret); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil, fmt.Errorf("kubeconfig for %s is not found", namespacedName.String())
		}

		return nil, nil, fmt.Errorf("cannot retrieve Tenant Control Plane kubeconfig Secret, %w", err)
	}

	if secret.Data == nil || secret.Data[secretKey] == nil {
		return nil, nil, fmt.Errorf("key %q does not exist in kubeconfig for %s", secretKey, namespacedName.String())
	}

	kConfig, kErr := clientcmd.Load(secret.Data[secretKey])
	if kErr != nil {
		return nil, nil, fmt.Errorf("cannot load configuration, %w", kErr)
	}

	return kConfig, secret.Data[secretKey], nil
}
