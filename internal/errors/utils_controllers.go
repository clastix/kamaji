// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package errors

import "github.com/pkg/errors"

func ShouldReconcileErrorBeIgnored(err error) bool {
	return errors.As(err, &NonExposedLoadBalancerError{}) || errors.As(err, &MissingValidIPError{})
}
