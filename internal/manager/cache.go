// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

// Package manager hosts helpers that wire Kamaji-specific configuration into
// controller-runtime's manager and cache.
package manager

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

// NewCacheFunc returns a cache.NewCacheFunc suitable for ctrl.Options.NewCache.
//
// The returned function applies the given resync period to every informer and,
// when namespaces is non-empty, restricts namespaced informers to that set via
// cache.Options.DefaultNamespaces. Cluster-scoped resources (CRDs, ClusterRole,
// ClusterRoleBinding, ValidatingWebhookConfiguration, the cluster-scoped
// DataStore CRD, ...) are never affected by namespace scoping.
//
// The helper does not implicitly add any namespace to the set: callers that
// must keep watching their own install namespace are responsible for including
// it before invoking NewCacheFunc.
func NewCacheFunc(resyncPeriod time.Duration, namespaces []string) cache.NewCacheFunc {
	return func(config *rest.Config, opts cache.Options) (cache.Cache, error) {
		return cache.New(config, applyOptions(opts, resyncPeriod, namespaces))
	}
}

// MergeWatchedNamespaces composes the final list of namespaces a manager must
// keep in its cache scope. When the user-supplied list is empty (nil, zero
// length, or only whitespace entries) the result is nil and cluster-wide
// caching stays on — extras are intentionally dropped in that case so a
// stray "--watch-namespaces= " on the command line cannot silently scope the
// cache to the install namespace alone. Otherwise the user list is
// concatenated with extras, then trimmed, deduplicated and stripped of
// blanks while preserving first-seen order so log lines and informer setup
// stay deterministic. Neither input slice is mutated.
//
// Typical use: the caller passes the operator's install namespace through
// extras to guarantee the migration Job watch keeps working when scoping is
// enabled.
func MergeWatchedNamespaces(user []string, extras ...string) []string {
	hasUserEntry := false

	for _, v := range user {
		if strings.TrimSpace(v) != "" {
			hasUserEntry = true

			break
		}
	}

	if !hasUserEntry {
		return nil
	}

	out := make([]string, 0, len(user)+len(extras))
	seen := make(map[string]struct{}, len(user)+len(extras))

	add := func(values []string) {
		for _, v := range values {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}

			if _, ok := seen[v]; ok {
				continue
			}

			seen[v] = struct{}{}
			out = append(out, v)
		}
	}

	add(user)
	add(extras)

	return out
}

// ValidateNamespaces checks that every non-blank entry in namespaces is a
// valid Kubernetes namespace identifier — an RFC 1123 label, max 63
// characters, lowercase alphanumerics and dashes only, no leading or
// trailing dash. Blank entries are ignored — they are dropped later by the
// cache builder anyway. The function returns a single error aggregating
// every offender so that the caller can surface them all at once.
func ValidateNamespaces(namespaces []string) error {
	var invalid []string

	for _, ns := range namespaces {
		ns = strings.TrimSpace(ns)
		if ns == "" {
			continue
		}

		if errs := validation.IsDNS1123Label(ns); len(errs) > 0 {
			invalid = append(invalid, fmt.Sprintf("%q: %s", ns, strings.Join(errs, "; ")))
		}
	}

	if len(invalid) == 0 {
		return nil
	}

	return fmt.Errorf("invalid --watch-namespaces value(s): %s", strings.Join(invalid, ", "))
}

// buildNamespaceConfig converts a canonicalised namespace list into a map
// suitable for cache.Options.DefaultNamespaces. The caller is expected to
// have already trimmed, deduplicated and stripped blanks via
// MergeWatchedNamespaces — this helper does no further normalisation. The
// result is nil when the effective set is empty so that callers can detect
// "no scoping requested" and leave the cache cluster-wide.
func buildNamespaceConfig(namespaces []string) map[string]cache.Config {
	if len(namespaces) == 0 {
		return nil
	}

	out := make(map[string]cache.Config, len(namespaces))
	for _, ns := range namespaces {
		out[ns] = cache.Config{}
	}

	return out
}

// applyOptions returns a new cache.Options with the configured resync period
// and (optionally) namespace scoping applied. The namespaces slice is
// canonicalised via MergeWatchedNamespaces before being installed, so callers
// that pass the raw flag value still end up with a deterministic cache
// configuration. Kept package-private so we can unit-test the behaviour
// without spinning up a real cache.
func applyOptions(opts cache.Options, resyncPeriod time.Duration, namespaces []string) cache.Options {
	opts.SyncPeriod = &resyncPeriod

	if cfg := buildNamespaceConfig(MergeWatchedNamespaces(namespaces)); len(cfg) > 0 {
		opts.DefaultNamespaces = cfg
	}

	return opts
}
