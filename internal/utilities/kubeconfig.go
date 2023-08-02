// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

func DecodeKubeconfig(secret corev1.Secret, key string) (*clientcmdapiv1.Config, error) {
	bytes, ok := secret.Data[key]
	if !ok {
		return nil, fmt.Errorf("%s is not into kubeconfig secret", key)
	}

	return DecodeKubeconfigYAML(bytes)
}

func DecodeKubeconfigYAML(bytes []byte) (*clientcmdapiv1.Config, error) {
	kubeconfig := &clientcmdapiv1.Config{}
	if err := DecodeFromYAML(string(bytes), kubeconfig); err != nil {
		return nil, err
	}

	return kubeconfig, nil
}
