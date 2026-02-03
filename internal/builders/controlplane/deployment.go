// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"context"
	"crypto/md5"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta3"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	pointer "k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

// Volume names.
const (
	kubernetesPKIVolumeName               = "etc-kubernetes-pki"
	caCertificatesVolumeName              = "etc-ca-certificates"
	usrShareCACertificatesVolumeName      = "usr-share-ca-certificates"
	usrLocalShareCaCertificateVolumeName  = "usr-local-share-ca-certificates"
	schedulerKubeconfigVolumeName         = "scheduler-kubeconfig"
	controllerManagerKubeconfigVolumeName = "controller-manager-kubeconfig"
	kineUDSVolume                         = "kine-uds"
	kineUDSFolder                         = "/uds"
	kineUDSPath                           = kineUDSFolder + "/kine"
	dataStoreCertsVolumeName              = "kine-config"
	kineVolumeCertName                    = "kine-certs"
)

const (
	apiServerFlagsAnnotation = "kube-apiserver.kamaji.clastix.io/args"
	// Kamaji container names.
	apiServerContainerName    = "kube-apiserver"
	controlPlaneContainerName = "kube-controller-manager"
	schedulerContainerName    = "kube-scheduler"
	kineContainerName         = "kine"
	kineInitContainerName     = "chmod"
)

type DataStoreOverrides struct {
	Resource  string
	DataStore kamajiv1alpha1.DataStore
}

type Deployment struct {
	KineContainerImage string
	DataStore          kamajiv1alpha1.DataStore
	DataStoreOverrides []DataStoreOverrides
	Client             client.Client
}

func (d Deployment) Build(ctx context.Context, deployment *appsv1.Deployment, tenantControlPlane kamajiv1alpha1.TenantControlPlane) {
	address, _, _ := tenantControlPlane.AssignedControlPlaneAddress()

	d.setLabels(deployment, utilities.MergeMaps(utilities.KamajiLabels(tenantControlPlane.GetName(), "deployment"), tenantControlPlane.Spec.ControlPlane.Deployment.AdditionalMetadata.Labels))
	d.setAnnotations(deployment, utilities.MergeMaps(deployment.Annotations, tenantControlPlane.Spec.ControlPlane.Deployment.AdditionalMetadata.Annotations))
	d.setTemplateLabels(&deployment.Spec.Template, utilities.MergeMaps(d.templateLabels(ctx, &tenantControlPlane), tenantControlPlane.Spec.ControlPlane.Deployment.PodAdditionalMetadata.Labels))
	d.setTemplateAnnotations(&deployment.Spec.Template, utilities.MergeMaps(tenantControlPlane.Spec.ControlPlane.Deployment.PodAdditionalMetadata.Annotations, map[string]string{"storage.kamaji.clastix.io/config": tenantControlPlane.Status.Storage.Config.Checksum}))
	d.setNodeSelector(&deployment.Spec.Template.Spec, tenantControlPlane)
	d.setToleration(&deployment.Spec.Template.Spec, tenantControlPlane)
	d.setAffinity(&deployment.Spec.Template.Spec, tenantControlPlane)
	d.setStrategy(&deployment.Spec, tenantControlPlane)
	d.setSelector(&deployment.Spec, tenantControlPlane)
	d.setTopologySpreadConstraints(&deployment.Spec, tenantControlPlane.Spec.ControlPlane.Deployment.TopologySpreadConstraints)
	d.setRuntimeClass(&deployment.Spec.Template.Spec, tenantControlPlane)
	d.setReplicas(&deployment.Spec, tenantControlPlane)
	d.resetKubeAPIServerFlags(deployment, tenantControlPlane)
	d.setInitContainers(&deployment.Spec.Template.Spec, tenantControlPlane)
	d.setAdditionalContainers(&deployment.Spec.Template.Spec, tenantControlPlane)
	d.setContainers(&deployment.Spec.Template.Spec, tenantControlPlane, address)
	d.setAdditionalVolumes(&deployment.Spec.Template.Spec, tenantControlPlane)
	d.setVolumes(&deployment.Spec.Template.Spec, tenantControlPlane)
	d.setServiceAccount(&deployment.Spec.Template.Spec, tenantControlPlane)
	d.Client.Scheme().Default(deployment)
}

func (d Deployment) setContainers(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane, address string) {
	d.buildKubeAPIServer(podSpec, tcp, address)
	d.buildScheduler(podSpec, tcp)
	d.buildControllerManager(podSpec, tcp)
	d.buildKine(podSpec, tcp)
}

// setInitContainers allows adding extra init containers from the user-space:
// this function must be called priorit the setContainers to ensure the idempotency of podSpec building.
func (d Deployment) setInitContainers(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	initContainers := tcp.Spec.ControlPlane.Deployment.AdditionalInitContainers

	if d.DataStore.Spec.Driver == kamajiv1alpha1.EtcdDriver {
		podSpec.InitContainers = initContainers
	}

	found, index := utilities.HasNamedContainer(podSpec.InitContainers, kineInitContainerName)
	if found {
		initContainers = append(initContainers, podSpec.InitContainers[index:]...)
	}

	podSpec.InitContainers = initContainers
}

// setAdditionalContainers must be called before setContainers: the user-space ones are going to be prepended
// to simplify the management of the Kamaji ones during the create or update action.
func (d Deployment) setAdditionalContainers(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	containers := tcp.Spec.ControlPlane.Deployment.AdditionalContainers

	found, index := utilities.HasNamedContainer(podSpec.Containers, apiServerContainerName)
	if found {
		containers = append(containers, podSpec.Containers[index:]...)
	}

	podSpec.Containers = containers
}

