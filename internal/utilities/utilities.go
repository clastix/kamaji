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

// EncondeToYaml returns the given object in yaml format and the error.
func EncondeToYaml(o runtime.Object) ([]byte, error) {
	scheme := runtime.NewScheme()
	encoder := json.NewYAMLSerializer(json.SimpleMetaFactory{}, scheme, scheme)
	buf := bytes.NewBuffer([]byte{})
	err := encoder.Encode(o, buf)

	return buf.Bytes(), err
}
