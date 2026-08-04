// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"sigs.k8s.io/controller-runtime/pkg/event"

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