func (d Deployment) setStrategy(deployment *appsv1.DeploymentSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	deployment.Strategy = appsv1.DeploymentStrategy{
		Type: tcp.Spec.ControlPlane.Deployment.Strategy.Type,
	}
	// If it's recreate strategy, we don't need any RollingUpdate params
	if tcp.Spec.ControlPlane.Deployment.Strategy.Type == appsv1.RecreateDeploymentStrategyType {
		tcp.Spec.ControlPlane.Deployment.Strategy.RollingUpdate = nil

		return
	}
	// In case of no RollingUpdate options, Kamaji will perform a blue/green rollout:
	// this will ensure to avoid round-robin between old and new version,
	// useful especially during the Kubernetes version upgrade phase.
	if tcp.Spec.ControlPlane.Deployment.Strategy.RollingUpdate == nil {
		maxSurge := intstr.FromString("100%")

		maxUnavailable := intstr.FromInt32(0)

		deployment.Strategy.RollingUpdate = &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &maxUnavailable,
			MaxSurge:       &maxSurge,
		}

		return
	}

	deployment.Strategy.RollingUpdate = &appsv1.RollingUpdateDeployment{}

	if tcp.Spec.ControlPlane.Deployment.Strategy.RollingUpdate.MaxUnavailable != nil {
		deployment.Strategy.RollingUpdate.MaxUnavailable = tcp.Spec.ControlPlane.Deployment.Strategy.RollingUpdate.MaxUnavailable
	}

	if tcp.Spec.ControlPlane.Deployment.Strategy.RollingUpdate.MaxSurge != nil {
		deployment.Strategy.RollingUpdate.MaxSurge = tcp.Spec.ControlPlane.Deployment.Strategy.RollingUpdate.MaxSurge
	}
}

func (d Deployment) setVolumes(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	for _, fn := range []func(*corev1.PodSpec, kamajiv1alpha1.TenantControlPlane){
		d.buildPKIVolume,
		d.buildCAVolume,
		d.buildShareCAVolume,
		d.buildLocalShareCAVolume,
		d.buildSchedulerVolume,
		d.buildControllerManagerVolume,
		d.buildKineVolume,
	} {
		fn(podSpec, tcp)
	}
}

func (d Deployment) buildPKIVolume(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	found, index := utilities.HasNamedVolume(podSpec.Volumes, kubernetesPKIVolumeName)
	if !found {
		index = len(podSpec.Volumes)
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	sources := []corev1.VolumeProjection{
		{
			Secret: d.secretProjection(tcp.Status.Certificates.APIServer.SecretName, constants.APIServerCertName, constants.APIServerKeyName),
		},
		{
			Secret: d.secretProjection(tcp.Status.Certificates.CA.SecretName, constants.CACertName, constants.CAKeyName),
		},
		{
			Secret: d.secretProjection(tcp.Status.Certificates.APIServerKubeletClient.SecretName, constants.APIServerKubeletClientCertName, constants.APIServerKubeletClientKeyName),
		},
		{
			Secret: d.secretProjection(tcp.Status.Certificates.FrontProxyCA.SecretName, constants.FrontProxyCACertName, constants.FrontProxyCAKeyName),
		},
		{
			Secret: d.secretProjection(tcp.Status.Certificates.FrontProxyClient.SecretName, constants.FrontProxyClientCertName, constants.FrontProxyClientKeyName),
		},
		{
			Secret: d.secretProjection(tcp.Status.Certificates.SA.SecretName, constants.ServiceAccountPublicKeyName, constants.ServiceAccountPrivateKeyName),
		},
	}

	if d.DataStore.Spec.Driver == kamajiv1alpha1.EtcdDriver {
		sources = append(sources, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: tcp.Status.Storage.Certificate.SecretName,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  "ca.crt",
						Path: "etcd/ca.crt",
					},
					{
						Key:  "server.crt",
						Path: "etcd/server.crt",
					},
					{
						Key:  "server.key",
						Path: "etcd/server.key",
					},
				},
			},
		})
	}

	podSpec.Volumes[index].Name = kubernetesPKIVolumeName
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		Projected: &corev1.ProjectedVolumeSource{
			Sources:     sources,
			DefaultMode: pointer.To(int32(420)),
		},
	}
}

func (d Deployment) buildCAVolume(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	found, index := utilities.HasNamedVolume(podSpec.Volumes, caCertificatesVolumeName)
	if !found {
		index = len(podSpec.Volumes)
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[index].Name = caCertificatesVolumeName
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName:  tcp.Status.Certificates.CA.SecretName,
			DefaultMode: pointer.To(int32(420)),
		},
	}
}

func (d Deployment) buildShareCAVolume(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	found, index := utilities.HasNamedVolume(podSpec.Volumes, usrShareCACertificatesVolumeName)
	if !found {
		index = len(podSpec.Volumes)
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[index].Name = usrShareCACertificatesVolumeName
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName:  tcp.Status.Certificates.CA.SecretName,
			DefaultMode: pointer.To(int32(420)),
		},
	}
}

func (d Deployment) buildLocalShareCAVolume(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	found, index := utilities.HasNamedVolume(podSpec.Volumes, usrLocalShareCaCertificateVolumeName)
	if !found {
		index = len(podSpec.Volumes)
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[index].Name = usrLocalShareCaCertificateVolumeName
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName:  tcp.Status.Certificates.CA.SecretName,
			DefaultMode: pointer.To(int32(420)),
		},
	}
}

