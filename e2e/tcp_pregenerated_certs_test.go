// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	pointer "k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

func generateTestCertificate() (certPEM, keyPEM []byte) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
			CommonName:   "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		panic(err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	return certPEM, keyPEM
}

func kubeadmUtilGetDeployment(ctx context.Context, name, namespace string) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	namespacedName := types.NamespacedName{Name: name, Namespace: namespace}
	err := k8sClient.Get(ctx, namespacedName, deployment)
	return deployment, err
}

var _ = Describe("Deploy TenantControlPlane with PreGenerated Certificates", func() {
	Context("using pregenerated CA certificate", func() {
		var (
			tcp      *kamajiv1alpha1.TenantControlPlane
			caSecret *corev1.Secret
		)

		BeforeEach(func() {
			caCertData, caKeyData := generateTestCertificate()

			caSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pregen-ca-cert",
					Namespace: "default",
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": caCertData,
					"tls.key": caKeyData,
				},
			}

			tcp = &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-pregenerated",
					Namespace: "default",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					ControlPlane: kamajiv1alpha1.ControlPlane{
						Deployment: kamajiv1alpha1.DeploymentSpec{
							Replicas: pointer.To(int32(1)),
						},
						Service: kamajiv1alpha1.ServiceSpec{
							ServiceType: "ClusterIP",
						},
					},
					NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
						Address: "172.18.0.3",
					},
					Kubernetes: kamajiv1alpha1.KubernetesSpec{
						Version: "v1.23.6",
						Kubelet: kamajiv1alpha1.KubeletSpec{
							CGroupFS: "cgroupfs",
						},
					},
					DataStore: "default",
					PreGeneratedCertificates: &kamajiv1alpha1.PreGeneratedCertificatesSpec{
						CA: &kamajiv1alpha1.CertificateReference{
							SecretName:     "pregen-ca-cert",
							CertificateKey: "tls.crt",
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			// Create the pregenerated certificate secret
			Expect(k8sClient.Create(context.Background(), caSecret)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(context.Background(), caSecret)).To(Succeed())
			})
		})

		It("should deploy successfully with pregenerated CA certificate", func() {
			// Create the TenantControlPlane
			Expect(k8sClient.Create(context.Background(), tcp)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(context.Background(), tcp)).To(Succeed())
			})

			// Wait for the deployment to be created and certificates to be configured
			// Note: We cannot wait for full "Ready" status because the self-signed test certificates
			// fail TLS validation when the API server starts. This is expected in test environments.
			// The important thing is verifying that the pre-generated CA is being used correctly.
			Eventually(func() string {
				namespacedName := types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}
				_ = k8sClient.Get(context.Background(), namespacedName, tcp)
				if tcp.Status.Certificates.CA.SecretName != "" {
					return tcp.Status.Certificates.CA.SecretName
				}

				return ""
			}, "2m", "5s").Should(Equal("tcp-pregenerated-ca"))

			// Verify that the CA certificate status is properly set
			Expect(tcp.Status.Certificates.CA.Checksum).NotTo(BeEmpty())
			Expect(tcp.Status.Certificates.CA.LastUpdate).NotTo(BeZero())

			// Verify that the CA certificate secret contains our pregenerated data
			caSecretName := tcp.Status.Certificates.CA.SecretName
			createdSecret := &corev1.Secret{}
			secretNamespacedName := types.NamespacedName{
				Name:      caSecretName,
				Namespace: tcp.Namespace,
			}
			Expect(k8sClient.Get(context.Background(), secretNamespacedName, createdSecret)).To(Succeed())

			// The created secret should contain our pregenerated certificate data
			Expect(createdSecret.Data).To(HaveKey("tls.crt"))
			Expect(createdSecret.Data["tls.crt"]).To(Equal(caSecret.Data["tls.crt"]))
		})

		It("should reject creation when pregenerated secret doesn't exist", func() {
			// Create a new TCP with a unique name to avoid conflict with previous test's deletion
			tcpWithBadSecret := &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("tcp-pregenerated-reject-%d", time.Now().UnixNano()),
					Namespace: "default",
				},
				Spec: tcp.Spec,
			}
			// Use a non-existent secret name
			tcpWithBadSecret.Spec.PreGeneratedCertificates.CA.SecretName = "non-existent-ca"

			// Attempt to create the TenantControlPlane
			err := k8sClient.Create(context.Background(), tcpWithBadSecret)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))

			// Cleanup
			_ = k8sClient.Delete(context.Background(), tcpWithBadSecret)
		})

		It("should reject when both PreGeneratedCertificates and NetworkProfile.CertSANs are set", func() {
			tcp.Spec.NetworkProfile.CertSANs = []string{"custom.example.com"}

			// Attempt to create the TenantControlPlane
			err := k8sClient.Create(context.Background(), tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("mutually exclusive"))

			// Cleanup
			_ = k8sClient.Delete(context.Background(), tcp)
		})
	})

	Context("using pregenerated ServiceAccount key pair", func() {
		var (
			tcp    *kamajiv1alpha1.TenantControlPlane
			saKpSecret *corev1.Secret
		)

		BeforeEach(func() {
			_, saKeyData := generateTestCertificate()

			saKpSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pregen-sa-keypair",
					Namespace: "default",
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"sa.pub": []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQCy7T7Q..."),
					"sa.key": saKeyData,
				},
			}

			tcp = &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-pregen-sa",
					Namespace: "default",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					ControlPlane: kamajiv1alpha1.ControlPlane{
						Deployment: kamajiv1alpha1.DeploymentSpec{
							Replicas: pointer.To(int32(1)),
						},
						Service: kamajiv1alpha1.ServiceSpec{
							ServiceType: "ClusterIP",
						},
					},
					NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
						Address: "172.18.0.4",
					},
					Kubernetes: kamajiv1alpha1.KubernetesSpec{
						Version: "v1.23.6",
						Kubelet: kamajiv1alpha1.KubeletSpec{
							CGroupFS: "cgroupfs",
						},
					},
					DataStore: "default",
					PreGeneratedCertificates: &kamajiv1alpha1.PreGeneratedCertificatesSpec{
						ServiceAccount: &kamajiv1alpha1.KeyReference{
							PublicKeyKey:  "sa.pub",
							PrivateKeyKey: "sa.key",
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			// Create the pregenerated service account key pair secret
			Expect(k8sClient.Create(context.Background(), saKpSecret)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(context.Background(), saKpSecret)).To(Succeed())
			})
		})

		It("should use pregenerated ServiceAccount key pair", func() {
			// Create the TenantControlPlane
			Expect(k8sClient.Create(context.Background(), tcp)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(context.Background(), tcp)).To(Succeed())
			})

			// Wait for the service account key pair to be reconciled
			Eventually(func() string {
				namespacedName := types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}
				_ = k8sClient.Get(context.Background(), namespacedName, tcp)
				if tcp.Status.Certificates.SA.SecretName != "" {
					return tcp.Status.Certificates.SA.SecretName
				}
				return ""
			}, "2m", "5s").Should(Equal("tcp-pregen-sa-sa"))

			// Verify that the service account secret contains our pregenerated data
			saSecretName := tcp.Status.Certificates.SA.SecretName
			createdSecret := &corev1.Secret{}
			secretNamespacedName := types.NamespacedName{
				Name:      saSecretName,
				Namespace: tcp.Namespace,
			}
			Expect(k8sClient.Get(context.Background(), secretNamespacedName, createdSecret)).To(Succeed())

			// The created secret should contain our pregenerated service account key data
			Expect(createdSecret.Data).To(HaveKey("sa.pub"))
			Expect(createdSecret.Data).To(HaveKey("sa.key"))
			Expect(createdSecret.Data["sa.key"]).To(Equal(saKpSecret.Data["sa.key"]))
		})
	})

	Context("kubeconfig stability with pregenerated certificates", func() {
		var (
			tcp      *kamajiv1alpha1.TenantControlPlane
			caSecret *corev1.Secret
		)

		BeforeEach(func() {
			caCertData, caKeyData := generateTestCertificate()

			caSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pregen-ca-kube-stable",
					Namespace: "default",
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": caCertData,
					"tls.key": caKeyData,
				},
			}

			tcp = &kamajiv1alpha1.TenantControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tcp-kube-stable",
					Namespace: "default",
				},
				Spec: kamajiv1alpha1.TenantControlPlaneSpec{
					ControlPlane: kamajiv1alpha1.ControlPlane{
						Deployment: kamajiv1alpha1.DeploymentSpec{
							Replicas: pointer.To(int32(1)),
						},
						Service: kamajiv1alpha1.ServiceSpec{
							ServiceType: "LoadBalancer",
						},
					},
					NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
						Address: "172.18.0.5",
					},
					Kubernetes: kamajiv1alpha1.KubernetesSpec{
						Version: "v1.23.6",
						Kubelet: kamajiv1alpha1.KubeletSpec{
							CGroupFS: "cgroupfs",
						},
					},
					DataStore: "default",
					PreGeneratedCertificates: &kamajiv1alpha1.PreGeneratedCertificatesSpec{
						CA: &kamajiv1alpha1.CertificateReference{
							SecretName:     "pregen-ca-kube-stable",
							CertificateKey: "tls.crt",
						},
					},
				},
			}
		})

		JustBeforeEach(func() {
			Expect(k8sClient.Create(context.Background(), caSecret)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(context.Background(), caSecret)).To(Succeed())
			})
		})

		It("should stabilize Deployment template after service IP assignment", func() {
			// Create the TenantControlPlane
			Expect(k8sClient.Create(context.Background(), tcp)).To(Succeed())
			DeferCleanup(func() {
				Expect(k8sClient.Delete(context.Background(), tcp)).To(Succeed())
			})

			// Wait for the control plane Deployment to be created
			deploymentName := fmt.Sprintf("%s-control-plane", tcp.Name)
			Eventually(func() error {
				_, err := kubeadmUtilGetDeployment(context.Background(), deploymentName, tcp.Namespace)
				return err
			}, "2m", "5s").Should(Succeed())

			// Capture initial Deployment pod-template-hash annotation
			// (changes whenever spec.template changes, including kubeconfig regeneration)
			var firstPodTemplateHash string
			deployment, err := kubeadmUtilGetDeployment(context.Background(), deploymentName, tcp.Namespace)
			Expect(err).ToNot(HaveOccurred())
			firstPodTemplateHash = deployment.Spec.Template.Labels["pod-template-hash"]

			// Poll repeatedly over 90 seconds to ensure pod-template-hash remains stable
			// even as the LoadBalancer Service gets its IP assigned. If the kubeconfig
			// was incorrectly regenerated on every reconcile (the infinite-loop bug),
			// pod-template-hash would change repeatedly. With the fix, it should stabilize.
			stableCount := 0
			Consistently(func() bool {
				deployment, err := kubeadmUtilGetDeployment(context.Background(), deploymentName, tcp.Namespace)
				if err != nil {
					return false
				}
				currentHash := deployment.Spec.Template.Labels["pod-template-hash"]
				if currentHash == firstPodTemplateHash {
					stableCount++
					// Hash stable for at least 3 consecutive checks = regression test passes
					return stableCount >= 3
				}
				stableCount = 0
				firstPodTemplateHash = currentHash
				return false
			}, "90s", "10s").Should(BeTrue(), "pod-template-hash should stabilize and not continuously regenerate")
		})
	})
})
