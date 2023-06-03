// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

const (
	AgentName      = "konnectivity-agent"
	CertCommonName = "system:konnectivity-server"

	konnectivityEgressSelectorConfigurationPath = "/etc/kubernetes/konnectivity/configurations/egress-selector-configuration.yaml"
	konnectivityServerName                      = "konnectivity-server"
	konnectivityServerPath                      = "/run/konnectivity"

	egressSelectorConfigurationVolume  = "egress-selector-configuration"
	konnectivityUDSVolume              = "konnectivity-uds"
	konnectivityServerKubeconfigVolume = "konnectivity-server-kubeconfig"
)

type Konnectivity struct {
	Scheme runtime.Scheme
}

func (k Konnectivity) buildKonnectivityContainer(addon *kamajiv1alpha1.KonnectivitySpec, replicas int32, podSpec *corev1.PodSpec) {
	found, index := utilities.HasNamedContainer(podSpec.Containers, konnectivityServerName)
	if !found {
		index = len(podSpec.Containers)
		podSpec.Containers = append(podSpec.Containers, corev1.Container{})
	}

	podSpec.Containers[index].Name = konnectivityServerName
	podSpec.Containers[index].Image = fmt.Sprintf("%s:%s", addon.KonnectivityServerSpec.Image, addon.KonnectivityServerSpec.Version)
	podSpec.Containers[index].Command = []string{"/proxy-server"}

	args := utilities.ArgsFromSliceToMap(addon.KonnectivityServerSpec.ExtraArgs)

	args["--uds-name"] = fmt.Sprintf("%s/konnectivity-server.socket", konnectivityServerPath)
	args["--cluster-cert"] = "/etc/kubernetes/pki/apiserver.crt"
	args["--cluster-key"] = "/etc/kubernetes/pki/apiserver.key"
	args["--mode"] = "grpc"
	args["--server-port"] = "0"
	args["--agent-port"] = fmt.Sprintf("%d", addon.KonnectivityServerSpec.Port)
	args["--admin-port"] = "8133"
	args["--health-port"] = "8134"
	args["--agent-namespace"] = "kube-system"
	args["--agent-service-account"] = AgentName
	args["--kubeconfig"] = "/etc/kubernetes/konnectivity-server.conf"
	args["--authentication-audience"] = CertCommonName
	args["--server-count"] = fmt.Sprintf("%d", replicas)

	podSpec.Containers[index].Args = utilities.ArgsFromMapToSlice(args)
	podSpec.Containers[index].LivenessProbe = &corev1.Probe{
		InitialDelaySeconds: 30,
		TimeoutSeconds:      60,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt(8134),
				Scheme: corev1.URISchemeHTTP,
			},
		},
	}
	podSpec.Containers[index].Ports = []corev1.ContainerPort{
		{
			Name:          "agentport",
			ContainerPort: addon.KonnectivityServerSpec.Port,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          "adminport",
			ContainerPort: 8133,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          "healthport",
			ContainerPort: 8134,
			Protocol:      corev1.ProtocolTCP,
		},
	}
	podSpec.Containers[index].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "etc-kubernetes-pki",
			MountPath: "/etc/kubernetes/pki",
			ReadOnly:  true,
		},
		{
			Name:      "konnectivity-server-kubeconfig",
			MountPath: "/etc/kubernetes/konnectivity-server.conf",
			SubPath:   "konnectivity-server.conf",
			ReadOnly:  true,
		},
		{
			Name:      konnectivityUDSVolume,
			MountPath: konnectivityServerPath,
			ReadOnly:  false,
		},
	}
	podSpec.Containers[index].ImagePullPolicy = corev1.PullAlways
	podSpec.Containers[index].Resources = corev1.ResourceRequirements{
		Limits:   nil,
		Requests: nil,
	}

	if resources := addon.KonnectivityServerSpec.Resources; resources != nil {
		podSpec.Containers[index].Resources.Limits = resources.Limits
		podSpec.Containers[index].Resources.Requests = resources.Requests
	}
}

func (k Konnectivity) RemovingVolumeMounts(podSpec *corev1.PodSpec) {
	found, index := utilities.HasNamedContainer(podSpec.Containers, apiServerContainerName)
	if !found {
		return
	}

	for _, volumeMountName := range []string{konnectivityUDSVolume, egressSelectorConfigurationVolume, konnectivityServerKubeconfigVolume} {
		if ok, i := utilities.HasNamedVolumeMount(podSpec.Containers[index].VolumeMounts, volumeMountName); ok {
			var volumesMounts []corev1.VolumeMount

			volumesMounts = append(volumesMounts, podSpec.Containers[index].VolumeMounts[:i]...)
			volumesMounts = append(volumesMounts, podSpec.Containers[index].VolumeMounts[i+1:]...)

			podSpec.Containers[index].VolumeMounts = volumesMounts
		}
	}
}

func (k Konnectivity) RemovingVolumes(podSpec *corev1.PodSpec) {
	for _, volumeName := range []string{konnectivityUDSVolume, egressSelectorConfigurationVolume} {
		if volumeFound, volumeIndex := utilities.HasNamedVolume(podSpec.Volumes, volumeName); volumeFound {
			var volumes []corev1.Volume

			volumes = append(volumes, podSpec.Volumes[:volumeIndex]...)
			volumes = append(volumes, podSpec.Volumes[volumeIndex+1:]...)

			podSpec.Volumes = volumes
		}
	}
}

