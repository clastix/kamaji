// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"bytes"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/constants"
)

const (
	separator = "-"
)

func KamajiLabels() map[string]string {
	return map[string]string{
		constants.ProjectNameLabelKey: constants.ProjectNameLabelValue,
	}
}

func CommonLabels(clusterName string) map[string]string {
	return map[string]string{
		"kamaji.clastix.io/type":    "cluster",
		"kamaji.clastix.io/cluster": clusterName,
	}
}

func MergeMaps(maps ...map[string]string) map[string]string {
	result := map[string]string{}

	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}

	return result
}

func AddTenantPrefix(name string, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) string {
	return fmt.Sprintf("%s%s%s", tenantControlPlane.GetName(), separator, name)
}

// EncodeToYaml returns the given object in yaml format and the error.
func EncodeToYaml(o runtime.Object) ([]byte, error) {
	scheme := runtime.NewScheme()

	encoder := json.NewSerializerWithOptions(json.SimpleMetaFactory{}, scheme, scheme, json.SerializerOptions{
		Yaml:   true,
		Pretty: false,
		Strict: false,
	})

	buf := bytes.NewBuffer([]byte{})

	if err := encoder.Encode(o, buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func DecodeFromYAML(o string, to runtime.Object) (err error) {
	scheme := runtime.NewScheme()

	encoder := json.NewSerializerWithOptions(json.SimpleMetaFactory{}, scheme, scheme, json.SerializerOptions{
		Yaml:   true,
		Pretty: false,
		Strict: false,
	})

	if to, _, err = encoder.Decode([]byte(o), nil, to); err != nil { //nolint:ineffassign,staticcheck
		return
	}

	return
}

func DecodeFromJSON(o string, to runtime.Object) (err error) {
	scheme := runtime.NewScheme()

	encoder := json.NewSerializerWithOptions(json.SimpleMetaFactory{}, scheme, scheme, json.SerializerOptions{
		Yaml:   false,
		Pretty: false,
		Strict: false,
	})

	if to, _, err = encoder.Decode([]byte(o), nil, to); err != nil { //nolint:ineffassign,staticcheck
		return
	}

	return
}

// EncodeToJSON returns the given object in JSON format and the error, respecting the Kubernetes struct tags.
func EncodeToJSON(o runtime.Object) ([]byte, error) {
	scheme := runtime.NewScheme()

	encoder := json.NewSerializerWithOptions(json.SimpleMetaFactory{}, scheme, scheme, json.SerializerOptions{
		Yaml:   false,
		Pretty: false,
		Strict: false,
	})

	buf := bytes.NewBuffer([]byte{})

	if err := encoder.Encode(o, buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