func (d Deployment) buildSchedulerVolume(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	found, index := utilities.HasNamedVolume(podSpec.Volumes, schedulerKubeconfigVolumeName)
	if !found {
		index = len(podSpec.Volumes)
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[index].Name = schedulerKubeconfigVolumeName
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName:  tcp.Status.KubeConfig.Scheduler.SecretName,
			DefaultMode: pointer.To(int32(420)),
		},
	}
}

func (d Deployment) buildControllerManagerVolume(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	found, index := utilities.HasNamedVolume(podSpec.Volumes, controllerManagerKubeconfigVolumeName)
	if !found {
		index = len(podSpec.Volumes)
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[index].Name = controllerManagerKubeconfigVolumeName
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName:  tcp.Status.KubeConfig.ControllerManager.SecretName,
			DefaultMode: pointer.To(int32(420)),
		},
	}
}

// setAdditionalVolumes must be called before setVolumes: the user-space ones are going to be prepended
// to simplify the management of the Kamaji ones during the create or update action.
func (d Deployment) setAdditionalVolumes(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	volumes := tcp.Spec.ControlPlane.Deployment.AdditionalVolumes

	found, index := utilities.HasNamedVolume(podSpec.Volumes, kubernetesPKIVolumeName)
	if found {
		volumes = append(volumes, podSpec.Volumes[index:]...)
	}

	podSpec.Volumes = volumes
}

func (d Deployment) buildScheduler(podSpec *corev1.PodSpec, tenantControlPlane kamajiv1alpha1.TenantControlPlane) {
	found, index := utilities.HasNamedContainer(podSpec.Containers, schedulerContainerName)
	if !found {
		index = len(podSpec.Containers)
		podSpec.Containers = append(podSpec.Containers, corev1.Container{})
	}

	args := map[string]string{}

	if tenantControlPlane.Spec.ControlPlane.Deployment.ExtraArgs != nil {
		args = utilities.ArgsFromSliceToMap(tenantControlPlane.Spec.ControlPlane.Deployment.ExtraArgs.Scheduler)
	}

	kubeconfig := "/etc/kubernetes/scheduler.conf"

	args["--authentication-kubeconfig"] = kubeconfig
	args["--authorization-kubeconfig"] = kubeconfig
	args["--bind-address"] = "0.0.0.0"
	args["--kubeconfig"] = kubeconfig
	args["--leader-elect"] = "true"

	podSpec.Containers[index].Name = schedulerContainerName
	podSpec.Containers[index].Image = tenantControlPlane.Spec.ControlPlane.Deployment.RegistrySettings.KubeSchedulerImage(tenantControlPlane.Spec.Kubernetes.Version)
	podSpec.Containers[index].Command = []string{"kube-scheduler"}
	podSpec.Containers[index].Args = utilities.ArgsFromMapToSlice(args)
	podSpec.Containers[index].LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt32(10259),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[index].StartupProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt32(10259),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}

	switch {
	case tenantControlPlane.Spec.ControlPlane.Deployment.Resources == nil:
		podSpec.Containers[index].Resources = corev1.ResourceRequirements{}
	case tenantControlPlane.Spec.ControlPlane.Deployment.Resources.Scheduler != nil:
		podSpec.Containers[index].Resources = *tenantControlPlane.Spec.ControlPlane.Deployment.Resources.Scheduler
	default:
		podSpec.Containers[index].Resources = corev1.ResourceRequirements{}
	}
	// Volume mounts
	var extraVolumeMounts []corev1.VolumeMount

	if additionalVolumeMounts := tenantControlPlane.Spec.ControlPlane.Deployment.AdditionalVolumeMounts; additionalVolumeMounts != nil {
		extraVolumeMounts = append(extraVolumeMounts, additionalVolumeMounts.Scheduler...)
	}

	volumeMounts := d.initVolumeMounts(schedulerKubeconfigVolumeName, podSpec.Containers[index].VolumeMounts, extraVolumeMounts...)

	d.ensureVolumeMount(&volumeMounts, corev1.VolumeMount{
		Name:      schedulerKubeconfigVolumeName,
		ReadOnly:  true,
		MountPath: "/etc/kubernetes",
	})

	podSpec.Containers[index].VolumeMounts = volumeMounts
}

