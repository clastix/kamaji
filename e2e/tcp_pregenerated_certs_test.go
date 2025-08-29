// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	pointer "k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Deploy TenantControlPlane with PreGenerated Certificates", func() {
	Context("using pregenerated CA certificate", func() {
		var (
			tcp      *kamajiv1alpha1.TenantControlPlane
			caSecret *corev1.Secret
		)

		BeforeEach(func() {
			// Create pregenerated CA certificate secret
			caSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pregen-ca-cert",
					Namespace: "default",
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					kubeadmconstants.CACertName: []byte(`-----BEGIN CERTIFICATE-----
MIIDCTCCAfGgAwIBAgIJAKNr9YHPXB3IMA0GCSqGSIb3DQEBCwUAMBIxEDAOBgNV
BAMMB0V0Y2QgQ0EwHhcNMjMwMjEwMTAwMDAwWhcNMzMwMjA3MTAwMDAwWjASMRAw
DgYDVQQDDAdFdGNkIENBMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA
0p1yQMvyBEQ==
-----END CERTIFICATE-----`),
					kubeadmconstants.CAKeyName: []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0p1yQMvyBEQxZwxq8VsKGYBZI9VBh2cJ2UjM3aKJjKkGqHWH
LZhE6H2QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQq
AoGBAPj/k8HHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHHH
-----END RSA PRIVATE KEY-----`),
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
					DataStore: "etcd-bronze",
					PreGeneratedCertificates: &kamajiv1alpha1.PreGeneratedCertificatesSpec{
						CA: &kamajiv1alpha1.CertificateReference{
							SecretName:     "pregen-ca-cert",
							CertificateKey: kubeadmconstants.CACertName,
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

			// Wait for the TenantControlPlane to be ready
			Eventually(func() kamajiv1alpha1.KubernetesVersionStatus {
				namespacedName := types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}
				Expect(k8sClient.Get(context.Background(), namespacedName, tcp)).To(Succeed())
				if tcp.Status.Kubernetes.Version.Status == nil {
					return ""
				}
				return *tcp.Status.Kubernetes.Version.Status
			}, "10m", "5s").Should(Equal(kamajiv1alpha1.VersionReady))

			// Verify that the CA certificate status is properly set
			Expect(tcp.Status.Certificates.CA.SecretName).To(Equal("default-tcp-pregenerated-ca-certificate"))
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
			Expect(createdSecret.Data).To(HaveKey(kubeadmconstants.CACertName))
			Expect(createdSecret.Data[kubeadmconstants.CACertName]).To(Equal(caSecret.Data[kubeadmconstants.CACertName]))
		})

		It("should reject creation when pregenerated secret doesn't exist", func() {
			// Use a non-existent secret name
			tcp.Spec.PreGeneratedCertificates.CA.SecretName = "non-existent-ca"

			// Attempt to create the TenantControlPlane
			err := k8sClient.Create(context.Background(), tcp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})
})
