// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package errors

import "errors"

func ShouldReconcileErrorBeIgnored(err error) bool {
	var (
		nonExposedLBErr   NonExposedLoadBalancerError
		missingValidIPErr MissingValidIPError
		migrationErr      MigrationInProcessError
	)

	switch {
	case errors.As(err, &nonExposedLBErr):
		return true
	case errors.As(err, &missingValidIPErr):
		return true
	case errors.As(err, &migrationErr):
		return true
	default:
		return false
	}
}
