// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/cache"
)

// Test fixture namespaces. Pulled out to package-level consts so goconst is
// happy and so an eventual rename does not require a sweeping diff.
const (
	teamA           = "team-a"
	teamB           = "team-b"
	kamajiNamespace = "kamaji-system"
)

// buildNamespaceConfig is a thin map-builder that assumes its input has
// already been canonicalised by MergeWatchedNamespaces. Trim/dedup/blank
// behaviour is exercised by TestMergeWatchedNamespaces; here we only verify
// the empty/nil contract and the zero cache.Config invariant.
func TestBuildNamespaceConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil input keeps cluster-wide cache", nil, nil},
		{"empty slice keeps cluster-wide cache", []string{}, nil},
		{"single namespace", []string{teamA}, []string{teamA}},
		{"multiple namespaces preserve every key", []string{teamA, teamB}, []string{teamA, teamB}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := buildNamespaceConfig(tc.in)

			if tc.want == nil {
				require.Nil(t, got, "expected nil map so the cache stays cluster-wide")

				return
			}

			require.Len(t, got, len(tc.want))

			keys := make([]string, 0, len(got))
			for k, v := range got {
				keys = append(keys, k)
				require.Equal(t, cache.Config{}, v, "namespace %q must use the zero cache.Config so controller-runtime applies its defaults", k)
			}

			sort.Strings(keys)

			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			require.Equal(t, want, keys)
		})
	}
}

func TestBuildNamespaceConfig_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	in := []string{teamA, teamB}
	snapshot := append([]string(nil), in...)

	_ = buildNamespaceConfig(in)

	require.Equal(t, snapshot, in, "buildNamespaceConfig must not mutate its caller-provided slice")
}

func TestApplyOptions_NoNamespacesKeepsCacheClusterWide(t *testing.T) {
	t.Parallel()

	opts := applyOptions(cache.Options{}, 7*time.Hour, nil)

	require.NotNil(t, opts.SyncPeriod, "the resync period must always be applied")
	require.Equal(t, 7*time.Hour, *opts.SyncPeriod)
	require.Nil(t, opts.DefaultNamespaces, "no namespaces means controller-runtime watches cluster-wide")
}

func TestApplyOptions_WithNamespacesScopesCache(t *testing.T) {
	t.Parallel()

	opts := applyOptions(cache.Options{}, time.Minute, []string{teamA, teamB})

	require.NotNil(t, opts.SyncPeriod)
	require.Equal(t, time.Minute, *opts.SyncPeriod)
	require.Len(t, opts.DefaultNamespaces, 2)
	require.Contains(t, opts.DefaultNamespaces, teamA)
	require.Contains(t, opts.DefaultNamespaces, teamB)
}

func TestApplyOptions_AllBlankNamespacesKeepsCacheClusterWide(t *testing.T) {
	t.Parallel()

	opts := applyOptions(cache.Options{}, time.Minute, []string{"", "  "})

	require.Nil(t, opts.DefaultNamespaces, "an effectively-empty namespace list must not collapse the cache to zero namespaces")
}

func TestApplyOptions_PreservesCallerProvidedDefaultNamespacesWhenScopingDisabled(t *testing.T) {
	t.Parallel()

	in := cache.Options{
		DefaultNamespaces: map[string]cache.Config{"caller-set": {}},
	}

	out := applyOptions(in, time.Minute, nil)

	require.Equal(t, in.DefaultNamespaces, out.DefaultNamespaces, "applyOptions must not overwrite a non-empty caller-provided DefaultNamespaces when the user did not request scoping")
}

func TestApplyOptions_UserScopingOverridesCallerProvidedDefaultNamespaces(t *testing.T) {
	t.Parallel()

	in := cache.Options{
		DefaultNamespaces: map[string]cache.Config{"caller-set": {}},
	}

	out := applyOptions(in, time.Minute, []string{teamA})

	require.Len(t, out.DefaultNamespaces, 1)
	require.Contains(t, out.DefaultNamespaces, teamA, "an explicit user-provided namespace list must take precedence over caller defaults")
	require.NotContains(t, out.DefaultNamespaces, "caller-set")
}

