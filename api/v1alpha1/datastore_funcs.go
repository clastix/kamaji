// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetContent is the resolver for the container of the Secret.
// The bare content has priority over the external reference.
func (in *ContentRef) GetContent(ctx context.Context, client client.Client) ([]byte, error) {
	if content := in.Content; len(content) > 0 {
		return content, nil
	}

	secretRef := in.SecretRef

	if secretRef == nil {
		return nil, fmt.Errorf("no bare content and no external Secret reference")
	}

	secret, namespacedName := &corev1.Secret{}, types.NamespacedName{Name: secretRef.Name, Namespace: secretRef.Namespace}
	if err := client.Get(ctx, namespacedName, secret); err != nil {
		return nil, err
	}

	v, ok := secret.Data[string(secretRef.KeyPath)]
	if !ok {
		return nil, fmt.Errorf("secret %s does not have key %s", namespacedName.String(), secretRef.KeyPath)
	}

	return v, nil
}
