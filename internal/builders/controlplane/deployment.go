// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta3"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/utils/pointer"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/types"
	"github.com/clastix/kamaji/internal/utilities"
)

type orderedIndex int

const (
	apiServerIndex orderedIndex = iota
	schedulerIndex
	controllerManagerIndex
	kineIndex
)

const (
	etcKubernetesPKIVolume orderedIndex = iota
	etcCACertificates
	etcSSLCerts
	usrShareCACertificates
	usrLocalShareCACertificates
	schedulerKubeconfig
	controllerManagerKubeconfig
	kineConfig
	kineCerts
)

const (
	apiServerFlagsAnnotation = "kube-apiserver.kamaji.clastix.io/args"
	kineVolumeName           = "kine-config"
	kineVolumeCertName       = "kine-certs"
)

type Deployment struct {
	Address                string
	ETCDEndpoints          []string
	ETCDCompactionInterval string
	ETCDStorageType        types.ETCDStorageType
	KineContainerImage     string
}

func (d *Deployment) SetContainers(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane, address string) {
	d.buildKubeAPIServer(podSpec, tcp, address)
	d.BuildScheduler(podSpec, tcp)
	d.buildControllerManager(podSpec, tcp)
	d.buildKine(podSpec, tcp)
}

func (d *Deployment) SetStrategy(deployment *appsv1.DeploymentSpec) {
	maxSurge := intstr.FromString("100%")

	maxUnavailable := intstr.FromInt(0)

	deployment.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &maxUnavailable,
			MaxSurge:       &maxSurge,
		},
	}
}

func (d *Deployment) SetVolumes(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	for _, fn := range []func(*corev1.PodSpec, *kamajiv1alpha1.TenantControlPlane){
		d.buildPKIVolume,
		d.buildCAVolume,
		d.buildSSLCertsVolume,
		d.buildShareCAVolume,
		d.buildLocalShareCAVolume,
		d.buildSchedulerVolume,
		d.buildControllerManagerVolume,
		d.buildKineVolume,
	} {
		fn(podSpec, tcp)
	}
}

func (d *Deployment) buildPKIVolume(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	if index := int(etcKubernetesPKIVolume) + 1; len(podSpec.Volumes) < index {
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

	if d.ETCDStorageType == types.ETCD {
		sources = append(sources, corev1.VolumeProjection{
			Secret: d.secretProjection(tcp.Status.Certificates.ETCD.APIServer.SecretName, constants.APIServerEtcdClientCertName, constants.APIServerEtcdClientKeyName),
		})
		sources = append(sources, corev1.VolumeProjection{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: tcp.Status.Certificates.ETCD.CA.SecretName,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  constants.CACertName,
						Path: constants.EtcdCACertName,
					},
				},
			},
		})
	}

	podSpec.Volumes[etcKubernetesPKIVolume] = corev1.Volume{
		Name: "etc-kubernetes-pki",
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources:     sources,
				DefaultMode: pointer.Int32Ptr(420),
			},
		},
	}
}

func (d *Deployment) buildCAVolume(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	if index := int(etcCACertificates) + 1; len(podSpec.Volumes) < index {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[etcCACertificates] = corev1.Volume{
		Name: "etc-ca-certificates",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  tcp.Status.Certificates.CA.SecretName,
				DefaultMode: pointer.Int32Ptr(420),
			},
		},
	}
}

func (d *Deployment) buildSSLCertsVolume(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	if index := int(etcSSLCerts) + 1; len(podSpec.Volumes) < index {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[etcSSLCerts] = corev1.Volume{
		Name: "etc-ssl-certs",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  tcp.Status.Certificates.CA.SecretName,
				DefaultMode: pointer.Int32Ptr(420),
			},
		},
	}
}

func (d *Deployment) buildShareCAVolume(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	if index := int(usrShareCACertificates) + 1; len(podSpec.Volumes) < index {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[usrShareCACertificates] = corev1.Volume{
		Name: "usr-share-ca-certificates",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  tcp.Status.Certificates.CA.SecretName,
				DefaultMode: pointer.Int32Ptr(420),
			},
		},
	}
}

