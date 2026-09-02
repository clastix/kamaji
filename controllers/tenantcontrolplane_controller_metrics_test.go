// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func TestResolveTenantControlPlaneAddressFromAssignedEndpoint(t *testing.T) {
	t.Parallel()

	tcp := &kamajiv1alpha1.TenantControlPlane{
		Status: kamajiv1alpha1.TenantControlPlaneStatus{
			ControlPlaneEndpoint: "203.0.113.10:6443",
		},
	}

	address := resolveTenantControlPlaneAddress(tcp)
	if address != "https://203.0.113.10:6443" {
		t.Fatalf("expected endpoint address https://203.0.113.10:6443, got %q", address)
	}
}

func TestResolveTenantControlPlaneAddressReturnsEmptyWhenUnresolvable(t *testing.T) {
	tcp := &kamajiv1alpha1.TenantControlPlane{
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Service: kamajiv1alpha1.ServiceSpec{ServiceType: kamajiv1alpha1.ServiceTypeLoadBalancer},
			},
		},
	}

	address := resolveTenantControlPlaneAddress(tcp)
	if address != "" {
		t.Fatalf("expected empty address fallback, got %q", address)
	}
}

func TestClientCASecretEnqueuesReferencingTenantControlPlane(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add Kamaji scheme: %v", err)
	}

	const (
		namespace      = "tenant"
		clientCASecret = "client-ca"
	)
	referencingTCP := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "references-client-ca",
			Namespace: namespace,
			Annotations: map[string]string{
				kamajiv1alpha1.ClientCASecretAnnotation: clientCASecret,
			},
		},
	}
	unrelatedTCP := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unrelated",
			Namespace: namespace,
		},
	}
	reconciler := &TenantControlPlaneReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(referencingTCP, unrelatedTCP).
			Build(),
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: clientCASecret, Namespace: namespace},
	}

	requests := reconciler.clientCASecretToTenantControlPlanes(t.Context(), secret)
	if len(requests) != 1 {
		t.Fatalf("expected one reconcile request, got %d", len(requests))
	}
	if requests[0].Namespace != namespace || requests[0].Name != referencingTCP.Name {
		t.Fatalf("unexpected reconcile request: %#v", requests[0])
	}
}
