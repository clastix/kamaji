// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

const (
	TriggerChannelBufferSize = 10
	// TriggerChannelTimeout must exceed controller-runtime's CacheSyncTimeout. A receiving
	// controller may still be waiting for its informer cache to finish its initial sync
	// before it starts consuming this channel: controller-runtime blocks a controller's
	// Start() on every registered source, including this one, until all of them have
	// synced, and defaults CacheSyncTimeout to 2 minutes. Once the buffer is full this
	// send is a single best-effort attempt with no retry elsewhere for status-only
	// changes, so its deadline must exceed that cache-sync budget or the event is
	// silently dropped forever.
	TriggerChannelTimeout = 3 * time.Minute
)

func TriggerChannel(ctx context.Context, receiver chan event.GenericEvent, tcp kamajiv1alpha1.TenantControlPlane) {
	deadlineCtx, cancelFn := context.WithTimeout(ctx, TriggerChannelTimeout)
	defer cancelFn()

	select {
	case receiver <- event.GenericEvent{Object: &tcp}:
		return
	case <-deadlineCtx.Done():
		log.FromContext(ctx).Error(deadlineCtx.Err(), "cannot send due to timeout")
	}
}