// TestApplyOptions_CanonicalisesRawInput verifies that callers passing the
// raw flag value (with whitespace/duplicates/blanks) still produce a clean
// DefaultNamespaces map, because applyOptions runs the input through
// MergeWatchedNamespaces internally.
func TestApplyOptions_CanonicalisesRawInput(t *testing.T) {
	t.Parallel()

	out := applyOptions(cache.Options{}, time.Minute, []string{"  team-a", teamA, "", "team-b\t"})

	require.Len(t, out.DefaultNamespaces, 2)
	require.Contains(t, out.DefaultNamespaces, teamA)
	require.Contains(t, out.DefaultNamespaces, teamB)
}

func TestNewCacheFunc_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	require.NotNil(t, NewCacheFunc(time.Hour, nil))
	require.NotNil(t, NewCacheFunc(time.Hour, []string{teamA}))
}

func TestMergeWatchedNamespaces(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		user     []string
		extras   []string
		expected []string
	}{
		{
			name:     "empty user list returns nil regardless of extras",
			user:     nil,
			extras:   []string{kamajiNamespace},
			expected: nil,
		},
		{
			name:     "all-blank user list returns nil regardless of extras",
			user:     []string{"", "  ", "\t"},
			extras:   []string{kamajiNamespace},
			expected: nil,
		},
		{
			name:     "extras are appended when user provided some namespaces",
			user:     []string{teamA, teamB},
			extras:   []string{kamajiNamespace},
			expected: []string{teamA, teamB, kamajiNamespace},
		},
		{
			name:     "blank extras are dropped",
			user:     []string{teamA},
			extras:   []string{"", " "},
			expected: []string{teamA},
		},
		{
			name:     "duplicates between user and extras collapse, user-first wins",
			user:     []string{teamA, kamajiNamespace},
			extras:   []string{kamajiNamespace},
			expected: []string{teamA, kamajiNamespace},
		},
		{
			name:     "duplicates inside the user list collapse without reordering",
			user:     []string{teamA, teamA, teamB},
			extras:   nil,
			expected: []string{teamA, teamB},
		},
		{
			name:     "whitespace is normalised before deduplication",
			user:     []string{"  team-a", "team-a "},
			extras:   []string{"\tteam-a"},
			expected: []string{teamA},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := MergeWatchedNamespaces(tc.user, tc.extras...)
			require.Equal(t, tc.expected, got)
		})
	}
}

func TestMergeWatchedNamespaces_DoesNotMutateInput(t *testing.T) {
	t.Parallel()

	user := []string{" team-a ", teamA}
	extras := []string{kamajiNamespace}

	userSnapshot := append([]string(nil), user...)
	extrasSnapshot := append([]string(nil), extras...)

	_ = MergeWatchedNamespaces(user, extras...)

	require.Equal(t, userSnapshot, user, "MergeWatchedNamespaces must not mutate the user slice")
	require.Equal(t, extrasSnapshot, extras, "MergeWatchedNamespaces must not mutate the extras slice")
}

func TestValidateNamespaces(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		in        []string
		expectErr bool
	}{
		{"nil is valid", nil, false},
		{"empty slice is valid", []string{}, false},
		{"valid single label", []string{teamA}, false},
		{"valid multiple labels", []string{teamA, teamB, "kube-system"}, false},
		{"surrounding whitespace is tolerated", []string{"  team-a ", "\tteam-b"}, false},
		{"blank entries are tolerated and ignored", []string{teamA, "", "  "}, false},
		{"uppercase rejected", []string{"Team-A"}, true},
		{"underscore rejected", []string{"team_a"}, true},
		{"leading hyphen rejected", []string{"-team-a"}, true},
		{"trailing hyphen rejected", []string{"team-a-"}, true},
		{"too long rejected", []string{strings.Repeat("a", 64)}, true},
		{"empty after trim is allowed (caller skips it)", []string{" "}, false},
		{"valid + invalid mix surfaces error", []string{teamA, "Bad Name"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateNamespaces(tc.in)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
