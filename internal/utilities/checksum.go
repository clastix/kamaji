// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
)

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

	return MD5Checksum([]byte(checksum))
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

	return MD5Checksum([]byte(checksum))
}

func MD5Checksum(value []byte) string {
	hash := md5.Sum(value)

	return hex.EncodeToString(hash[:])
}
