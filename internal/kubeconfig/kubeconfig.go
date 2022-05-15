// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeconfig

import (
	"bytes"

	"k8s.io/apimachinery/pkg/util/yaml"
	v1 "k8s.io/client-go/tools/clientcmd/api/v1"
)

type Kubeconfig v1.Config

func GetKubeconfigFromBytesBuffer(buffer *bytes.Buffer) (*Kubeconfig, error) {
	kubeconfig := &Kubeconfig{}
	if err := yaml.NewYAMLOrJSONDecoder(buffer, buffer.Len()).Decode(kubeconfig); err != nil {
		return nil, err
	}

	return kubeconfig, nil
}

func GetKubeconfigFromBytes(b []byte) (*Kubeconfig, error) {
	buffer := bytes.NewBuffer(b)

	return GetKubeconfigFromBytesBuffer(buffer)
}
