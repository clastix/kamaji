// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func GetKindIPAddress() string {
	ep := &corev1.Endpoints{}
	Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: "kubernetes", Namespace: "default"}, ep)).ToNot(HaveOccurred())

	return ep.Subsets[0].Addresses[0].IP
}
