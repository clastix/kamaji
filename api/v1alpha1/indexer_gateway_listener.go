// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	GatewayListenerNameKey = "spec.listeners.name"
)

type GatewayListener struct{}

func (g *GatewayListener) Object() client.Object {
	return &gatewayv1.Gateway{}
}

func (g *GatewayListener) Field() string {
	return GatewayListenerNameKey
}

func (g *GatewayListener) ExtractValue() client.IndexerFunc {
	return func(object client.Object) []string {
		gateway := object.(*gatewayv1.Gateway) //nolint:forcetypeassert

		listenerNames := make([]string, 0, len(gateway.Spec.Listeners))
		for _, listener := range gateway.Spec.Listeners {
			// Create a composite key: namespace/gatewayName/listenerName
			// This allows us to look up gateways by listener name while ensuring uniqueness
			key := fmt.Sprintf("%s/%s/%s", gateway.Namespace, gateway.Name, listener.Name)
			listenerNames = append(listenerNames, key)
		}

		return listenerNames
	}
}

func (g *GatewayListener) SetupWithManager(ctx context.Context, mgr controllerruntime.Manager) error {
	return mgr.GetFieldIndexer().IndexField(ctx, g.Object(), g.Field(), g.ExtractValue())
}
