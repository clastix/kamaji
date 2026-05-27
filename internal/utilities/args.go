// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"fmt"
	"sort"
	"strings"
)

// ArgsFromSliceToMap transforms a slice of string into a map, simplifying the subsequent mangling.
func ArgsFromSliceToMap(args []string) (m map[string]string) {
	m = make(map[string]string)

	for _, arg := range args {
		flag, value, _ := strings.Cut(arg, "=")

		m[flag] = value
	}

	return m
}

// ArgsFromMapToSlice create the slice of args, and sorting the resulting output in order to make it idempotent.
// Along with that, if a flag doesn't have a value, it's presented barely without a value assignment.
func ArgsFromMapToSlice(args map[string]string) (slice []string) {
	for flag, value := range args {
		if len(value) == 0 {
			slice = append(slice, flag)

			continue
		}

		slice = append(slice, fmt.Sprintf("%s=%s", flag, value))
	}

	sort.Strings(slice)

	return slice
}
