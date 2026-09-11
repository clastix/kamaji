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

// InsertArgInSortedSegment inserts arg into the sorted, Kamaji-owned head of args,
// leaving the user's extra args at the tail in their original order.
//
// kube-apiserver args are rendered as a sorted segment followed by the user's
// ExtraArgs verbatim. An addon flag appended past the extras would be sorted back
// into the segment on the next reconcile: same flags, different order, a new
// pod-template hash, and a redundant rollout of the control plane.
func InsertArgInSortedSegment(args, userExtras []string, arg string) []string {
	// The end of the sorted segment is found by matching the extras off the tail of
	// args; extras the merge dropped as duplicates of managed flags are skipped.
	boundary := len(args)

	for i := len(userExtras) - 1; i >= 0 && boundary > 0; i-- {
		if args[boundary-1] == userExtras[i] {
			boundary--
		}
	}

	index := sort.SearchStrings(args[:boundary], arg)

	out := make([]string, 0, len(args)+1)
	out = append(out, args[:index]...)
	out = append(out, arg)
	out = append(out, args[index:]...)

	return out
}
