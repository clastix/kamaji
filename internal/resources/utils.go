// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"math/rand"
	"time"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var letters = []byte("abcdefghijklmnopqrstuvwxyz")

func updateOperationResult(current controllerutil.OperationResult, op controllerutil.OperationResult) controllerutil.OperationResult {
	if current == controllerutil.OperationResultCreated || op == controllerutil.OperationResultCreated {
		return controllerutil.OperationResultCreated
	}

	if current == controllerutil.OperationResultUpdated || op == controllerutil.OperationResultUpdated {
		return controllerutil.OperationResultUpdated
	}

	if current == controllerutil.OperationResultUpdatedStatus || op == controllerutil.OperationResultUpdatedStatus {
		return controllerutil.OperationResultUpdatedStatus
	}

	if current == controllerutil.OperationResultUpdatedStatusOnly || op == controllerutil.OperationResultUpdatedStatusOnly {
		return controllerutil.OperationResultUpdatedStatusOnly
	}

	return controllerutil.OperationResultNone
}

func secretProjection(secretName, certKeyName, keyName string) *v1.SecretProjection {
	return &v1.SecretProjection{
		LocalObjectReference: v1.LocalObjectReference{
			Name: secretName,
		},
		Items: []v1.KeyToPath{
			{
				Key:  certKeyName,
				Path: certKeyName,
			},
			{
				Key:  keyName,
				Path: keyName,
			},
		},
	}
}

func randomString(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return string(b)
}

func getLatestConfigRV(tenantControlPlane kamajiv1alpha1.TenantControlPlane) string {
	return tenantControlPlane.Status.KubeadmConfig.Checksum
}