func (d *Deployment) buildLocalShareCAVolume(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	if index := int(usrLocalShareCACertificates) + 1; len(podSpec.Volumes) < index {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[usrLocalShareCACertificates] = corev1.Volume{
		Name: "usr-local-share-ca-certificates",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  tcp.Status.Certificates.CA.SecretName,
				DefaultMode: pointer.Int32Ptr(420),
			},
		},
	}
}

func (d *Deployment) buildSchedulerVolume(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	if index := int(schedulerKubeconfig) + 1; len(podSpec.Volumes) < index {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[schedulerKubeconfig] = corev1.Volume{
		Name: "scheduler-kubeconfig",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  tcp.Status.KubeConfig.Scheduler.SecretName,
				DefaultMode: pointer.Int32Ptr(420),
			},
		},
	}
}

func (d *Deployment) buildControllerManagerVolume(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	if index := int(controllerManagerKubeconfig) + 1; len(podSpec.Volumes) < index {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
	}

	podSpec.Volumes[controllerManagerKubeconfig] = corev1.Volume{
		Name: "controller-manager-kubeconfig",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  tcp.Status.KubeConfig.ControllerManager.SecretName,
				DefaultMode: pointer.Int32Ptr(420),
			},
		},
	}
}

func (d *Deployment) BuildScheduler(podSpec *corev1.PodSpec, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) {
	if index := int(schedulerIndex) + 1; len(podSpec.Containers) < index {
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
	args["--leader-elect"] = "true" // nolint:goconst

	podSpec.Containers[schedulerIndex].Name = "kube-scheduler"
	podSpec.Containers[schedulerIndex].Image = fmt.Sprintf("k8s.gcr.io/kube-scheduler:%s", tenantControlPlane.Spec.Kubernetes.Version)
	podSpec.Containers[schedulerIndex].Command = []string{"kube-scheduler"}
	podSpec.Containers[schedulerIndex].Args = utilities.ArgsFromMapToSlice(args)
	podSpec.Containers[schedulerIndex].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "scheduler-kubeconfig",
			ReadOnly:  true,
			MountPath: "/etc/kubernetes",
		},
	}
	podSpec.Containers[schedulerIndex].LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt(10259),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[schedulerIndex].StartupProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt(10259),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[schedulerIndex].ImagePullPolicy = corev1.PullAlways
	podSpec.Containers[schedulerIndex].Resources = corev1.ResourceRequirements{
		Limits:   nil,
		Requests: nil,
	}

	if componentsResources := tenantControlPlane.Spec.ControlPlane.Deployment.Resources; componentsResources != nil {
		if resource := componentsResources.Scheduler; resource != nil {
			podSpec.Containers[schedulerIndex].Resources = *resource
		}
	}
}