func (d Deployment) buildControllerManager(podSpec *corev1.PodSpec, tenantControlPlane kamajiv1alpha1.TenantControlPlane) {
	found, index := utilities.HasNamedContainer(podSpec.Containers, controlPlaneContainerName)
	if !found {
		index = len(podSpec.Containers)
		podSpec.Containers = append(podSpec.Containers, corev1.Container{})
	}

	kubeconfig := "/etc/kubernetes/controller-manager.conf"

	args := map[string]string{
		"--allocate-node-cidrs":              "true",
		"--authentication-kubeconfig":        kubeconfig,
		"--authorization-kubeconfig":         kubeconfig,
		"--bind-address":                     "0.0.0.0",
		"--client-ca-file":                   path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName),
		"--cluster-name":                     tenantControlPlane.GetName(),
		"--cluster-signing-cert-file":        path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName),
		"--cluster-signing-key-file":         path.Join(v1beta3.DefaultCertificatesDir, constants.CAKeyName),
		"--controllers":                      "*,bootstrapsigner,tokencleaner",
		"--kubeconfig":                       kubeconfig,
		"--leader-elect":                     "true",
		"--service-cluster-ip-range":         tenantControlPlane.Spec.NetworkProfile.ServiceCIDR,
		"--cluster-cidr":                     tenantControlPlane.Spec.NetworkProfile.PodCIDR,
		"--requestheader-client-ca-file":     path.Join(v1beta3.DefaultCertificatesDir, constants.FrontProxyCACertName),
		"--root-ca-file":                     path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName),
		"--service-account-private-key-file": path.Join(v1beta3.DefaultCertificatesDir, constants.ServiceAccountPrivateKeyName),
		"--use-service-account-credentials":  "true",
	}

	if extraArgs := tenantControlPlane.Spec.ControlPlane.Deployment.ExtraArgs; extraArgs != nil && len(extraArgs.ControllerManager) > 0 {
		args = utilities.MergeMaps(args, utilities.ArgsFromSliceToMap(extraArgs.ControllerManager))
	}

	podSpec.Containers[index].Name = "kube-controller-manager"
	podSpec.Containers[index].Image = tenantControlPlane.Spec.ControlPlane.Deployment.RegistrySettings.KubeControllerManagerImage(tenantControlPlane.Spec.Kubernetes.Version)
	podSpec.Containers[index].Command = []string{"kube-controller-manager"}
	podSpec.Containers[index].Args = utilities.ArgsFromMapToSlice(args)
	podSpec.Containers[index].LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt32(10257),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[index].StartupProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt32(10257),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	switch {
	case tenantControlPlane.Spec.ControlPlane.Deployment.Resources == nil:
		podSpec.Containers[index].Resources = corev1.ResourceRequirements{}
	case tenantControlPlane.Spec.ControlPlane.Deployment.Resources.ControllerManager != nil:
		podSpec.Containers[index].Resources = *tenantControlPlane.Spec.ControlPlane.Deployment.Resources.ControllerManager
	default:
		podSpec.Containers[index].Resources = corev1.ResourceRequirements{}
	}
	// Volume mounts
	var extraVolumeMounts []corev1.VolumeMount

	if additionalVolumeMounts := tenantControlPlane.Spec.ControlPlane.Deployment.AdditionalVolumeMounts; additionalVolumeMounts != nil {
		extraVolumeMounts = append(extraVolumeMounts, additionalVolumeMounts.ControllerManager...)
	}

	volumeMounts := d.initVolumeMounts(controllerManagerKubeconfigVolumeName, podSpec.Containers[index].VolumeMounts, extraVolumeMounts...)

	d.ensureVolumeMount(&volumeMounts, corev1.VolumeMount{
		Name:      controllerManagerKubeconfigVolumeName,
		ReadOnly:  true,
		MountPath: "/etc/kubernetes",
	})
	d.ensureVolumeMount(&volumeMounts, corev1.VolumeMount{
		Name:      kubernetesPKIVolumeName,
		ReadOnly:  true,
		MountPath: v1beta3.DefaultCertificatesDir,
	})
	d.ensureVolumeMount(&volumeMounts, corev1.VolumeMount{
		Name:      caCertificatesVolumeName,
		ReadOnly:  true,
		MountPath: "/etc/ca-certificates",
	})
	d.ensureVolumeMount(&volumeMounts, corev1.VolumeMount{
		Name:      usrShareCACertificatesVolumeName,
		ReadOnly:  true,
		MountPath: "/usr/share/ca-certificates",
	})
	d.ensureVolumeMount(&volumeMounts, corev1.VolumeMount{
		Name:      usrLocalShareCaCertificateVolumeName,
		ReadOnly:  true,
		MountPath: "/usr/local/share/ca-certificates",
	})

	podSpec.Containers[index].VolumeMounts = volumeMounts
}

// ensureVolumeMount retrieve the index for the named volumeMount, in case of missing it's going to be appended.
func (d Deployment) ensureVolumeMount(in *[]corev1.VolumeMount, desired corev1.VolumeMount) {
	list := *in

	found, index := utilities.HasNamedVolumeMount(*in, desired.Name)
	if !found {
		index = len(list)
		list = append(list, corev1.VolumeMount{})
	}

	list[index] = desired

	*in = list
}

// initVolumeMounts is responsible to create the idempotent slice of corev1.VolumeMount:
// firstSystemVolumeMountName must refer to the first Kamaji-space volume mount to detect properly user-space ones.
func (d Deployment) initVolumeMounts(firstSystemVolumeMountName string, actual []corev1.VolumeMount, extra ...corev1.VolumeMount) []corev1.VolumeMount {
	var volumeMounts []corev1.VolumeMount

	volumeMounts = append(volumeMounts, extra...)
	// Retrieve the first safe volume mount to pick up from:
	// this is required to be sure to delete all the extra containers from the user space.
	if vmFound, vmIndex := utilities.HasNamedVolumeMount(actual, firstSystemVolumeMountName); vmFound {
		volumeMounts = append(volumeMounts, actual[vmIndex:]...)
	}

	return volumeMounts
}

