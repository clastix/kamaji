// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	fakeClient client.Client
	scheme     *runtime.Scheme = runtime.NewScheme()
)

func TestDatastore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Datastore Suite")
}