func (d *Deployment) buildControllerManager(podSpec *corev1.PodSpec, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) {
	if index := int(controllerManagerIndex) + 1; len(podSpec.Containers) < index {
		podSpec.Containers = append(podSpec.Containers, corev1.Container{})
	}

	args := map[string]string{}

	if tenantControlPlane.Spec.ControlPlane.Deployment.ExtraArgs != nil {
		args = utilities.ArgsFromSliceToMap(tenantControlPlane.Spec.ControlPlane.Deployment.ExtraArgs.ControllerManager)
	}

	kubeconfig := "/etc/kubernetes/controller-manager.conf"

	args["--allocate-node-cidrs"] = "true"
	args["--authentication-kubeconfig"] = kubeconfig
	args["--authorization-kubeconfig"] = kubeconfig
	args["--bind-address"] = "0.0.0.0"
	args["--client-ca-file"] = path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName)
	args["--cluster-name"] = tenantControlPlane.GetName()
	args["--cluster-signing-cert-file"] = path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName)
	args["--cluster-signing-key-file"] = path.Join(v1beta3.DefaultCertificatesDir, constants.CAKeyName)
	args["--controllers"] = "*,bootstrapsigner,tokencleaner"
	args["--kubeconfig"] = kubeconfig
	args["--leader-elect"] = "true"
	args["--service-cluster-ip-range"] = tenantControlPlane.Spec.NetworkProfile.ServiceCIDR
	args["--cluster-cidr"] = tenantControlPlane.Spec.NetworkProfile.PodCIDR
	args["--requestheader-client-ca-file"] = path.Join(v1beta3.DefaultCertificatesDir, constants.FrontProxyCACertName)
	args["--root-ca-file"] = path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName)
	args["--service-account-private-key-file"] = path.Join(v1beta3.DefaultCertificatesDir, constants.ServiceAccountPrivateKeyName)
	args["--use-service-account-credentials"] = "true"

	podSpec.Containers[controllerManagerIndex].Name = "kube-controller-manager"
	podSpec.Containers[controllerManagerIndex].Image = fmt.Sprintf("k8s.gcr.io/kube-controller-manager:%s", tenantControlPlane.Spec.Kubernetes.Version)
	podSpec.Containers[controllerManagerIndex].Command = []string{"kube-controller-manager"}
	podSpec.Containers[controllerManagerIndex].Args = utilities.ArgsFromMapToSlice(args)
	podSpec.Containers[controllerManagerIndex].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "controller-manager-kubeconfig",
			ReadOnly:  true,
			MountPath: "/etc/kubernetes",
		},
		{
			Name:      "etc-kubernetes-pki",
			ReadOnly:  true,
			MountPath: v1beta3.DefaultCertificatesDir,
		},
		{
			Name:      "etc-ca-certificates",
			ReadOnly:  true,
			MountPath: "/etc/ca-certificates",
		},
		{
			Name:      "etc-ssl-certs",
			ReadOnly:  true,
			MountPath: "/etc/ssl/certs",
		},
		{
			Name:      "usr-share-ca-certificates",
			ReadOnly:  true,
			MountPath: "/usr/share/ca-certificates",
		},
		{
			Name:      "usr-local-share-ca-certificates",
			ReadOnly:  true,
			MountPath: "/usr/local/share/ca-certificates",
		},
	}
	podSpec.Containers[controllerManagerIndex].LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt(10257),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[controllerManagerIndex].StartupProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/healthz",
				Port:   intstr.FromInt(10257),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[controllerManagerIndex].ImagePullPolicy = corev1.PullAlways
	podSpec.Containers[controllerManagerIndex].Resources = corev1.ResourceRequirements{
		Limits:   nil,
		Requests: nil,
	}

	if componentsResources := tenantControlPlane.Spec.ControlPlane.Deployment.Resources; componentsResources != nil {
		if resource := componentsResources.ControllerManager; resource != nil {
			podSpec.Containers[controllerManagerIndex].Resources = *resource
		}
	}
}

