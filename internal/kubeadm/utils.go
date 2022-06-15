// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"bytes"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

func EncondeToYaml(o runtime.Object) ([]byte, error) {
	scheme := runtime.NewScheme()
	encoder := json.NewSerializerWithOptions(json.SimpleMetaFactory{}, scheme, scheme, json.SerializerOptions{})
	buf := bytes.NewBuffer([]byte{})
	err := encoder.Encode(o, buf)

	return buf.Bytes(), err
}
