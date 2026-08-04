// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"crypto/md5"
	"encoding/hex"
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clastix/kamaji/internal/constants"
)

// GetObjectChecksum returns the annotation checksum in case this is set,
// otherwise, an empty string.
func GetObjectChecksum(obj client.Object) string {
	v, ok := obj.GetAnnotations()[constants.Checksum]
	if !ok {
		return ""
	}

	return v
}

// SetObjectChecksum calculates the checksum for the given map and store it in the object annotations.
func SetObjectChecksum(obj client.Object, data any) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[constants.Checksum] = CalculateMapChecksum(data)

	obj.SetAnnotations(annotations)
}

// CalculateStringSliceChecksum calculates a checksum of slice, calculating the overall md5 of each values.
// It takes order into account, as in :
//
//	CalculateStringSliceChecksum([a, b]) != CalculateStringSliceChecksum([b, a]) // (if a != b)
func CalculateStringSliceChecksum(data []string) (string, error) {
	h := md5.New()
	for _, s := range data {
		if _, err := h.Write([]byte(s)); err != nil {
			return "", err
		}
		// Add separator to avoid collisions (e.g., ["ab", "c"] vs ["a", "bc"])
		if _, err := h.Write([]byte{0}); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(h.Sum([]byte{})), nil
}

// CalculateMapChecksum orders the map according to its key, and calculating the overall md5 of the values.
// It's expected to work with ConfigMap (map[string]string) and Secrets (map[string][]byte).
func CalculateMapChecksum(data any) string {
	switch t := data.(type) {
	case map[string]string:
		return calculateMapStringString(t)
	case map[string][]byte:
		return calculateMapStringByte(t)
	default:
		return ""
	}
}

func calculateMapStringString(data map[string]string) string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	var checksum string

	for _, key := range keys {
		checksum += data[key]
	}

	return md5Checksum([]byte(checksum))
}

func calculateMapStringByte(data map[string][]byte) string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	var checksum string

	for _, key := range keys {
		checksum += string(data[key])
	}

	return md5Checksum([]byte(checksum))
}

func md5Checksum(value []byte) string {
	hash := md5.Sum(value)

	return hex.EncodeToString(hash[:])
}
