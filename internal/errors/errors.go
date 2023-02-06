// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package errors

type MigrationInProcessError struct{}

func (n MigrationInProcessError) Error() string {
	return "cannot continue reconciliation, the current TenantControlPlane is still in migration status"
}

type NonExposedLoadBalancerError struct{}

func (n NonExposedLoadBalancerError) Error() string {
	return "cannot retrieve the TenantControlPlane address, Service resource is not yet exposed as LoadBalancer"
}

type MissingValidIPError struct{}

func (m MissingValidIPError) Error() string {
	return "the actual resource doesn't have yet a valid IP address"
}
