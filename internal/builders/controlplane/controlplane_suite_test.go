// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestControlpalne(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControlPlane Suite")
}
