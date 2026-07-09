// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func newKamajiTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding kamaji scheme: %v", err)
	}

	return scheme
}

func tenantControlPlaneWithDataStoreStatus(specDataStore, statusDataStore string) *kamajiv1alpha1.TenantControlPlane {
	return &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tcp-a",
			Namespace: "default",
		},
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			DataStore: specDataStore,
		},
		Status: kamajiv1alpha1.TenantControlPlaneStatus{
			Storage: kamajiv1alpha1.StorageStatus{
				DataStoreName: statusDataStore,
			},
		},
	}
}
