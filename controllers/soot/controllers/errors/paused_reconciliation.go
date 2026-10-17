// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package sooterrors

import "errors"

var ErrPausedReconciliation = errors.New("paused reconciliation, no further actions")
