// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package e2e

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

const (
	namespace                   = "default"
	tcpName                     = "tcp-additional"
	initContainerName           = "init"
	initContainerImage          = "registry.k8s.io/e2e-test-images/busybox:1.29-4"
	additionalContainerName     = "nginx"
	additionalContainerImage    = "registry.k8s.io/e2e-test-images/nginx:1.15-4"
	apiServerVolumeName         = "api-server-volume"
	controllerManagerVolumeName = "controller-manager-volume"
	schedulerVolumeName         = "scheduler-volume"
)

var _ = Describe("Deploy a TenantControlPlane resource with additional options", func() {
	// TenantControlPlane object with additional resources
	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tcpName,
			Namespace: namespace,
		},
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Deployment: kamajiv1alpha1.DeploymentSpec{
					Replicas: pointer.Int32(1),
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
			NetworkProfile: kamajiv1alpha1.NetworkProfileSpec{
				Address: "172.18.0.2",
			},
			Kubernetes: kamajiv1alpha1.KubernetesSpec{
				Version: "v1.23.6",
				Kubelet: kamajiv1alpha1.KubeletSpec{
					CGroupFS: "cgroupfs",
				},
				AdmissionControllers: kamajiv1alpha1.AdmissionControllers{
					"LimitRanger",
					"ResourceQuota",
				},
			},
			Addons: kamajiv1alpha1.AddonsSpec{},
		},
	}
	apiServerConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-server",
			Namespace: namespace,
		},
		Data: map[string]string{
			"api-server": "true",
		},
	}
	controllerManagerConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller-manager",
			Namespace: namespace,
		},
		Data: map[string]string{
			"controller-manager": "true",
		},
	}
	schedulerConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scheduler",
			Namespace: namespace,
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

	It("should have the additional resources", func() {
		// Should be ready
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
		// Should have a TCP deployment
		deploy := appsv1.Deployment{}
		Expect(k8sClient.Get(context.Background(), types.NamespacedName{
			Name:      tcpName,
			Namespace: namespace,
		}, &deploy)).NotTo(HaveOccurred())

		By("checking additional init containers", func() {
			found, _ := utilities.HasNamedContainer(deploy.Spec.Template.Spec.InitContainers, initContainerName)
			Expect(found).To(BeTrue(), "Should have the configured AdditionalInitContainers")
		})

		By("checking additional containers", func() {
			found, _ := utilities.HasNamedContainer(deploy.Spec.Template.Spec.Containers, additionalContainerName)
			Expect(found).To(BeTrue(), "Should have the configured AdditionalContainers")
		})

		By("checking kube-apiserver volumes", func() {
			found, _ := utilities.HasNamedVolume(deploy.Spec.Template.Spec.Volumes, apiServerVolumeName)
			Expect(found).To(BeTrue())

			found, containerIndex := utilities.HasNamedContainer(deploy.Spec.Template.Spec.Containers, "kube-apiserver")
			Expect(found).To(BeTrue())

			found, _ = utilities.HasNamedVolumeMount(deploy.Spec.Template.Spec.Containers[containerIndex].VolumeMounts, apiServerVolumeName)
			Expect(found).To(BeTrue())
		})

		By("checking kube-scheduler volumes", func() {
			found, _ := utilities.HasNamedVolume(deploy.Spec.Template.Spec.Volumes, schedulerVolumeName)
			Expect(found).To(BeTrue())

			found, containerIndex := utilities.HasNamedContainer(deploy.Spec.Template.Spec.Containers, "kube-scheduler")
			Expect(found).To(BeTrue())

			found, _ = utilities.HasNamedVolumeMount(deploy.Spec.Template.Spec.Containers[containerIndex].VolumeMounts, schedulerVolumeName)
			Expect(found).To(BeTrue())
		})

		By("checking kube-controller-manager volumes", func() {
			found, _ := utilities.HasNamedVolume(deploy.Spec.Template.Spec.Volumes, controllerManagerVolumeName)
			Expect(found).To(BeTrue())

			found, containerIndex := utilities.HasNamedContainer(deploy.Spec.Template.Spec.Containers, "kube-controller-manager")
			Expect(found).To(BeTrue())

			found, _ = utilities.HasNamedVolumeMount(deploy.Spec.Template.Spec.Containers[containerIndex].VolumeMounts, controllerManagerVolumeName)
			Expect(found).To(BeTrue())
		})

		By("removing the additional resources", func() {
			var containerName string
			volumeNames, volumeMounts := sets.New[string](), tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts

			Eventually(func() error {
				Expect(k8sClient.Get(context.Background(), types.NamespacedName{Name: tcp.Name, Namespace: tcp.Namespace}, tcp)).NotTo(HaveOccurred())
				containerName = tcp.Spec.ControlPlane.Deployment.AdditionalContainers[0].Name

				for _, volume := range tcp.Spec.ControlPlane.Deployment.AdditionalVolumes {
					volumeNames.Insert(volume.Name)
				}

				tcp.Spec.ControlPlane.Deployment.AdditionalInitContainers = nil
				tcp.Spec.ControlPlane.Deployment.AdditionalContainers = nil
				tcp.Spec.ControlPlane.Deployment.AdditionalVolumeMounts = nil
				tcp.Spec.ControlPlane.Deployment.AdditionalVolumes = nil

				return k8sClient.Update(context.Background(), tcp)
			}, 10*time.Second, time.Second).ShouldNot(HaveOccurred())

			Eventually(func() []corev1.Container {
				deploy := appsv1.Deployment{}

				Expect(k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      tcpName,
					Namespace: namespace,
				}, &deploy)).NotTo(HaveOccurred())

				return deploy.Spec.Template.Spec.InitContainers
			}, 10*time.Second, time.Second).Should(HaveLen(0), "Deployment should not contain anymore the init container")

			Eventually(func() bool {
				deploy := appsv1.Deployment{}

				Expect(k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      tcpName,
					Namespace: namespace,
				}, &deploy)).NotTo(HaveOccurred())

				found, _ := utilities.HasNamedContainer(deploy.Spec.Template.Spec.Containers, containerName)

				return found
			}, 10*time.Second, time.Second).Should(BeFalse(), "Deployment should not contain anymore the additional container")

			Eventually(func() error {
				deploy := appsv1.Deployment{}

				Expect(k8sClient.Get(context.Background(), types.NamespacedName{
					Name:      tcpName,
					Namespace: namespace,
				}, &deploy)).NotTo(HaveOccurred())

				for _, volume := range deploy.Spec.Template.Spec.Volumes {
					if volumeNames.Has(volume.Name) {
						return fmt.Errorf("extra volume with name %s is still present", volume.Name)
					}
				}

				type testCase struct {
					containerName string
					volumeMounts  []corev1.VolumeMount
				}

				for _, tc := range []testCase{
					{
						containerName: "kube-scheduler",
						volumeMounts:  volumeMounts.Scheduler,
					},
					{
						containerName: "kube-apiserver",
						volumeMounts:  volumeMounts.APIServer,
					},
					{
						containerName: "kube-scheduler",
						volumeMounts:  volumeMounts.Scheduler,
					},
				} {
					for _, volumeMount := range tc.volumeMounts {
						found, containerIndex := utilities.HasNamedContainer(deploy.Spec.Template.Spec.Containers, tc.containerName)
						if !found {
							return fmt.Errorf("expected %s, container not found", tc.containerName)
						}

						found, _ = utilities.HasNamedVolumeMount(deploy.Spec.Template.Spec.Containers[containerIndex].VolumeMounts, volumeMount.Name)
						if found {
							return fmt.Errorf("extra volume mount with name %s is still present", volumeMount.Name)
						}
					}
				}

				return nil
			}, 10*time.Second, time.Second).Should(BeNil(), "Deployment should not contain anymore the extra volumes")
		})
	})
})
