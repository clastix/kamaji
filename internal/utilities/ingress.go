// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"strconv"
	"strings"
)

func GetControlPlaneAddressAndPortFromHostname(hostname string, defaultPort int32) (address string, port int32) {
	parts := strings.Split(hostname, ":")

	address, port = parts[0], defaultPort

	if len(parts) == 2 {
		intPort, _ := strconv.Atoi(parts[1])

		if intPort > 0 {
			port = int32(intPort)
		}
	}

	return address, port
}
