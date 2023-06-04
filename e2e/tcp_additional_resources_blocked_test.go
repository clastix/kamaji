// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane resource with additional resources", func() {
	// TenantControlPlane object with additional resources
	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "validated-additional-resources",
			Namespace: "default",
		},
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Deployment: kamajiv1alpha1.DeploymentSpec{
					Replicas: 1,
					AdditionalInitContainers: []corev1.Container{{
						Name:  initContainerName,
						Image: initContainerImage,
						Command: []string{
							"/bin/sh",
							"-c",
							"echo hello world",
						},
					}},
					AdditionalContainers: []corev1.Container{{
						Name:  additionalContainerName,
						Image: additionalContainerImage,
					}},
					AdditionalVolumes: []corev1.Volume{
						{
							Name: apiServerVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "api-server",
									},
								},
							},
						},
						{
							Name: controllerManagerVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "controller-manager",
									},
								},
							},
						},
						{
							Name: schedulerVolumeName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "scheduler",
									},
								},
							},
						},
					},
					AdditionalVolumeMounts: &kamajiv1alpha1.AdditionalVolumeMounts{
						APIServer: []corev1.VolumeMount{
							{
								Name:      apiServerVolumeName,
								MountPath: "/etc/api-server",
							},
						},
						ControllerManager: []corev1.VolumeMount{
							{
								Name:      controllerManagerVolumeName,
								MountPath: "/etc/controller-manager",
							},
						},
						Scheduler: []corev1.VolumeMount{
							{
								Name:      schedulerVolumeName,
								MountPath: "/etc/scheduler",
							},
						},
					},
				},
				Service: kamajiv1alpha1.ServiceSpec{
					ServiceType: "ClusterIP",
				},
			},
			Kubernetes: kamajiv1alpha1.KubernetesSpec{
				Version: "v1.23.6",
			},
		},
	}

	apiServerConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-server",
			Namespace: tcp.Namespace,
		},
		Data: map[string]string{
			"api-server": "true",
		},
	}

	controllerManagerConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller-manager",
			Namespace: tcp.Namespace,
		},
		Data: map[string]string{
			"controller-manager": "true",
		},
	}

	schedulerConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scheduler",
			Namespace: tcp.Namespace,
		},
		Data: map[string]string{
			"scheduler": "true",
		},
	}
	// Create a TenantControlPlane resource into the cluster
	JustBeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), apiServerConfigMap)).NotTo(HaveOccurred())
		Expect(k8sClient.Create(context.Background(), controllerManagerConfigMap)).NotTo(HaveOccurred())
		Expect(k8sClient.Create(context.Background(), schedulerConfigMap)).NotTo(HaveOccurred())
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
	})
	// Delete the TenantControlPlane resource after test is finished
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
		Expect(k8sClient.Delete(context.Background(), apiServerConfigMap)).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(context.Background(), controllerManagerConfigMap)).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(context.Background(), schedulerConfigMap)).NotTo(HaveOccurred())
	})
	It("should block wrong Deployment configuration", func() {
		// Should be ready
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)

		By("duplicating mount path", func() {
			Consistently(func() error {
				Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, tcp)).NotTo(HaveOccurred())

				lastVolumeMountIndex := len(tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts.APIServer) - 1
				additionalVolumeMount := tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts.APIServer[lastVolumeMountIndex]
				additionalVolumeMount.Name = "duplicated"
				tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts.APIServer = append(tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts.APIServer, additionalVolumeMount)

				return k8sClient.Update(context.Background(), tcp)
			}, 10*time.Second, time.Second).ShouldNot(Succeed())
		})

		By("duplicating container", func() {
			Consistently(func() error {
				Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, tcp)).NotTo(HaveOccurred())

				tcp.Spec.ControlPlane.Deployment.AdditionalContainers = append(tcp.Spec.ControlPlane.Deployment.AdditionalContainers, corev1.Container{
					Name:  "kube-apiserver",
					Image: "mocked",
				})

				return k8sClient.Update(context.Background(), tcp)
			}, 10*time.Second, time.Second).ShouldNot(Succeed())
		})
	})
})
