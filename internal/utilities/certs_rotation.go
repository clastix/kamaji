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
	// PrunePreviousKeyRequestAnnotation asks Kamaji to drop the verification-only public key
	// retained after a graceful rotation of the ServiceAccount signing key.
	PrunePreviousKeyRequestAnnotation = "certs.kamaji.clastix.io/prune-previous-key"

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

func IsPreviousKeyPruneRequested(obj client.Object) bool {
	_, ok := obj.GetAnnotations()[PrunePreviousKeyRequestAnnotation]

	return ok
}

func RemovePreviousKeyPruneRequest(obj client.Object) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return
	}

	delete(annotations, PrunePreviousKeyRequestAnnotation)

	obj.SetAnnotations(annotations)
}

func SetLastRotationTimestamp(obj client.Object) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[RotateCertificateRequestAnnotation] = metav1.Now().Format(time.RFC3339)

	obj.SetAnnotations(annotations)
}
