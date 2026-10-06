// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

// partialGatewayAPICache behaves like a real cache on a cluster whose RESTMapper does not know some
// gateway.networking.k8s.io kinds: GetInformer fails with NoKindMatchError for them.
// FakeInformers is not safe for concurrent use, hence the mutex.
type partialGatewayAPICache struct {
	*informertest.FakeInformers

	mu     sync.Mutex
	served map[string]bool
}

func (c *partialGatewayAPICache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	gvk, err := apiutil.GVKForObject(obj, c.Scheme)
	if err != nil {
		return nil, err
	}

	if gvk.Group == gatewayv1.GroupName && !c.served[gvk.Kind] {
		return nil, &meta.NoKindMatchError{GroupKind: gvk.GroupKind(), SearchedVersions: []string{gvk.Version}}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.FakeInformers.GetInformer(ctx, obj, opts...)
}

// GKE's managed Gateway API serves the gateway.networking.k8s.io group without GRPCRoute.
func TestTenantControlPlaneControllerStartsWithoutGRPCRoute(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := gatewayv1.Install(scheme); err != nil {
		t.Fatal(err)
	}

	servedKinds := []string{"Gateway", "HTTPRoute", "TLSRoute"}
	served := map[string]bool{}
	apiResources := make([]metav1.APIResource, 0, len(servedKinds))
	for _, kind := range servedKinds {
		served[kind] = true
		apiResources = append(apiResources, metav1.APIResource{Kind: kind})
	}

	discoveryClient := &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{
		Resources: []*metav1.APIResourceList{{
			GroupVersion: gatewayv1.GroupVersion.String(),
			APIResources: apiResources,
		}},
	}}

	mgr, err := ctrl.NewManager(&rest.Config{Host: "https://127.0.0.1:1"}, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		Controller: config.Controller{
			CacheSyncTimeout:   time.Second,
			SkipNameValidation: ptr.To(true),
		},
		NewCache: func(*rest.Config, cache.Options) (cache.Cache, error) {
			return &partialGatewayAPICache{FakeInformers: &informertest.FakeInformers{Scheme: scheme}, served: served}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	reconciler := &TenantControlPlaneReconciler{
		Client:          fake.NewClientBuilder().WithScheme(scheme).Build(),
		DiscoveryClient: discoveryClient,
		TriggerChan:     make(chan event.GenericEvent),
		CertificateChan: make(chan event.GenericEvent),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err = reconciler.SetupWithManager(ctx, mgr); err != nil {
		t.Fatal(err)
	}

	if err = mgr.Start(ctx); err != nil {
		t.Fatalf("expected the manager to run until the context expired, got: %v", err)
	}
}
