// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package konnectivity_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
	"github.com/clastix/kamaji/internal/utilities"
)

var _ = Describe("KonnectivityKubeconfigResource", func() {
	It("regenerates kubeconfig when embedded CA is stale", func() {
		ctx := context.Background()

		oldCACert, oldCAKey := createSelfSignedCA("old-ca")
		newCACert, newCAKey := createSelfSignedCA("new-ca")

		oldClientCert, oldClientKey := createSignedKonnectivityCert(oldCACert, oldCAKey)
		newClientCert, newClientKey := createSignedKonnectivityCert(newCACert, newCAKey)

		tcp := &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tenant-01",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{Port: 6443},
				Addons: kamajiv1alpha1.AddonsSpec{
					Konnectivity: &kamajiv1alpha1.KonnectivitySpec{},
				},
			},
			Status: kamajiv1alpha1.TenantControlPlaneStatus{
				Certificates: kamajiv1alpha1.CertificatesStatus{
					CA: kamajiv1alpha1.CertificatePrivateKeyPairStatus{SecretName: "tenant-01-ca"},
				},
				Addons: kamajiv1alpha1.AddonsStatus{
					Konnectivity: kamajiv1alpha1.KonnectivityStatus{
						Certificate: kamajiv1alpha1.CertificatePrivateKeyPairStatus{SecretName: "tenant-01-konnectivity-certificate"},
					},
				},
			},
		}

		staleKubeconfig := renderKubeconfig(oldCACert, oldClientCert, oldClientKey)

		kubeconfigSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tenant-01-konnectivity-kubeconfig",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"konnectivity-server.conf": staleKubeconfig,
			},
		}
		utilities.SetObjectChecksum(kubeconfigSecret, kubeconfigSecret.Data)
		tcp.Status.Addons.Konnectivity.Kubeconfig.Checksum = utilities.GetObjectChecksum(kubeconfigSecret)

		caSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "tenant-01-ca", Namespace: "default"},
			Data: map[string][]byte{
				kubeadmconstants.CACertName: newCACert,
				kubeadmconstants.CAKeyName:  newCAKey,
			},
		}

		certificateSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "tenant-01-konnectivity-certificate", Namespace: "default"},
			Data: map[string][]byte{
				corev1.TLSCertKey:       newClientCert,
				corev1.TLSPrivateKeyKey: newClientKey,
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(runtimeScheme).
			WithObjects(kubeconfigSecret, caSecret, certificateSecret).
			Build()

		resource := &konnectivity.KubeconfigResource{Client: fakeClient}
		Expect(resource.Define(ctx, tcp)).To(Succeed())

		_, err := resource.CreateOrUpdate(ctx, tcp)
		Expect(err).NotTo(HaveOccurred())

		reconciled := &corev1.Secret{}
		Expect(fakeClient.Get(ctx, types.NamespacedName{Name: "tenant-01-konnectivity-kubeconfig", Namespace: "default"}, reconciled)).To(Succeed())

		reconciledKubeconfig := decodeKubeconfig(reconciled.Data["konnectivity-server.conf"])
		Expect(reconciledKubeconfig.Clusters).To(HaveLen(1))
		Expect(reconciledKubeconfig.Clusters[0].Cluster.CertificateAuthorityData).To(Equal(newCACert))
		Expect(reconciledKubeconfig.AuthInfos).To(HaveLen(1))
		Expect(reconciledKubeconfig.AuthInfos[0].AuthInfo.ClientCertificateData).To(Equal(newClientCert))
		Expect(reconciledKubeconfig.AuthInfos[0].AuthInfo.ClientKeyData).To(Equal(newClientKey))
	})
})

func createSelfSignedCA(commonName string) ([]byte, []byte) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now().Add(-1 * time.Minute),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	Expect(err).NotTo(HaveOccurred())

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return certPEM, keyPEM
}

func createSignedKonnectivityCert(caCert, caKey []byte) ([]byte, []byte) {
	template := crypto.NewCertificateTemplate(konnectivity.CertCommonName)
	template.NotBefore = time.Now().Add(-1 * time.Minute)
	template.NotAfter = time.Now().Add(24 * time.Hour)

	cert, key, err := crypto.GenerateCertificatePrivateKeyPair(template, caCert, caKey)
	Expect(err).NotTo(HaveOccurred())

	return cert.Bytes(), key.Bytes()
}

func renderKubeconfig(ca, cert, key []byte) []byte {
	cfg := &clientcmdapiv1.Config{
		Kind:       "Config",
		APIVersion: "v1",
		AuthInfos: []clientcmdapiv1.NamedAuthInfo{
			{
				Name: konnectivity.CertCommonName,
				AuthInfo: clientcmdapiv1.AuthInfo{
					ClientKeyData:         key,
					ClientCertificateData: cert,
				},
			},
		},
		Clusters: []clientcmdapiv1.NamedCluster{
			{
				Name: "kubernetes",
				Cluster: clientcmdapiv1.Cluster{
					Server:                   "https://localhost:6443",
					CertificateAuthorityData: ca,
				},
			},
		},
		Contexts: []clientcmdapiv1.NamedContext{
			{
				Name: "system:konnectivity-server@kubernetes",
				Context: clientcmdapiv1.Context{
					Cluster:  "kubernetes",
					AuthInfo: konnectivity.CertCommonName,
				},
			},
		},
		CurrentContext: "system:konnectivity-server@kubernetes",
	}

	b, err := utilities.EncodeToYaml(cfg)
	Expect(err).NotTo(HaveOccurred())

	return b
}

func decodeKubeconfig(content []byte) *clientcmdapiv1.Config {
	cfg, err := utilities.DecodeKubeconfigYAML(content)
	Expect(err).NotTo(HaveOccurred())

	return cfg
}
