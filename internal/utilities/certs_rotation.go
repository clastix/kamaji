// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package utilities

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	RotateCertificateRequestAnnotation = "certs.kamaji.clastix.io/rotate"

	CertificateX509Label       = "x509"
	CertificateKubeconfigLabel = "kubeconfig"
)

func IsRotationRequested(obj client.Object) bool {
	if obj.GetAnnotations() == nil {
		return false
	}

	v, ok := obj.GetAnnotations()[RotateCertificateRequestAnnotation]
	if ok && v == "" {
		return true
	}

	return false
}

func SetLastRotationTimestamp(obj client.Object) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[RotateCertificateRequestAnnotation] = metav1.Now().Format(time.RFC3339)

	obj.SetAnnotations(annotations)
}