func (d Deployment) buildKubeAPIServer(podSpec *corev1.PodSpec, tenantControlPlane kamajiv1alpha1.TenantControlPlane, address string) {
	found, index := utilities.HasNamedContainer(podSpec.Containers, apiServerContainerName)
	if !found {
		index = len(podSpec.Containers)
		podSpec.Containers = append(podSpec.Containers, corev1.Container{})
	}

	args := d.buildKubeAPIServerCommand(tenantControlPlane, address, utilities.ArgsFromSliceToMap(podSpec.Containers[index].Args))

	podSpec.Containers[index].Name = apiServerContainerName
	podSpec.Containers[index].Args = utilities.ArgsFromMapToSlice(args)
	podSpec.Containers[index].Image = tenantControlPlane.Spec.ControlPlane.Deployment.RegistrySettings.KubeAPIServerImage(tenantControlPlane.Spec.Kubernetes.Version)
	podSpec.Containers[index].Command = []string{"kube-apiserver"}
	podSpec.Containers[index].LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/livez",
				Port:   intstr.FromInt32(tenantControlPlane.Spec.NetworkProfile.Port),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[index].ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/readyz",
				Port:   intstr.FromInt32(tenantControlPlane.Spec.NetworkProfile.Port),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[index].StartupProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/livez",
				Port:   intstr.FromInt32(tenantControlPlane.Spec.NetworkProfile.Port),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[index].ImagePullPolicy = corev1.PullAlways
	// Volume mounts
	var extraVolumeMounts []corev1.VolumeMount

	if additionalVolumeMounts := tenantControlPlane.Spec.ControlPlane.Deployment.AdditionalVolumeMounts; additionalVolumeMounts != nil {
		extraVolumeMounts = append(extraVolumeMounts, additionalVolumeMounts.APIServer...)
	}

	volumeMounts := d.initVolumeMounts(kubernetesPKIVolumeName, podSpec.Containers[index].VolumeMounts, extraVolumeMounts...)

	if d.DataStore.Spec.Driver == kamajiv1alpha1.KineMySQLDriver ||
		d.DataStore.Spec.Driver == kamajiv1alpha1.KinePostgreSQLDriver ||
		d.DataStore.Spec.Driver == kamajiv1alpha1.KineNatsDriver {
		d.ensureVolumeMount(&volumeMounts, corev1.VolumeMount{
			Name:      kineUDSVolume,
			ReadOnly:  false,
			MountPath: kineUDSFolder,
		})
	}

	d.ensureVolumeMount(&volumeMounts, corev1.VolumeMount{
		Name:      kubernetesPKIVolumeName,
		ReadOnly:  true,
		MountPath: v1beta3.DefaultCertificatesDir,
	})
	d.ensureVolumeMount(&volumeMounts, corev1.VolumeMount{
		Name:      caCertificatesVolumeName,
		ReadOnly:  true,
		MountPath: "/etc/ca-certificates",
	})
	d.ensureVolumeMount(&volumeMounts, corev1.VolumeMount{
		Name:      usrShareCACertificatesVolumeName,
		ReadOnly:  true,
		MountPath: "/usr/share/ca-certificates",
	})
	d.ensureVolumeMount(&volumeMounts, corev1.VolumeMount{
		Name:      usrLocalShareCaCertificateVolumeName,
		ReadOnly:  true,
		MountPath: "/usr/local/share/ca-certificates",
	})

	podSpec.Containers[index].VolumeMounts = volumeMounts

	switch {
	case tenantControlPlane.Spec.ControlPlane.Deployment.Resources == nil:
		podSpec.Containers[index].Resources = corev1.ResourceRequirements{}
	case tenantControlPlane.Spec.ControlPlane.Deployment.Resources.APIServer != nil:
		podSpec.Containers[index].Resources = *tenantControlPlane.Spec.ControlPlane.Deployment.Resources.APIServer
	default:
		podSpec.Containers[index].Resources = corev1.ResourceRequirements{}
	}
}

func (d Deployment) buildKubeAPIServerCommand(tenantControlPlane kamajiv1alpha1.TenantControlPlane, address string, current map[string]string) map[string]string {
	var extraArgs map[string]string

	if tenantControlPlane.Spec.ControlPlane.Deployment.ExtraArgs != nil {
		extraArgs = utilities.ArgsFromSliceToMap(tenantControlPlane.Spec.ControlPlane.Deployment.ExtraArgs.APIServer)
	}

	kubeletPreferredAddressTypes := make([]string, 0, len(tenantControlPlane.Spec.Kubernetes.Kubelet.PreferredAddressTypes))

	for _, addressType := range tenantControlPlane.Spec.Kubernetes.Kubelet.PreferredAddressTypes {
		kubeletPreferredAddressTypes = append(kubeletPreferredAddressTypes, string(addressType))
	}

	desiredArgs := map[string]string{
		"--allow-privileged":                   "true",
		"--authorization-mode":                 "Node,RBAC",
		"--advertise-address":                  address,
		"--client-ca-file":                     path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName),
		"--enable-admission-plugins":           strings.Join(tenantControlPlane.Spec.Kubernetes.AdmissionControllers.ToSlice(), ","),
		"--enable-bootstrap-token-auth":        "true",
		"--service-cluster-ip-range":           tenantControlPlane.Spec.NetworkProfile.ServiceCIDR,
		"--kubelet-client-certificate":         path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerKubeletClientCertName),
		"--kubelet-client-key":                 path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerKubeletClientKeyName),
		"--kubelet-preferred-address-types":    strings.Join(kubeletPreferredAddressTypes, ","),
		"--proxy-client-cert-file":             path.Join(v1beta3.DefaultCertificatesDir, constants.FrontProxyClientCertName),
		"--proxy-client-key-file":              path.Join(v1beta3.DefaultCertificatesDir, constants.FrontProxyClientKeyName),
		"--requestheader-allowed-names":        constants.FrontProxyClientCertCommonName,
		"--requestheader-client-ca-file":       path.Join(v1beta3.DefaultCertificatesDir, constants.FrontProxyCACertName),
		"--requestheader-extra-headers-prefix": "X-Remote-Extra-",
		"--requestheader-group-headers":        "X-Remote-Group",
		"--requestheader-username-headers":     "X-Remote-User",
		"--secure-port":                        fmt.Sprintf("%d", tenantControlPlane.Spec.NetworkProfile.Port),
		"--service-account-issuer":             "https://kubernetes.default.svc.cluster.local",
		"--service-account-key-file":           path.Join(v1beta3.DefaultCertificatesDir, constants.ServiceAccountPublicKeyName),
		"--service-account-signing-key-file":   path.Join(v1beta3.DefaultCertificatesDir, constants.ServiceAccountPrivateKeyName),
		"--tls-cert-file":                      path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerCertName),
		"--tls-private-key-file":               path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerKeyName),
	}

	switch d.DataStore.Spec.Driver {
	case kamajiv1alpha1.KineMySQLDriver, kamajiv1alpha1.KinePostgreSQLDriver, kamajiv1alpha1.KineNatsDriver:
		desiredArgs["--etcd-servers"] = "unix://" + kineUDSPath
	case kamajiv1alpha1.EtcdDriver:
		httpsEndpoints := make([]string, 0, len(d.DataStore.Spec.Endpoints))

		for _, ep := range d.DataStore.Spec.Endpoints {
			httpsEndpoints = append(httpsEndpoints, fmt.Sprintf("https://%s", ep))
		}

		desiredArgs["--etcd-compaction-interval"] = "0"
		desiredArgs["--etcd-prefix"] = fmt.Sprintf("/%s", tenantControlPlane.Status.Storage.Setup.Schema)
		desiredArgs["--etcd-servers"] = strings.Join(httpsEndpoints, ",")
		desiredArgs["--etcd-cafile"] = "/etc/kubernetes/pki/etcd/ca.crt"
		desiredArgs["--etcd-certfile"] = "/etc/kubernetes/pki/etcd/server.crt"
		desiredArgs["--etcd-keyfile"] = "/etc/kubernetes/pki/etcd/server.key"
	}

	if len(d.DataStoreOverrides) != 0 {
		desiredArgs["--etcd-servers-overrides"] = d.etcdServersOverrides()
	}

	// Order matters, here: extraArgs could try to overwrite some arguments managed by Kamaji and that would be crucial.
	// Adding as first element of the array of maps, we're sure that these overrides will be sanitized by our configuration.
	return utilities.MergeMaps(current, desiredArgs, extraArgs)
}