func (d *Deployment) buildKubeAPIServer(podSpec *corev1.PodSpec, tenantControlPlane *kamajiv1alpha1.TenantControlPlane, address string) {
	if index := int(apiServerIndex) + 1; len(podSpec.Containers) < index {
		podSpec.Containers = append(podSpec.Containers, corev1.Container{})
	}

	args := d.buildKubeAPIServerCommand(tenantControlPlane, address, utilities.ArgsFromSliceToMap(podSpec.Containers[apiServerIndex].Args))

	podSpec.Containers[apiServerIndex].Name = "kube-apiserver"
	podSpec.Containers[apiServerIndex].Args = utilities.ArgsFromMapToSlice(args)
	podSpec.Containers[apiServerIndex].Image = fmt.Sprintf("k8s.gcr.io/kube-apiserver:%s", tenantControlPlane.Spec.Kubernetes.Version)
	podSpec.Containers[apiServerIndex].Command = []string{"kube-apiserver"}
	podSpec.Containers[apiServerIndex].LivenessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/livez",
				Port:   intstr.FromInt(int(tenantControlPlane.Spec.NetworkProfile.Port)),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[apiServerIndex].ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/readyz",
				Port:   intstr.FromInt(int(tenantControlPlane.Spec.NetworkProfile.Port)),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[apiServerIndex].StartupProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/livez",
				Port:   intstr.FromInt(int(tenantControlPlane.Spec.NetworkProfile.Port)),
				Scheme: corev1.URISchemeHTTPS,
			},
		},
		InitialDelaySeconds: 0,
		TimeoutSeconds:      1,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
	podSpec.Containers[apiServerIndex].ImagePullPolicy = corev1.PullAlways

	if len(podSpec.Containers[apiServerIndex].VolumeMounts) < 5 {
		podSpec.Containers[apiServerIndex].VolumeMounts = make([]corev1.VolumeMount, 5)
	}
	podSpec.Containers[apiServerIndex].VolumeMounts[0] = corev1.VolumeMount{
		Name:      "etc-kubernetes-pki",
		ReadOnly:  true,
		MountPath: v1beta3.DefaultCertificatesDir,
	}
	podSpec.Containers[apiServerIndex].VolumeMounts[1] = corev1.VolumeMount{
		Name:      "etc-ca-certificates",
		ReadOnly:  true,
		MountPath: "/etc/ca-certificates",
	}
	podSpec.Containers[apiServerIndex].VolumeMounts[2] = corev1.VolumeMount{
		Name:      "etc-ssl-certs",
		ReadOnly:  true,
		MountPath: "/etc/ssl/certs",
	}
	podSpec.Containers[apiServerIndex].VolumeMounts[3] = corev1.VolumeMount{
		Name:      "usr-share-ca-certificates",
		ReadOnly:  true,
		MountPath: "/usr/share/ca-certificates",
	}
	podSpec.Containers[apiServerIndex].VolumeMounts[4] = corev1.VolumeMount{
		Name:      "usr-local-share-ca-certificates",
		ReadOnly:  true,
		MountPath: "/usr/local/share/ca-certificates",
	}
	podSpec.Containers[apiServerIndex].Resources = corev1.ResourceRequirements{
		Limits:   nil,
		Requests: nil,
	}

	if componentsResources := tenantControlPlane.Spec.ControlPlane.Deployment.Resources; componentsResources != nil {
		if resource := componentsResources.APIServer; resource != nil {
			podSpec.Containers[apiServerIndex].Resources = *resource
		}
	}
}

func (d *Deployment) buildKubeAPIServerCommand(tenantControlPlane *kamajiv1alpha1.TenantControlPlane, address string, current map[string]string) map[string]string {
	var extraArgs map[string]string

	if tenantControlPlane.Spec.ControlPlane.Deployment.ExtraArgs != nil {
		extraArgs = utilities.ArgsFromSliceToMap(tenantControlPlane.Spec.ControlPlane.Deployment.ExtraArgs.APIServer)
	}

	desiredArgs := map[string]string{
		"--allow-privileged":                   "true",
		"--authorization-mode":                 "Node,RBAC",
		"--advertise-address":                  address,
		"--client-ca-file":                     path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName),
		"--enable-admission-plugins":           strings.Join(tenantControlPlane.Spec.Kubernetes.AdmissionControllers.ToSlice(), ","),
		"--enable-bootstrap-token-auth":        "true",
		"--etcd-servers":                       strings.Join(d.ETCDEndpoints, ","),
		"--service-cluster-ip-range":           tenantControlPlane.Spec.NetworkProfile.ServiceCIDR,
		"--kubelet-client-certificate":         path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerKubeletClientCertName),
		"--kubelet-client-key":                 path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerKubeletClientKeyName),
		"--kubelet-preferred-address-types":    "Hostname,InternalIP,ExternalIP",
		"--proxy-client-cert-file":             path.Join(v1beta3.DefaultCertificatesDir, constants.FrontProxyClientCertName),
		"--proxy-client-key-file":              path.Join(v1beta3.DefaultCertificatesDir, constants.FrontProxyClientKeyName),
		"--requestheader-allowed-names":        "front-proxy-client",
		"--requestheader-extra-headers-prefix": "X-Remote-Extra-",
		"--requestheader-group-headers":        "X-Remote-Group",
		"--requestheader-username-headers":     "X-Remote-User",
		"--secure-port":                        fmt.Sprintf("%d", tenantControlPlane.Spec.NetworkProfile.Port),
		"--service-account-issuer":             fmt.Sprintf("https://localhost:%d", tenantControlPlane.Spec.NetworkProfile.Port),
		"--service-account-key-file":           path.Join(v1beta3.DefaultCertificatesDir, constants.ServiceAccountPublicKeyName),
		"--service-account-signing-key-file":   path.Join(v1beta3.DefaultCertificatesDir, constants.ServiceAccountPrivateKeyName),
		"--tls-cert-file":                      path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerCertName),
		"--tls-private-key-file":               path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerKeyName),
	}

	if d.ETCDStorageType == types.ETCD {
		desiredArgs["--etcd-compaction-interval"] = d.ETCDCompactionInterval
		desiredArgs["--etcd-cafile"] = path.Join(v1beta3.DefaultCertificatesDir, constants.EtcdCACertName)
		desiredArgs["--etcd-certfile"] = path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerEtcdClientCertName)
		desiredArgs["--etcd-keyfile"] = path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerEtcdClientKeyName)
		desiredArgs["--etcd-prefix"] = fmt.Sprintf("/%s", tenantControlPlane.GetName())
	}
	// Order matters, here: extraArgs could try to overwrite some arguments managed by Kamaji and that would be crucial.
	// Adding as first element of the array of maps, we're sure that these overrides will be sanitized by our configuration.
	return utilities.MergeMaps(extraArgs, current, desiredArgs)
}

