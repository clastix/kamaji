// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func KubernetesVersion(config *rest.Config) (string, error) {
	cs, csErr := kubernetes.NewForConfig(config)
	if csErr != nil {
		return "", errors.Wrap(csErr, "cannot create kubernetes clientset")
	}

	sv, svErr := cs.ServerVersion()
	if svErr != nil {
		return "", errors.Wrap(svErr, "cannot get Kubernetes version")
	}

	return sv.GitVersion, nil
}
