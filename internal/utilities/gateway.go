// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"strings"

	v1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GetControlPlaneAddressAndPortFromGateway returns the control plane address and
// port associated with a Gateway configuration.
//
// The address is obtained from the provided hostname. Since `v1.Hostname` does
// not include a port (as opposed to the Ingress), it must be determined from the first
// `ParentReference` object.
//
// If no `ParentReference` is provided or its `Port` field is nil, the provided `defaultPort` is used.
// We should never reach this case as it's required at the spec level.
func GetControlPlaneAddressAndPortFromGateway(hostname v1.Hostname, parentRefs []v1.ParentReference, defaultPort int32) (string, int32) {
	gaddr := strings.Split(string(hostname), ":")[0]
	gport := defaultPort

	if len(parentRefs) > 0 && parentRefs[0].Port != nil {
		gport = *parentRefs[0].Port
	}

	return gaddr, gport
}
