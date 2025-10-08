// Copyright 2025 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestControlPlane(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlane Suite")
}
