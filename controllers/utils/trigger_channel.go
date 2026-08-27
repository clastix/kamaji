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

// CoalesceTriggerChannelBufferSize is the buffer size of the channels fed by CoalesceTriggerChannel,
// and one slot is both the minimum and the maximum that makes sense.
//
// It cannot be zero: a send on an unbuffered channel only succeeds when the receiver is already
// parked on it, so every nudge would be skipped in the window where the source.Channel consumer
// hasn't started yet, which is precisely the window the send has to survive.
//
// It doesn't need to be more than one: all the events are interchangeable nudges for the same
// workqueue key, so a pending one already tells the consumer to reconcile, and the workqueue
// collapses anything queued on top of it. Extra slots would only buy redundant sends.
const CoalesceTriggerChannelBufferSize = 1

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

// CoalesceTriggerChannel enqueues a reconciliation for the given TenantControlPlane without ever
// blocking: it's meant for channels dedicated to a single TenantControlPlane, where all the events
// are interchangeable nudges for the same workqueue key. When the buffer is full a reconciliation
// is already pending and covers this request, so skipping the send coalesces it rather than losing
// it: no goroutine, and no deadline to tune.
//
// Channels shared by several TenantControlPlane objects carry a distinct workqueue key per event
// and cannot use this: they have to wait for the consumer instead of skipping the send.
func CoalesceTriggerChannel(receiver chan event.GenericEvent, tcp kamajiv1alpha1.TenantControlPlane) {
	select {
	case receiver <- event.GenericEvent{Object: &tcp}:
	default:
	}
}