func (d Deployment) etcdServersOverrides() string {
	dataStoreOverridesEndpoints := make([]string, 0, len(d.DataStoreOverrides))
	for _, dso := range d.DataStoreOverrides {
		httpsEndpoints := make([]string, 0, len(dso.DataStore.Spec.Endpoints))

		for _, ep := range dso.DataStore.Spec.Endpoints {
			httpsEndpoints = append(httpsEndpoints, fmt.Sprintf("https://%s", ep))
		}
		dataStoreOverridesEndpoints = append(dataStoreOverridesEndpoints, fmt.Sprintf("%s#%s", dso.Resource, strings.Join(httpsEndpoints, ";")))
	}

	return strings.Join(dataStoreOverridesEndpoints, ",")
}

func (d Deployment) secretProjection(secretName, certKeyName, keyName string) *corev1.SecretProjection {
	return &corev1.SecretProjection{
		LocalObjectReference: corev1.LocalObjectReference{
			Name: secretName,
		},
		Items: []corev1.KeyToPath{
			{
				Key:  certKeyName,
				Path: certKeyName,
			},
			{
				Key:  keyName,
				Path: keyName,
			},
		},
	}
}

func (d Deployment) removeKineVolumes(podSpec *corev1.PodSpec) {
	for _, volumeName := range []string{kineVolumeCertName, dataStoreCertsVolumeName, kineUDSVolume} {
		if found, index := utilities.HasNamedVolume(podSpec.Volumes, volumeName); found {
			var volumes []corev1.Volume

			volumes = append(volumes, podSpec.Volumes[:index]...)
			volumes = append(volumes, podSpec.Volumes[index+1:]...)

			podSpec.Volumes = volumes
		}
	}
}

func (d Deployment) buildKineVolume(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	if d.DataStore.Spec.Driver == kamajiv1alpha1.EtcdDriver {
		return
	}

	found, index := utilities.HasNamedVolume(podSpec.Volumes, kineUDSVolume)
	if !found {
		index = len(podSpec.Volumes)
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[index].Name = kineUDSVolume
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{
			Medium: "Memory",
		},
	}

	found, index = utilities.HasNamedVolume(podSpec.Volumes, dataStoreCertsVolumeName)
	if !found {
		index = len(podSpec.Volumes)
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[index].Name = dataStoreCertsVolumeName
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName:  tcp.Status.Storage.Certificate.SecretName,
			DefaultMode: pointer.To(int32(420)),
		},
	}
	// Adding the volume to read Kine certificates:
	// these must be subsequently fixed with a chmod due to pg issues with private key.
	found, index = utilities.HasNamedVolume(podSpec.Volumes, kineVolumeCertName)
	if !found {
		index = len(podSpec.Volumes)
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[index].Name = kineVolumeCertName
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}
}

func (d Deployment) removeKineContainers(podSpec *corev1.PodSpec) {
	// Removing the kine container, if present
	if found, index := utilities.HasNamedContainer(podSpec.Containers, kineContainerName); found {
		var containers []corev1.Container

		containers = append(containers, podSpec.Containers[:index]...)
		containers = append(containers, podSpec.Containers[index+1:]...)

		podSpec.Containers = containers
	}
	d.removeKineInitContainers(podSpec)
}

func (d Deployment) removeKineInitContainers(podSpec *corev1.PodSpec) {
	if found, index := utilities.HasNamedContainer(podSpec.InitContainers, kineInitContainerName); found {
		var initContainers []corev1.Container

		initContainers = append(initContainers, podSpec.InitContainers[:index]...)
		initContainers = append(initContainers, podSpec.InitContainers[index+1:]...)

		podSpec.InitContainers = initContainers
	}
}

