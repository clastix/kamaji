// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/metrics"
	"github.com/clastix/kamaji/internal/utilities"
)

func TestCertificateRefreshMetrics(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding kamaji scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 scheme: %v", err)
	}

	validCertPEM, err := testCertificatePEM(time.Now().Add(48 * time.Hour))
	if err != nil {
		t.Fatalf("failed generating valid cert: %v", err)
	}
	expiringCertPEM, err := testCertificatePEM(time.Now().Add(30 * time.Minute))
	if err != nil {
		t.Fatalf("failed generating expiring cert: %v", err)
	}

	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&kamajiv1alpha1.TenantControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "tcp-a", Namespace: "default"}},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-valid",
				Namespace: "default",
				Labels: map[string]string{
					constants.ControlPlaneLabelKey:    "tcp-a",
					constants.ControllerLabelResource: utilities.CertificateX509Label,
				},
			},
			Data: map[string][]byte{"tls.crt": validCertPEM},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-expiring",
				Namespace: "default",
				Labels: map[string]string{
					constants.ControlPlaneLabelKey:    "tcp-a",
					constants.ControllerLabelResource: utilities.CertificateX509Label,
				},
			},
			Data: map[string][]byte{"tls.crt": expiringCertPEM},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-invalid",
				Namespace: "default",
				Labels: map[string]string{
					constants.ControlPlaneLabelKey:    "tcp-a",
					constants.ControllerLabelResource: utilities.CertificateX509Label,
				},
			},
			Data: map[string][]byte{"tls.crt": []byte("not-a-certificate")},
		},
	).Build()

	registry := prometheus.NewRegistry()
	recorder := metrics.NewRecorder(registry)

	s := &CertificateLifecycle{
		Deadline: 24 * time.Hour,
		client:   reader,
		Metrics:  recorder,
	}

	if err := s.refreshCertificatesMetrics(t.Context()); err != nil {
		t.Fatalf("refreshCertificatesMetrics returned error: %v", err)
	}

	family := mustMetricFamilyFromGatherer(t, registry, "kamaji_certificates_current")
	if got := gaugeValueByLabels(t, family, map[string]string{"tcp_namespace": "default", "tcp_name": "tcp-a", "status": metrics.CertificateStatusValid, "strategy": metrics.CertificateStrategyX509}); got != 1 {
		t.Fatalf("expected valid/x509 certificates gauge to be 1, got %v", got)
	}
	if got := gaugeValueByLabels(t, family, map[string]string{"tcp_namespace": "default", "tcp_name": "tcp-a", "status": metrics.CertificateStatusExpiring, "strategy": metrics.CertificateStrategyX509}); got != 1 {
		t.Fatalf("expected expiring/x509 certificates gauge to be 1, got %v", got)
	}
	if got := gaugeValueByLabels(t, family, map[string]string{"tcp_namespace": "default", "tcp_name": "tcp-a", "status": metrics.CertificateStatusInvalid, "strategy": metrics.CertificateStrategyX509}); got != 1 {
		t.Fatalf("expected invalid/x509 certificates gauge to be 1, got %v", got)
	}
	if got := gaugeValueByLabels(t, family, map[string]string{"tcp_namespace": "default", "tcp_name": "tcp-a", "status": metrics.CertificateStatusValid, "strategy": metrics.CertificateStrategyKubeconfig}); got != 0 {
		t.Fatalf("expected valid/kubeconfig certificates gauge to be 0, got %v", got)
	}
}

func TestCertificateRefreshMetricsListsOnlyLabelledSecrets(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	if err := kamajiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding kamaji scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed adding corev1 scheme: %v", err)
	}

	validCertPEM, err := testCertificatePEM(time.Now().Add(48 * time.Hour))
	if err != nil {
		t.Fatalf("failed generating valid cert: %v", err)
	}

	var listedSecrets []string

	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&kamajiv1alpha1.TenantControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "tcp-a", Namespace: "default"}},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cert-valid",
				Namespace: "default",
				Labels: map[string]string{
					constants.ControlPlaneLabelKey:    "tcp-a",
					constants.ControllerLabelResource: utilities.CertificateX509Label,
				},
			},
			Data: map[string][]byte{"tls.crt": validCertPEM},
		},
		// Both labels but an unsupported strategy: listed, then skipped by the loop.
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "unsupported-strategy",
				Namespace: "default",
				Labels: map[string]string{
					constants.ControlPlaneLabelKey:    "tcp-a",
					constants.ControllerLabelResource: "unsupported",
				},
			},
		},
		// Secrets missing at least one of the labels, e.g. Helm release storage: not listed.
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "sh.helm.release.v1.app.v1", Namespace: "default"},
			Type:       "helm.sh/release.v1",
			Data:       map[string][]byte{"release": []byte("payload")},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "only-tcp-label",
				Namespace: "default",
				Labels:    map[string]string{constants.ControlPlaneLabelKey: "tcp-a"},
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "only-controller-label",
				Namespace: "other",
				Labels:    map[string]string{constants.ControllerLabelResource: utilities.CertificateX509Label},
			},
		},
	).WithInterceptorFuncs(interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if err := c.List(ctx, list, opts...); err != nil {
				return err
			}

			if secrets, ok := list.(*corev1.SecretList); ok {
				for _, secret := range secrets.Items {
					listedSecrets = append(listedSecrets, secret.Namespace+"/"+secret.Name)
				}
			}

			return nil
		},
	}).Build()

	registry := prometheus.NewRegistry()

	s := &CertificateLifecycle{
		Deadline: 24 * time.Hour,
		client:   reader,
		Metrics:  metrics.NewRecorder(registry),
	}

	if err := s.refreshCertificatesMetrics(t.Context()); err != nil {
		t.Fatalf("refreshCertificatesMetrics returned error: %v", err)
	}

	expectedSecrets := []string{"default/cert-valid", "default/unsupported-strategy"}
	slices.Sort(listedSecrets)
	if !slices.Equal(listedSecrets, expectedSecrets) {
		t.Fatalf("expected %v to be listed, got %v", expectedSecrets, listedSecrets)
	}

	family := mustMetricFamilyFromGatherer(t, registry, "kamaji_certificates_current")
	if got := gaugeValueByLabels(t, family, map[string]string{"tcp_namespace": "default", "tcp_name": "tcp-a", "status": metrics.CertificateStatusValid, "strategy": metrics.CertificateStrategyX509}); got != 1 {
		t.Fatalf("expected valid/x509 certificates gauge to be 1, got %v", got)
	}
}

func testCertificatePEM(notAfter time.Time) ([]byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "metrics-test",
		},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}), nil
}
