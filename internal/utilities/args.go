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

// InsertArgInSortedSegment inserts arg into the leading, sorted segment of args,
// leaving the trailing segment — the user-provided extra args, deliberately kept
// in their original order — untouched at the end.
//
// The kube-apiserver argument list is rendered as a sorted, Kamaji-owned segment
// followed by the user's ExtraArgs verbatim. An addon flag appended to the very
// end would land after the user extras, and the next reconcile would move it back
// into the sorted segment: the same flags in a different order, hence a different
// pod-template hash and an extra, no-op rollout of the whole control plane.
// Inserting the flag at its final position the first time it is written keeps the
// rendered pod spec a deterministic function of the TenantControlPlane spec.
func InsertArgInSortedSegment(args, userExtras []string, arg string) []string {
	boundary := sortedSegmentLen(args, userExtras)
	index := sort.SearchStrings(args[:boundary], arg)

	out := make([]string, 0, len(args)+1)
	out = append(out, args[:index]...)
	out = append(out, arg)
	out = append(out, args[index:]...)

	return out
}

// sortedSegmentLen reports how many leading elements of args belong to the sorted,
// Kamaji-owned segment, by matching the user extra args off the tail of args.
// Extras the merge dropped (duplicates of Kamaji-managed flags) are simply skipped,
// since the rendered tail is a subsequence of userExtras preserving its order.
func sortedSegmentLen(args, userExtras []string) int {
	boundary := len(args)

	for i := len(userExtras) - 1; i >= 0 && boundary > 0; i-- {
		if args[boundary-1] == userExtras[i] {
			boundary--
		}
	}

	return boundary
}
