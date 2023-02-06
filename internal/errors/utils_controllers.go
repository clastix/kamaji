// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package errors

import "github.com/pkg/errors"

func ShouldReconcileErrorBeIgnored(err error) bool {
	switch {
	case errors.As(err, &NonExposedLoadBalancerError{}):
		return true
	case errors.As(err, &MissingValidIPError{}):
		return true
	case errors.As(err, &MigrationInProcessError{}):
		return true
	default:
		return false
	}
}
