// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

type RegistrySettings struct {
	// +kubebuilder:default="registry.k8s.io"
	Registry string `json:"registry,omitempty"`
	// The tag to append to all the Control Plane container images.
	// Optional.
	TagSuffix string `json:"tagSuffix,omitempty"`
	// +kubebuilder:default="kube-apiserver"
	APIServerImage string `json:"apiServerImage,omitempty"`
	// +kubebuilder:default="kube-controller-manager"
	ControllerManagerImage string `json:"controllerManagerImage,omitempty"`
	// +kubebuilder:default="kube-scheduler"
	SchedulerImage string `json:"schedulerImage,omitempty"`
}