func (k Konnectivity) RemovingKubeAPIServerContainerArg(podSpec *corev1.PodSpec) {
	if found, index := utilities.HasNamedContainer(podSpec.Containers, apiServerContainerName); found {
		argsMap := utilities.ArgsFromSliceToMap(podSpec.Containers[index].Args)

		if utilities.ArgsRemoveFlag(argsMap, "--egress-selector-config-file") {
			podSpec.Containers[index].Args = utilities.ArgsFromMapToSlice(argsMap)
		}
	}
}

func (k Konnectivity) RemovingContainer(podSpec *corev1.PodSpec) {
	if found, index := utilities.HasNamedContainer(podSpec.Containers, konnectivityServerName); found {
		var containers []corev1.Container

		containers = append(containers, podSpec.Containers[:index]...)
		containers = append(containers, podSpec.Containers[index+1:]...)

		podSpec.Containers = containers
	}
}

func (k Konnectivity) buildVolumeMounts(podSpec *corev1.PodSpec) {
	found, index := utilities.HasNamedContainer(podSpec.Containers, apiServerContainerName)
	if !found {
		return
	}
	// Adding the egress selector config file flag
	args := utilities.ArgsFromSliceToMap(podSpec.Containers[index].Args)

	utilities.ArgsAddFlagValue(args, "--egress-selector-config-file", konnectivityEgressSelectorConfigurationPath)

	podSpec.Containers[index].Args = utilities.ArgsFromMapToSlice(args)

	vFound, vIndex := false, 0
	// Patching the volume mounts
	if vFound, vIndex = utilities.HasNamedVolumeMount(podSpec.Containers[index].VolumeMounts, konnectivityUDSVolume); !vFound {
		vIndex = len(podSpec.Containers[index].VolumeMounts)
		podSpec.Containers[index].VolumeMounts = append(podSpec.Containers[index].VolumeMounts, corev1.VolumeMount{})
	}

	podSpec.Containers[index].VolumeMounts[vIndex].Name = konnectivityUDSVolume
	podSpec.Containers[index].VolumeMounts[vIndex].ReadOnly = false
	podSpec.Containers[index].VolumeMounts[vIndex].MountPath = konnectivityServerPath

	if vFound, vIndex = utilities.HasNamedVolumeMount(podSpec.Containers[index].VolumeMounts, egressSelectorConfigurationVolume); !vFound {
		vIndex = len(podSpec.Containers[index].VolumeMounts)
		podSpec.Containers[index].VolumeMounts = append(podSpec.Containers[index].VolumeMounts, corev1.VolumeMount{})
	}

	podSpec.Containers[index].VolumeMounts[vIndex].Name = egressSelectorConfigurationVolume
	podSpec.Containers[index].VolumeMounts[vIndex].ReadOnly = false
	podSpec.Containers[index].VolumeMounts[vIndex].MountPath = "/etc/kubernetes/konnectivity/configurations"
}

func (k Konnectivity) buildVolumes(status kamajiv1alpha1.KonnectivityStatus, podSpec *corev1.PodSpec) {
	found, index := false, 0
	// Defining volumes for the UDS socket
	found, index = utilities.HasNamedVolume(podSpec.Volumes, konnectivityUDSVolume)
	if !found {
		index = len(podSpec.Volumes)
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[index].Name = konnectivityUDSVolume
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{
			Medium: "Memory",
		},
	}
	// Defining volumes for the egress selector configuration
	found, index = utilities.HasNamedVolume(podSpec.Volumes, egressSelectorConfigurationVolume)
	if !found {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
		index = len(podSpec.Volumes) - 1
	}

	podSpec.Volumes[index].Name = egressSelectorConfigurationVolume
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		ConfigMap: &corev1.ConfigMapVolumeSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: status.ConfigMap.Name,
			},
			DefaultMode: pointer.Int32(420),
		},
	}
	// Defining volume for the Konnectivity kubeconfig
	found, index = utilities.HasNamedVolume(podSpec.Volumes, konnectivityServerKubeconfigVolume)
	if !found {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
		index = len(podSpec.Volumes) - 1
	}

	podSpec.Volumes[index].Name = konnectivityServerKubeconfigVolume
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName:  status.Kubeconfig.SecretName,
			DefaultMode: pointer.Int32(420),
		},
	}
}

func (k Konnectivity) Build(deployment *appsv1.Deployment, tenantControlPlane kamajiv1alpha1.TenantControlPlane) {
	k.buildKonnectivityContainer(tenantControlPlane.Spec.Addons.Konnectivity, tenantControlPlane.Spec.ControlPlane.Deployment.Replicas, &deployment.Spec.Template.Spec)
	k.buildVolumeMounts(&deployment.Spec.Template.Spec)
	k.buildVolumes(tenantControlPlane.Status.Addons.Konnectivity, &deployment.Spec.Template.Spec)

	k.Scheme.Default(deployment)
}
