// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
)

// CalculateConfigMapChecksum orders the map according to its key, and calculating the overall md5 of the values.
func CalculateConfigMapChecksum(data map[string]string) string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	var checksum string

	for _, key := range keys {
		checksum += data[key]
	}

	hash := md5.Sum([]byte(checksum))

	return hex.EncodeToString(hash[:])
}
