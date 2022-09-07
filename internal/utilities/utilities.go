// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"net"
	"regexp"
	"sort"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// IsValidIP checks if the given argument is an IP.
func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// IsValidHostname checks if the given argument is a valid hostname.
func IsValidHostname(hostname string) bool {
	pattern := "^([a-z0-9]|[a-z0-9][a-z0-9-]{0,61}[a-z0-9])(\\.([a-z0-9]|[a-z0-9][a-z0-9-]{0,61}[a-z0-9]))*$"

	return validateRegex(pattern, hostname)
}

func validateRegex(pattern string, value string) bool {
	isFound, err := regexp.MatchString(pattern, value)
	if err != nil {
		return false
	}

	return isFound
}

func GenerateUUID() uuid.UUID {
	return uuid.New()
}

func GenerateUUIDString() string {
	return GenerateUUID().String()
}

// SecretHashValue function returns the md5 value for the secret of the given name and namespace.
func SecretHashValue(ctx context.Context, client client.Client, namespace, name string) (string, error) {
	secret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
		return "", errors.Wrap(err, "cannot retrieve *corev1.Secret for resource version retrieval")
	}

	return HashValue(*secret), nil
}

// HashValue function returns the md5 value for the given secret.
func HashValue(secret corev1.Secret) string {
	// Go access map values in random way, it means we have to sort them.
	keys := make([]string, 0, len(secret.Data))

	for k := range secret.Data {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	// Generating MD5 of Secret values, sorted by key
	h := md5.New()

	for _, key := range keys {
		h.Write(secret.Data[key])
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}
