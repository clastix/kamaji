// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"context"
	"fmt"

	"gomodules.xyz/jsonpatch/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/webhook/utils"
)

type TenantControlPlanePreGeneratedCerts struct {
	Client client.Client
}

func (t TenantControlPlanePreGeneratedCerts) ValidatePreGeneratedCerts(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane) error {
	// Check mutual exclusivity between preGeneratedCertificates and certSANs
	if tcp.Spec.PreGeneratedCertificates != nil && len(tcp.Spec.NetworkProfile.CertSANs) > 0 {
		return fmt.Errorf("preGeneratedCertificates cannot be specified when certSANs is configured - use either auto-generated certificates with certSANs OR pregenerated certificates, not both")
	}

	if tcp.Spec.PreGeneratedCertificates == nil {
		return nil
	}

	preGenCerts := tcp.Spec.PreGeneratedCertificates

	// Validate CA certificate if specified
	if preGenCerts.CA != nil {
		if err := t.validateCertificateReference(ctx, tcp, preGenCerts.CA, "ca"); err != nil {
			return fmt.Errorf("invalid CA certificate: %w", err)
		}
	}

	// Validate API Server certificate if specified
	if preGenCerts.APIServer != nil {
		if err := t.validateCertificateReference(ctx, tcp, preGenCerts.APIServer, "apiServer"); err != nil {
			return fmt.Errorf("invalid API Server certificate: %w", err)
		}
	}

	// Validate Kubelet client certificate if specified
	if preGenCerts.KubeletClient != nil {
		if err := t.validateCertificateReference(ctx, tcp, preGenCerts.KubeletClient, "kubeletClient"); err != nil {
			return fmt.Errorf("invalid Kubelet client certificate: %w", err)
		}
	}

	// Validate Front Proxy CA certificate if specified
	if preGenCerts.FrontProxyCA != nil {
		if err := t.validateCertificateReference(ctx, tcp, preGenCerts.FrontProxyCA, "frontProxyCA"); err != nil {
			return fmt.Errorf("invalid Front Proxy CA certificate: %w", err)
		}
	}

	// Validate Front Proxy client certificate if specified
	if preGenCerts.FrontProxyClient != nil {
		if err := t.validateCertificateReference(ctx, tcp, preGenCerts.FrontProxyClient, "frontProxyClient"); err != nil {
			return fmt.Errorf("invalid Front Proxy client certificate: %w", err)
		}
	}

	// Validate Service Account key if specified
	if preGenCerts.ServiceAccount != nil {
		if err := t.validateKeyReference(ctx, tcp, preGenCerts.ServiceAccount, "serviceAccount"); err != nil {
			return fmt.Errorf("invalid Service Account key: %w", err)
		}
	}

	return nil
}

func (t TenantControlPlanePreGeneratedCerts) validateCertificateReference(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane, certRef *kamajiv1alpha1.CertificateReference, _ string) error {
	// Determine the namespace for the secret
	secretNamespace := certRef.SecretNamespace
	if secretNamespace == "" {
		secretNamespace = tcp.GetNamespace()
	}

	// Get the referenced secret
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      certRef.SecretName,
		Namespace: secretNamespace,
	}

	if err := t.Client.Get(ctx, secretKey, secret); err != nil {
		return fmt.Errorf("failed to get secret %s: %w", secretKey, err)
	}

	// Determine certificate and private key keys
	certKey := certRef.CertificateKey
	if certKey == "" {
		certKey = corev1.TLSCertKey
	}

	privKeyKey := certRef.PrivateKeyKey
	if privKeyKey == "" {
		privKeyKey = corev1.TLSPrivateKeyKey
	}

	// Check if the required keys exist in the secret
	certData, exists := secret.Data[certKey]
	if !exists {
		return fmt.Errorf("certificate key %s not found in secret %s", certKey, secretKey)
	}

	privKeyData, exists := secret.Data[privKeyKey]
	if !exists {
		return fmt.Errorf("private key %s not found in secret %s", privKeyKey, secretKey)
	}

	// Validate the certificate and private key pair
	isValid, err := crypto.CheckCertificateAndPrivateKeyPairValidity(certData, privKeyData, 0)
	if err != nil {
		return fmt.Errorf("failed to validate certificate-private key pair: %w", err)
	}

	if !isValid {
		return fmt.Errorf("certificate-private key pair is not valid")
	}

	return nil
}

func (t TenantControlPlanePreGeneratedCerts) validateKeyReference(ctx context.Context, tcp *kamajiv1alpha1.TenantControlPlane, keyRef *kamajiv1alpha1.KeyReference, _ string) error {
	// Determine the namespace for the secret
	secretNamespace := keyRef.SecretNamespace
	if secretNamespace == "" {
		secretNamespace = tcp.GetNamespace()
	}

	// Get the referenced secret
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      keyRef.SecretName,
		Namespace: secretNamespace,
	}

	if err := t.Client.Get(ctx, secretKey, secret); err != nil {
		return fmt.Errorf("failed to get secret %s: %w", secretKey, err)
	}

	// Determine public and private key keys
	pubKeyKey := keyRef.PublicKeyKey
	if pubKeyKey == "" {
		pubKeyKey = kubeadmconstants.ServiceAccountPublicKeyName
	}

	privKeyKey := keyRef.PrivateKeyKey
	if privKeyKey == "" {
		privKeyKey = kubeadmconstants.ServiceAccountPrivateKeyName
	}

	// Check if the required keys exist in the secret
	pubKeyData, exists := secret.Data[pubKeyKey]
	if !exists {
		return fmt.Errorf("public key %s not found in secret %s", pubKeyKey, secretKey)
	}

	privKeyData, exists := secret.Data[privKeyKey]
	if !exists {
		return fmt.Errorf("private key %s not found in secret %s", privKeyKey, secretKey)
	}

	// Basic validation - ensure keys are parseable
	if _, err := crypto.ParsePrivateKeyBytes(privKeyData); err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	if _, err := crypto.ParsePublicKeyBytes(pubKeyData); err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	return nil
}

func (t TenantControlPlanePreGeneratedCerts) OnCreate(obj runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := obj.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		return nil, t.ValidatePreGeneratedCerts(ctx, tcp)
	}
}

func (t TenantControlPlanePreGeneratedCerts) OnDelete(runtime.Object) AdmissionResponse {
	return utils.NilOp()
}

func (t TenantControlPlanePreGeneratedCerts) OnUpdate(newObject runtime.Object, prevObject runtime.Object) AdmissionResponse {
	return func(ctx context.Context, req admission.Request) ([]jsonpatch.JsonPatchOperation, error) {
		tcp := newObject.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert

		return nil, t.ValidatePreGeneratedCerts(ctx, tcp)
	}
}