func (d *Deployment) secretProjection(secretName, certKeyName, keyName string) *corev1.SecretProjection {
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

func (d *Deployment) buildKineVolume(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	// Kine is expecting an additional volume for its configuration, and it must be removed before proceeding with the
	// customized storage that is idempotent
	if found, index := utilities.HasNamedVolume(podSpec.Volumes, kineVolumeName); found {
		var volumes []corev1.Volume

		volumes = append(volumes, podSpec.Volumes[:index]...)
		volumes = append(volumes, podSpec.Volumes[index+1:]...)

		podSpec.Volumes = volumes
	}

	if found, index := utilities.HasNamedVolume(podSpec.Volumes, kineVolumeCertName); found {
		var volumes []corev1.Volume

		volumes = append(volumes, podSpec.Volumes[:index]...)
		volumes = append(volumes, podSpec.Volumes[index+1:]...)

		podSpec.Volumes = volumes
	}

	if d.ETCDStorageType == types.KineMySQL || d.ETCDStorageType == types.KinePostgreSQL {
		if index := int(kineConfig) + 1; len(podSpec.Volumes) < index {
			podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
		}
		// Adding the volume to read Kine certificates:
		// these must be subsequently fixed with a chmod due to pg issues with private key.
		podSpec.Volumes[kineConfig].Name = kineVolumeName
		podSpec.Volumes[kineConfig].VolumeSource = corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  tcp.Status.Storage.Kine.Certificate.SecretName,
				DefaultMode: pointer.Int32Ptr(420),
			},
		}
		// Adding the Volume for the certificates with fixed permission
		if index := int(kineCerts) + 1; len(podSpec.Volumes) < index {
			podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
		}

		podSpec.Volumes[kineCerts].Name = kineVolumeCertName
		podSpec.Volumes[kineCerts].VolumeSource = corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}
	}
}

