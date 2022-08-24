// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import "sigs.k8s.io/controller-runtime/pkg/event"

type TenantControlPlaneChannel chan event.GenericEvent