func (d Deployment) buildKine(podSpec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	if d.DataStore.Spec.Driver == kamajiv1alpha1.EtcdDriver {
		d.removeKineContainers(podSpec)
		d.removeKineVolumes(podSpec)

		return
	}

	// Building kine arguments, taking in consideration the user-space ones if provided.
	args := map[string]string{}

	args["--listen-address"] = "unix://" + kineUDSPath

	if d.DataStore.Spec.TLSConfig != nil {
		// Ensuring the init container required for kine is present:
		// a chmod is required for kine in order to read the certificates to connect to the secured datastore.
		found, index := utilities.HasNamedContainer(podSpec.InitContainers, kineInitContainerName)
		if !found {
			index = len(podSpec.InitContainers)
			podSpec.InitContainers = append(podSpec.InitContainers, corev1.Container{})
		}

		podSpec.InitContainers[index].Name = kineInitContainerName
		podSpec.InitContainers[index].Image = d.KineContainerImage
		podSpec.InitContainers[index].Command = []string{"sh"}

		podSpec.InitContainers[index].Args = []string{
			"-c",
			"cp /kine/*.* /certs && chmod -R 600 /certs/*.*",
		}

		podSpec.InitContainers[index].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      dataStoreCertsVolumeName,
				ReadOnly:  true,
				MountPath: "/kine",
			},
			{
				Name:      kineVolumeCertName,
				MountPath: "/certs",
				ReadOnly:  false,
			},
		}

		args["--ca-file"] = "/certs/ca.crt"

		if d.DataStore.Spec.TLSConfig.ClientCertificate != nil {
			args["--cert-file"] = "/certs/server.crt"
			args["--key-file"] = "/certs/server.key"
		}
	} else {
		// if no TLS configuration is provided, the kine initContainer must be removed.
		d.removeKineInitContainers(podSpec)
	}

	// Kine is expecting an additional container, and it must be removed before proceeding with the additional one
	// in order to make this function idempotent.
	found, index := utilities.HasNamedContainer(podSpec.Containers, kineContainerName)
	if !found {
		index = len(podSpec.Containers)
		podSpec.Containers = append(podSpec.Containers, corev1.Container{})
	}

	if tcp.Spec.ControlPlane.Deployment.ExtraArgs != nil {
		utilArgs := utilities.ArgsFromSliceToMap(tcp.Spec.ControlPlane.Deployment.ExtraArgs.Kine)

		// Merging the user-space arguments with the Kamaji ones.
		for k, v := range utilArgs {
			args[k] = v
		}
	}

	switch d.DataStore.Spec.Driver {
	case kamajiv1alpha1.KineMySQLDriver:
		args["--endpoint"] = "mysql://$(DB_USER):$(DB_PASSWORD)@tcp($(DB_CONNECTION_STRING))/$(DB_SCHEMA)"
	case kamajiv1alpha1.KinePostgreSQLDriver:
		args["--endpoint"] = "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_CONNECTION_STRING)/$(DB_SCHEMA)"
	case kamajiv1alpha1.KineNatsDriver:
		args["--endpoint"] = "nats://$(DB_USER):$(DB_PASSWORD)@$(DB_CONNECTION_STRING)?bucket=$(DB_SCHEMA)&noEmbed"
	}

	podSpec.Containers[index].Name = kineContainerName
	podSpec.Containers[index].Image = d.KineContainerImage
	podSpec.Containers[index].Command = []string{"/bin/kine"}
	podSpec.Containers[index].Args = utilities.ArgsFromMapToSlice(args)
	podSpec.Containers[index].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      kineVolumeCertName,
			MountPath: "/certs",
			ReadOnly:  false,
		},
		{
			Name:      kineUDSVolume,
			MountPath: kineUDSFolder,
			ReadOnly:  false,
		},
	}
	podSpec.Containers[index].Env = []corev1.EnvVar{
		{
			Name:  "GODEBUG",
			Value: "x509ignoreCN=0",
		},
	}
	podSpec.Containers[index].EnvFrom = []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: tcp.Status.Storage.Config.SecretName,
				},
			},
		},
	}
	podSpec.Containers[index].Ports = []corev1.ContainerPort{
		{
			ContainerPort: 2379,
			Name:          "server",
			Protocol:      corev1.ProtocolTCP,
		},
	}

	podSpec.Containers[index].ImagePullPolicy = corev1.PullAlways

	switch {
	case tcp.Spec.ControlPlane.Deployment.Resources == nil:
		podSpec.Containers[index].Resources = corev1.ResourceRequirements{}
	case tcp.Spec.ControlPlane.Deployment.Resources.Kine != nil:
		podSpec.Containers[index].Resources = *tcp.Spec.ControlPlane.Deployment.Resources.Kine
	default:
		podSpec.Containers[index].Resources = corev1.ResourceRequirements{}
	}
}

func (d Deployment) setSelector(deploymentSpec *appsv1.DeploymentSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	deploymentSpec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"kamaji.clastix.io/name": tcp.GetName(),
		},
	}
}

func (d Deployment) setReplicas(deploymentSpec *appsv1.DeploymentSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	deploymentSpec.Replicas = tcp.Spec.ControlPlane.Deployment.Replicas
}

func (d Deployment) setRuntimeClass(spec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	if len(tcp.Spec.ControlPlane.Deployment.RuntimeClassName) > 0 {
		spec.RuntimeClassName = pointer.To(tcp.Spec.ControlPlane.Deployment.RuntimeClassName)

		return
	}

	spec.RuntimeClassName = nil
}

