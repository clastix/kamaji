// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package soot

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/finalizers"
)

func TestReconcileFailedAnnotationStopsSootManager(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding kamaji scheme: %v", err)
	}

	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "tenant",
			Namespace:   "default",
			Annotations: map[string]string{sootManagerAnnotation: sootManagerFailedAnnotation},
			Finalizers:  []string{finalizers.SootFinalizer},
		},
	}
	tcp.Status.Kubernetes.Version.Status = ptr.To(kamajiv1alpha1.VersionReady)

	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tcp).Build()

	request := reconcile.Request{NamespacedName: types.NamespacedName{Namespace: tcp.Namespace, Name: tcp.Name}}
	completedCh := make(chan struct{})
	cancelled := false

	m := &Manager{
		AdminClient: cl,
		sootMap: sootMap{
			request.String(): {
				cancelFn: func() {
					cancelled = true

					close(completedCh)
				},
				completedCh: completedCh,
			},
		},
	}

	if _, err := m.Reconcile(context.Background(), request); err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}

	if !cancelled {
		t.Fatal("the soot manager was removed without being cancelled")
	}

	if _, ok := m.sootMap[request.String()]; ok {
		t.Fatal("the soot manager is still in the soot map")
	}

	var got kamajiv1alpha1.TenantControlPlane
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(tcp), &got); err != nil {
		t.Fatalf("failed getting the TenantControlPlane: %v", err)
	}

	if _, ok := got.Annotations[sootManagerAnnotation]; ok {
		t.Fatal("the failed annotation was not removed")
	}
}