func (d *Deployment) buildKine(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	const kineContainerName = "kine"
	// Kine is expecting an additional container, and it must be removed before proceeding with the additional one
	// in order to make this function idempotent.
	if found, index := utilities.HasNamedContainer(podSpec.Containers, kineContainerName); found {
		var containers []corev1.Container

		containers = append(containers, podSpec.Containers[:index]...)
		containers = append(containers, podSpec.Containers[index+1:]...)

		podSpec.Containers = containers
	}
	// In case of bare ETCD we exit without mangling the PodSpec resource.
	if d.ETCDStorageType == types.ETCD {
		return
	}

	if index := int(kineIndex) + 1; len(podSpec.Containers) < index {
		podSpec.Containers = append(podSpec.Containers, corev1.Container{})
	}

	args := map[string]string{}

	if tcp.Spec.ControlPlane.Deployment.ExtraArgs != nil {
		args = utilities.ArgsFromSliceToMap(tcp.Spec.ControlPlane.Deployment.ExtraArgs.Kine)
	}

	switch d.ETCDStorageType {
	case types.KineMySQL:
		args["--endpoint"] = "mysql://$(DB_USER):$(DB_PASSWORD)@tcp($(DB_HOST):$(DB_PORT))/$(DB_SCHEMA)"
	case types.KinePostgreSQL:
		args["--endpoint"] = "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_SCHEMA)"
	}

	args["--ca-file"] = "/certs/ca.crt"
	args["--cert-file"] = "/certs/server.crt"
	args["--key-file"] = "/certs/server.key"

	podSpec.InitContainers = []corev1.Container{
		{
			Name:                     "chmod",
			Image:                    d.KineContainerImage,
			ImagePullPolicy:          corev1.PullAlways,
			TerminationMessagePath:   corev1.TerminationMessagePathDefault,
			TerminationMessagePolicy: corev1.TerminationMessageReadFile,
			Command:                  []string{"sh"},
			Args: []string{
				"-c",
				"cp /kine/*.* /certs && chmod -R 600 /certs/*.*",
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      kineVolumeName,
					ReadOnly:  true,
					MountPath: "/kine",
				},
				{
					Name:      kineVolumeCertName,
					MountPath: "/certs",
					ReadOnly:  false,
				},
			},
		},
	}

	podSpec.Containers[kineIndex].Name = kineContainerName
	podSpec.Containers[kineIndex].Image = d.KineContainerImage
	podSpec.Containers[kineIndex].Command = []string{"/bin/kine"}
	podSpec.Containers[kineIndex].Args = utilities.ArgsFromMapToSlice(args)
	podSpec.Containers[kineIndex].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      kineVolumeCertName,
			MountPath: "/certs",
			ReadOnly:  false,
		},
	}
	podSpec.Containers[kineIndex].TerminationMessagePath = corev1.TerminationMessagePathDefault
	podSpec.Containers[kineIndex].TerminationMessagePolicy = corev1.TerminationMessageReadFile
	podSpec.Containers[kineIndex].Env = []corev1.EnvVar{
		{
			Name:  "GODEBUG",
			Value: "x509ignoreCN=0",
		},
	}
	podSpec.Containers[kineIndex].EnvFrom = []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: tcp.Status.Storage.Kine.Config.SecretName,
				},
			},
		},
	}
	podSpec.Containers[kineIndex].Ports = []corev1.ContainerPort{
		{
			ContainerPort: 2379,
			Name:          "server",
			Protocol:      corev1.ProtocolTCP,
		},
	}
	podSpec.Containers[kineIndex].ImagePullPolicy = corev1.PullAlways
}

func (d *Deployment) SetSelector(deploymentSpec *appsv1.DeploymentSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	deploymentSpec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"kamaji.clastix.io/soot": tcp.GetName(),
		},
	}
}

func (d *Deployment) SetReplicas(deploymentSpec *appsv1.DeploymentSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	deploymentSpec.Replicas = pointer.Int32(tcp.Spec.ControlPlane.Deployment.Replicas)
}

func (d *Deployment) SetTemplateLabels(template *corev1.PodTemplateSpec, labels map[string]string) {
	template.SetLabels(labels)
}

func (d *Deployment) SetLabels(resource *appsv1.Deployment, labels map[string]string) {
	resource.SetLabels(labels)
}

func (d *Deployment) SetAnnotations(resource *appsv1.Deployment, annotations map[string]string) {
	resource.SetAnnotations(annotations)
}

// ResetKubeAPIServerFlags ensures that upon a change of the kube-apiserver extra flags the desired ones are properly
// applied, also considering that the container could be lately patched by the konnectivity addon resources.
func (d *Deployment) ResetKubeAPIServerFlags(resource *appsv1.Deployment, tcp *kamajiv1alpha1.TenantControlPlane) {
	if tcp.Spec.ControlPlane.Deployment.ExtraArgs == nil {
		return
	}
	// kube-apiserver container is not still there, we can skip the hashing
	if found, _ := utilities.HasNamedContainer(resource.Spec.Template.Spec.Containers, "kube-apiserver"); !found {
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
		resource.Spec.Template.Spec.Containers[apiServerIndex].Args = []string{}
	}

	resource.GetAnnotations()[apiServerFlagsAnnotation] = fmt.Sprintf("%d", len(tcp.Spec.ControlPlane.Deployment.ExtraArgs.APIServer))
}