func (d Deployment) templateLabels(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (labels map[string]string) {
	hash := func(ctx context.Context, namespace, secretName string) string {
		h, _ := d.secretHashValue(ctx, d.Client, namespace, secretName)

		return h
	}

	labels = map[string]string{
		"kamaji.clastix.io/name":                                            tenantControlPlane.GetName(),
		"kamaji.clastix.io/component":                                       "deployment",
		"component.kamaji.clastix.io/api-server-certificate":                hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.APIServer.SecretName),
		"component.kamaji.clastix.io/api-server-kubelet-client-certificate": hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.APIServerKubeletClient.SecretName),
		"component.kamaji.clastix.io/ca":                                    hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.CA.SecretName),
		"component.kamaji.clastix.io/controller-manager-kubeconfig":         hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.KubeConfig.ControllerManager.SecretName),
		"component.kamaji.clastix.io/front-proxy-ca-certificate":            hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.FrontProxyCA.SecretName),
		"component.kamaji.clastix.io/front-proxy-client-certificate":        hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.FrontProxyClient.SecretName),
		"component.kamaji.clastix.io/service-account":                       hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.SA.SecretName),
		"component.kamaji.clastix.io/scheduler-kubeconfig":                  hash(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.KubeConfig.Scheduler.SecretName),
		"component.kamaji.clastix.io/datastore":                             tenantControlPlane.Status.Storage.DataStoreName,
	}

	return labels
}

// secretHashValue function returns the md5 value for the secret of the given name and namespace.
func (d Deployment) secretHashValue(ctx context.Context, client client.Client, namespace, name string) (string, error) {
	secret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
		return "", fmt.Errorf("cannot retrieve *corev1.Secret for resource version retrieval: %w", err)
	}

	return d.hashValue(*secret), nil
}

// hashValue function returns the md5 value for the given secret.
func (d Deployment) hashValue(secret corev1.Secret) string {
	// Go access map values in random way, it means we have to sort them.
	keys := make([]string, 0, len(secret.Data))

	for k := range secret.Data {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	// Generating MD5 of Secret values, sorted by key
	h := md5.New()

	for _, key := range keys {
		h.Write(secret.Data[key])
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

func (d Deployment) setTemplateLabels(template *corev1.PodTemplateSpec, labels map[string]string) {
	template.SetLabels(labels)
}

func (d Deployment) setTemplateAnnotations(template *corev1.PodTemplateSpec, annotations map[string]string) {
	template.SetAnnotations(annotations)
}

func (d Deployment) setLabels(resource *appsv1.Deployment, labels map[string]string) {
	resource.SetLabels(labels)
}

func (d Deployment) setAnnotations(resource *appsv1.Deployment, annotations map[string]string) {
	resource.SetAnnotations(annotations)
}

func (d Deployment) setTopologySpreadConstraints(spec *appsv1.DeploymentSpec, topologies []corev1.TopologySpreadConstraint) {
	defaultSelector := spec.Selector

	for index, topology := range topologies {
		if topology.LabelSelector == nil {
			topologies[index].LabelSelector = defaultSelector
		}
	}

	spec.Template.Spec.TopologySpreadConstraints = topologies
}

// resetKubeAPIServerFlags ensures that upon a change of the kube-apiserver extra flags the desired ones are properly
// applied, also considering that the container could be lately patched by the konnectivity addon resources.
func (d Deployment) resetKubeAPIServerFlags(resource *appsv1.Deployment, tcp kamajiv1alpha1.TenantControlPlane) {
	if tcp.Spec.ControlPlane.Deployment.ExtraArgs == nil {
		return
	}
	// kube-apiserver container is not still there, we can skip the hashing
	if found, _ := utilities.HasNamedContainer(resource.Spec.Template.Spec.Containers, apiServerContainerName); !found {
		return
	}
	// setting up annotation to avoid assignment to a nil one
	if resource.GetAnnotations() == nil {
		resource.SetAnnotations(map[string]string{})
	}
	// retrieving the current amount of extra flags, used as a sort of hash:
	// in case of non-matching values, removing all the args in order to perform a full reconciliation from a clean start.
	var count int

	if v, ok := resource.GetAnnotations()[apiServerFlagsAnnotation]; ok {
		var err error

		if count, err = strconv.Atoi(v); err != nil {
			return
		}
	}
	// there's a mismatch in the count from the previous hash: let's reset and store the desired extra args count.
	if count != len(tcp.Spec.ControlPlane.Deployment.ExtraArgs.APIServer) {
		_, index := utilities.HasNamedContainer(resource.Spec.Template.Spec.Containers, apiServerContainerName)
		resource.Spec.Template.Spec.Containers[index].Args = []string{}
	}

	resource.GetAnnotations()[apiServerFlagsAnnotation] = fmt.Sprintf("%d", len(tcp.Spec.ControlPlane.Deployment.ExtraArgs.APIServer))
}

func (d Deployment) setNodeSelector(spec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	spec.NodeSelector = tcp.Spec.ControlPlane.Deployment.NodeSelector
}

func (d Deployment) setToleration(spec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	spec.Tolerations = tcp.Spec.ControlPlane.Deployment.Tolerations
}

func (d Deployment) setAffinity(spec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	spec.Affinity = tcp.Spec.ControlPlane.Deployment.Affinity
}

func (d Deployment) setServiceAccount(spec *corev1.PodSpec, tcp kamajiv1alpha1.TenantControlPlane) {
	if len(tcp.Spec.ControlPlane.Deployment.ServiceAccountName) > 0 {
		spec.ServiceAccountName = tcp.Spec.ControlPlane.Deployment.ServiceAccountName

		return
	}

	spec.ServiceAccountName = "default"
}
