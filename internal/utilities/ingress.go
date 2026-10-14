// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"net"
	"strconv"
	"strings"
)

func GetControlPlaneAddressAndPortFromHostname(hostname string, defaultPort int32) (address string, port int32) {
	host, rawPort, err := net.SplitHostPort(hostname)
	if err != nil {
		// No port, or a bare IPv6 literal: the whole string is the address,
		// except for a bracketed IPv6 literal that has to be unwrapped.
		return strings.TrimSuffix(strings.TrimPrefix(hostname, "["), "]"), defaultPort
	}

	port = defaultPort

	if intPort, _ := strconv.Atoi(rawPort); intPort > 0 {
		port = int32(intPort)
	}

	return host, port
}
