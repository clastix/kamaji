// Copyright 2025 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/builders/controlplane"
)

var _ = Describe("Deployment Builder", func() {
	var (
		ctx            context.Context
		deploymentBldr controlplane.Deployment
		deployment     appsv1.Deployment
		tcp            kamajiv1alpha1.TenantControlPlane
		datastore      kamajiv1alpha1.DataStore
	)

	BeforeEach(func() {
		ctx = context.Background()

		datastore = kamajiv1alpha1.DataStore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-datastore",
				Namespace: "kamaji-system",
			},
			Spec: kamajiv1alpha1.DataStoreSpec{
				Driver: kamajiv1alpha1.EtcdDriver,
			},
		}

		tcp = kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-tcp",
				Namespace: "default",
			},
			Spec: kamajiv1alpha1.TenantControlPlaneSpec{
				ControlPlane: kamajiv1alpha1.ControlPlane{
					Deployment: kamajiv1alpha1.DeploymentSpec{
						Replicas: pointer.To(int32(1)),
					},
				},
				Kubernetes: kamajiv1alpha1.KubernetesSpec{
					Version: "v1.33.4",
				},
			},
			Status: kamajiv1alpha1.TenantControlPlaneStatus{
				Certificates: kamajiv1alpha1.CertificatesStatus{
					CA: kamajiv1alpha1.CertificatePrivateKeyPairStatus{
						SecretName: "test-ca-secret",
					},
				},
			},
		}

		deployment = appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment",
				Namespace: "default",
			},
		}

		deploymentBldr = controlplane.Deployment{
			Client:    fake.NewClientBuilder().Build(),
			DataStore: datastore,
		}
	})

	Context("InternalCACertificatesConfigMap functionality", func() {
		It("should use cluster CA secret when InternalCACertificatesConfigMap is not specified", func() {
			// Ensure field is not set
			Expect(tcp.Spec.InternalCACertificatesConfigMap).To(BeNil())

			deploymentBldr.Build(ctx, &deployment, tcp)

			// Check etc-ca-certificates volume uses Secret
			var etcCAVolume *corev1.Volume
			for _, vol := range deployment.Spec.Template.Spec.Volumes {
				if vol.Name == "etc-ca-certificates" {
					etcCAVolume = &vol
					break
				}
			}

			Expect(etcCAVolume).NotTo(BeNil())
			Expect(etcCAVolume.VolumeSource.Secret).NotTo(BeNil())
			Expect(etcCAVolume.VolumeSource.Secret.SecretName).To(Equal("test-ca-secret"))
			Expect(etcCAVolume.VolumeSource.ConfigMap).To(BeNil())

			// Check usr-share-ca-certificates volume uses Secret
			var usrShareCAVolume *corev1.Volume
			for _, vol := range deployment.Spec.Template.Spec.Volumes {
				if vol.Name == "usr-share-ca-certificates" {
					usrShareCAVolume = &vol
					break
				}
			}

			Expect(usrShareCAVolume).NotTo(BeNil())
			Expect(usrShareCAVolume.VolumeSource.Secret).NotTo(BeNil())
			Expect(usrShareCAVolume.VolumeSource.Secret.SecretName).To(Equal("test-ca-secret"))
			Expect(usrShareCAVolume.VolumeSource.ConfigMap).To(BeNil())

			// Check usr-local-share-ca-certificates volume uses Secret
			var usrLocalShareCAVolume *corev1.Volume
			for _, vol := range deployment.Spec.Template.Spec.Volumes {
				if vol.Name == "usr-local-share-ca-certificates" {
					usrLocalShareCAVolume = &vol
					break
				}
			}

			Expect(usrLocalShareCAVolume).NotTo(BeNil())
			Expect(usrLocalShareCAVolume.VolumeSource.Secret).NotTo(BeNil())
			Expect(usrLocalShareCAVolume.VolumeSource.Secret.SecretName).To(Equal("test-ca-secret"))
			Expect(usrLocalShareCAVolume.VolumeSource.ConfigMap).To(BeNil())
		})

		It("should use internal CA ConfigMap when InternalCACertificatesConfigMap is specified", func() {
			configMapName := "internal-ca-certs"
			tcp.Spec.InternalCACertificatesConfigMap = &configMapName

			deploymentBldr.Build(ctx, &deployment, tcp)

			// Check etc-ca-certificates volume uses ConfigMap
			var etcCAVolume *corev1.Volume
			for _, vol := range deployment.Spec.Template.Spec.Volumes {
				if vol.Name == "etc-ca-certificates" {
					etcCAVolume = &vol
					break
				}
			}

			Expect(etcCAVolume).NotTo(BeNil())
			Expect(etcCAVolume.VolumeSource.ConfigMap).NotTo(BeNil())
			Expect(etcCAVolume.VolumeSource.ConfigMap.Name).To(Equal(configMapName))
			Expect(etcCAVolume.VolumeSource.Secret).To(BeNil())

			// Check usr-share-ca-certificates volume uses ConfigMap
			var usrShareCAVolume *corev1.Volume
			for _, vol := range deployment.Spec.Template.Spec.Volumes {
				if vol.Name == "usr-share-ca-certificates" {
					usrShareCAVolume = &vol
					break
				}
			}

			Expect(usrShareCAVolume).NotTo(BeNil())
			Expect(usrShareCAVolume.VolumeSource.ConfigMap).NotTo(BeNil())
			Expect(usrShareCAVolume.VolumeSource.ConfigMap.Name).To(Equal(configMapName))
			Expect(usrShareCAVolume.VolumeSource.Secret).To(BeNil())

			// Check usr-local-share-ca-certificates volume uses ConfigMap
			var usrLocalShareCAVolume *corev1.Volume
			for _, vol := range deployment.Spec.Template.Spec.Volumes {
				if vol.Name == "usr-local-share-ca-certificates" {
					usrLocalShareCAVolume = &vol
					break
				}
			}

			Expect(usrLocalShareCAVolume).NotTo(BeNil())
			Expect(usrLocalShareCAVolume.VolumeSource.ConfigMap).NotTo(BeNil())
			Expect(usrLocalShareCAVolume.VolumeSource.ConfigMap.Name).To(Equal(configMapName))
			Expect(usrLocalShareCAVolume.VolumeSource.Secret).To(BeNil())
		})

		It("should set correct DefaultMode for all CA volumes", func() {
			deploymentBldr.Build(ctx, &deployment, tcp)

			// Check all CA-related volumes have correct DefaultMode
			volumeNames := []string{"etc-ca-certificates", "usr-share-ca-certificates", "usr-local-share-ca-certificates"}

			for _, volumeName := range volumeNames {
				var volume *corev1.Volume
				for _, vol := range deployment.Spec.Template.Spec.Volumes {
					if vol.Name == volumeName {
						volume = &vol
						break
					}
				}

				Expect(volume).NotTo(BeNil(), "Volume %s should exist", volumeName)

				if volume.VolumeSource.Secret != nil {
					Expect(volume.VolumeSource.Secret.DefaultMode).NotTo(BeNil())
					Expect(*volume.VolumeSource.Secret.DefaultMode).To(Equal(int32(420)))
				} else if volume.VolumeSource.ConfigMap != nil {
					Expect(volume.VolumeSource.ConfigMap.DefaultMode).NotTo(BeNil())
					Expect(*volume.VolumeSource.ConfigMap.DefaultMode).To(Equal(int32(420)))
				}
			}
		})

		It("should handle empty string ConfigMap name gracefully", func() {
			emptyConfigMapName := ""
			tcp.Spec.InternalCACertificatesConfigMap = &emptyConfigMapName

			deploymentBldr.Build(ctx, &deployment, tcp)

			// When ConfigMap name is empty, it should still create ConfigMap volume source
			var etcCAVolume *corev1.Volume
			for _, vol := range deployment.Spec.Template.Spec.Volumes {
				if vol.Name == "etc-ca-certificates" {
					etcCAVolume = &vol
					break
				}
			}

			Expect(etcCAVolume).NotTo(BeNil())
			Expect(etcCAVolume.VolumeSource.ConfigMap).NotTo(BeNil())
			Expect(etcCAVolume.VolumeSource.ConfigMap.Name).To(Equal(""))
			Expect(etcCAVolume.VolumeSource.Secret).To(BeNil())
		})
	})
})
