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
	"k8s.io/client-go/util/retry"
	pointer "k8s.io/utils/ptr"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

var _ = Describe("Deploy a TenantControlPlane resource with security contexts", func() {
	// Contexts are chosen without overlap to allow to verify that context is set on the desired container
	apiServerSecurityContext := &corev1.SecurityContext{
		ReadOnlyRootFilesystem: pointer.To(true),
	}

	controllerManagerSecurityContext := &corev1.SecurityContext{
		AllowPrivilegeEscalation: pointer.To(false),
	}

	schedulerSecurityContext := &corev1.SecurityContext{
		Privileged: pointer.To(false),
	}

	konnectivitySecurityContext := &corev1.SecurityContext{
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}

	podSecurityContext := &corev1.PodSecurityContext{
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}

	// Fill TenantControlPlane object
	tcp := &kamajiv1alpha1.TenantControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tcp-clusterip-security-contexts",
			Namespace: "default",
		},
		Spec: kamajiv1alpha1.TenantControlPlaneSpec{
			ControlPlane: kamajiv1alpha1.ControlPlane{
				Deployment: kamajiv1alpha1.DeploymentSpec{
					Replicas: pointer.To(int32(1)),
					ContainerSecurityContexts: &kamajiv1alpha1.ControlPlaneContainerSecurityContexts{
						APIServer:         apiServerSecurityContext,
						ControllerManager: controllerManagerSecurityContext,
						Scheduler:         schedulerSecurityContext,
					},
					PodSecurityContext: podSecurityContext,
				},
				Service: kamajiv1alpha1.ServiceSpec{
					ServiceType: "ClusterIP",
				},
			},
			Kubernetes: kamajiv1alpha1.KubernetesSpec{
				Version: "v1.36.2",
			},
			Addons: kamajiv1alpha1.AddonsSpec{
				Konnectivity: &kamajiv1alpha1.KonnectivitySpec{
					KonnectivityServerSpec: kamajiv1alpha1.KonnectivityServerSpec{
						Port:            8132,
						SecurityContext: konnectivitySecurityContext,
					},
				},
			},
		},
	}

	// Create a TenantControlPlane resource into the cluster
	JustBeforeEach(func() {
		Expect(k8sClient.Create(context.Background(), tcp)).NotTo(HaveOccurred())
	})

	// Delete the TenantControlPlane resource after test is finished
	JustAfterEach(func() {
		Expect(k8sClient.Delete(context.Background(), tcp)).Should(Succeed())
	})

	// 1. Create TenantControlPlane resource with security contexts set
	// 2. Verify that the security contexts are set correctly
	// 3. Update tenant control plane, unsetting the security contexts
	// 4. Verify that the security contexts are unset correctly
	It("Should be Ready with the expected security contexts", func() {
		StatusMustEqualTo(tcp, kamajiv1alpha1.VersionReady)
		waitUntilControlPlaneReconciliationIsFinished(tcp)

		// Check if individual container security contexts are set correctly
		containerSecurityContextMustEqualTo(tcp, "kube-apiserver", apiServerSecurityContext)
		containerSecurityContextMustEqualTo(tcp, "kube-controller-manager", controllerManagerSecurityContext)
		containerSecurityContextMustEqualTo(tcp, "kube-scheduler", schedulerSecurityContext)
		containerSecurityContextMustEqualTo(tcp, "konnectivity-server", konnectivitySecurityContext)

		// Check if pod security context is set correctly
		podSecurityContextMustEqualTo(tcp, podSecurityContext)

		// This construction prevents "the object has been modified; please apply your changes to the latest version and try again"
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			tcp := &kamajiv1alpha1.TenantControlPlane{}
			err := k8sClient.Get(context.Background(), types.NamespacedName{
				Name:      "tcp-clusterip-security-contexts",
				Namespace: "default",
			}, tcp)
			if err != nil {
				return err
			}

			// Now, set the security contexts to nil
			// We unset the whole ContainerSecurityContexts key
			// This should unset the securityContext for all containers
			tcp.Spec.ControlPlane.Deployment.ContainerSecurityContexts = nil

			tcp.Spec.ControlPlane.Deployment.PodSecurityContext = nil
			tcp.Spec.Addons.Konnectivity.KonnectivityServerSpec.SecurityContext = nil

			return k8sClient.Update(context.Background(), tcp)
		})
		Expect(err).NotTo(HaveOccurred())

		waitUntilControlPlaneReconciliationIsFinished(tcp)

		// Verify that unsetting the keys actually unsets the security context
		containerSecurityContextMustEqualTo(tcp, "kube-apiserver", nil)
		containerSecurityContextMustEqualTo(tcp, "kube-scheduler", nil)
		containerSecurityContextMustEqualTo(tcp, "kube-controller-manager", nil)
		containerSecurityContextMustEqualTo(tcp, "konnectivity-server", nil)
		podSecurityContextMustEqualTo(tcp, &corev1.PodSecurityContext{})
	})
})
