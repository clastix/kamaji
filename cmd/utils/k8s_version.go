// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func KubernetesVersion(config *rest.Config) (string, error) {
	cs, csErr := kubernetes.NewForConfig(config)
	if csErr != nil {
		return "", fmt.Errorf("cannot create kubernetes clientset: %w", csErr)
	}

	sv, svErr := cs.ServerVersion()
	if svErr != nil {
		return "", fmt.Errorf("cannot get Kubernetes version: %w", svErr)
	}

	return sv.GitVersion, nil
}
