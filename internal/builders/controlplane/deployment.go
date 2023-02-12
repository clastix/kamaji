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
	"github.com/clastix/kamaji/internal/utilities"
)

type orderedIndex int

const (
	apiServerIndex orderedIndex = iota
	schedulerIndex
	controllerManagerIndex
)

const (
	etcKubernetesPKIVolume orderedIndex = iota
	etcCACertificates
	etcSSLCerts
	usrShareCACertificates
	usrLocalShareCACertificates
	schedulerKubeconfig
	controllerManagerKubeconfig
)

const (
	apiServerFlagsAnnotation = "kube-apiserver.kamaji.clastix.io/args"
	kineContainerName        = "kine"
	dataStoreCerts           = "kine-config"
	kineVolumeCertName       = "kine-certs"
)

type Deployment struct {
	Address            string
	KineContainerImage string
	DataStore          kamajiv1alpha1.DataStore
}

func (d *Deployment) SetContainers(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane, address string) {
	d.buildKubeAPIServer(podSpec, tcp, address)
	d.BuildScheduler(podSpec, tcp)
	d.buildControllerManager(podSpec, tcp)
	d.buildKine(podSpec, tcp)
}

func (d *Deployment) SetStrategy(deployment *appsv1.DeploymentSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
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

		maxUnavailable := intstr.FromInt(0)

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

	podSpec.Volumes[etcKubernetesPKIVolume] = corev1.Volume{
		Name: "etc-kubernetes-pki",
		VolumeSource: corev1.VolumeSource{
			Projected: &corev1.ProjectedVolumeSource{
				Sources:     sources,
				DefaultMode: pointer.Int32(420),
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
				DefaultMode: pointer.Int32(420),
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
				DefaultMode: pointer.Int32(420),
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
				DefaultMode: pointer.Int32(420),
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
				DefaultMode: pointer.Int32(420),
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
				DefaultMode: pointer.Int32(420),
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
				DefaultMode: pointer.Int32(420),
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
	args["--leader-elect"] = "true" //nolint:goconst

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
			podSpec.Containers[schedulerIndex].Resources.Limits = resource.Limits
			podSpec.Containers[schedulerIndex].Resources.Requests = resource.Requests
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
			podSpec.Containers[controllerManagerIndex].Resources.Limits = resource.Limits
			podSpec.Containers[controllerManagerIndex].Resources.Requests = resource.Requests
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
			podSpec.Containers[apiServerIndex].Resources.Limits = resource.Limits
			podSpec.Containers[apiServerIndex].Resources.Requests = resource.Requests
		}
	}
}

func (d *Deployment) buildKubeAPIServerCommand(tenantControlPlane *kamajiv1alpha1.TenantControlPlane, address string, current map[string]string) map[string]string {
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
	case kamajiv1alpha1.KineMySQLDriver, kamajiv1alpha1.KinePostgreSQLDriver:
		desiredArgs["--etcd-servers"] = "http://127.0.0.1:2379"
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

func (d *Deployment) removeKineVolumes(podSpec *corev1.PodSpec) {
	if found, index := utilities.HasNamedVolume(podSpec.Volumes, kineVolumeCertName); found {
		var volumes []corev1.Volume

		volumes = append(volumes, podSpec.Volumes[:index]...)
		volumes = append(volumes, podSpec.Volumes[index+1:]...)

		podSpec.Volumes = volumes
	}
}

func (d *Deployment) buildKineVolume(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	// Adding the volume for chmod'ed Kine certificates.
	found, index := utilities.HasNamedVolume(podSpec.Volumes, dataStoreCerts)
	if !found {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
		index = len(podSpec.Volumes) - 1
	}

	podSpec.Volumes[index].Name = dataStoreCerts
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{
			SecretName:  tcp.Status.Storage.Certificate.SecretName,
			DefaultMode: pointer.Int32(420),
		},
	}
	if d.DataStore.Spec.Driver == kamajiv1alpha1.EtcdDriver {
		d.removeKineVolumes(podSpec)

		return
	}
	// Adding the volume to read Kine certificates:
	// these must be subsequently fixed with a chmod due to pg issues with private key.
	if found, index = utilities.HasNamedVolume(podSpec.Volumes, kineVolumeCertName); !found {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{})
		index = len(podSpec.Volumes) - 1
	}

	podSpec.Volumes[index].Name = kineVolumeCertName
	podSpec.Volumes[index].VolumeSource = corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}
}

func (d *Deployment) removeKineContainers(podSpec *corev1.PodSpec) {
	found, index := utilities.HasNamedContainer(podSpec.Containers, kineContainerName)
	if found {
		var containers []corev1.Container

		containers = append(containers, podSpec.Containers[:index]...)
		containers = append(containers, podSpec.Containers[index+1:]...)

		podSpec.Containers = containers
	}

	podSpec.InitContainers = nil
}

func (d *Deployment) buildKine(podSpec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	if d.DataStore.Spec.Driver == kamajiv1alpha1.EtcdDriver {
		d.removeKineContainers(podSpec)

		return
	}
	// Kine is expecting an additional container, and it must be removed before proceeding with the additional one
	// in order to make this function idempotent.
	found, index := utilities.HasNamedContainer(podSpec.Containers, kineContainerName)
	if !found {
		podSpec.Containers = append(podSpec.Containers, corev1.Container{})
		index = len(podSpec.Containers) - 1
	}

	args := map[string]string{}

	if tcp.Spec.ControlPlane.Deployment.ExtraArgs != nil {
		args = utilities.ArgsFromSliceToMap(tcp.Spec.ControlPlane.Deployment.ExtraArgs.Kine)
	}

	switch d.DataStore.Spec.Driver {
	case kamajiv1alpha1.KineMySQLDriver:
		args["--endpoint"] = "mysql://$(DB_USER):$(DB_PASSWORD)@tcp($(DB_CONNECTION_STRING))/$(DB_SCHEMA)"
	case kamajiv1alpha1.KinePostgreSQLDriver:
		args["--endpoint"] = "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_CONNECTION_STRING)/$(DB_SCHEMA)"
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
					Name:      dataStoreCerts,
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
	}
	podSpec.Containers[index].TerminationMessagePath = corev1.TerminationMessagePathDefault
	podSpec.Containers[index].TerminationMessagePolicy = corev1.TerminationMessageReadFile
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
}

func (d *Deployment) SetSelector(deploymentSpec *appsv1.DeploymentSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	deploymentSpec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			"kamaji.clastix.io/name": tcp.GetName(),
		},
	}
}

func (d *Deployment) SetReplicas(deploymentSpec *appsv1.DeploymentSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	deploymentSpec.Replicas = pointer.Int32(tcp.Spec.ControlPlane.Deployment.Replicas)
}

func (d *Deployment) SetRuntimeClass(spec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	if len(tcp.Spec.ControlPlane.Deployment.RuntimeClassName) > 0 {
		spec.RuntimeClassName = pointer.String(tcp.Spec.ControlPlane.Deployment.RuntimeClassName)

		return
	}

	spec.RuntimeClassName = nil
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

func (d *Deployment) SetTopologySpreadConstraints(spec *appsv1.DeploymentSpec, topologies []corev1.TopologySpreadConstraint) {
	defaultSelector := spec.Selector

	for index, topology := range topologies {
		if topology.LabelSelector == nil {
			topologies[index].LabelSelector = defaultSelector
		}
	}

	spec.Template.Spec.TopologySpreadConstraints = topologies
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

func (d *Deployment) SetNodeSelector(spec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	spec.NodeSelector = tcp.Spec.ControlPlane.Deployment.NodeSelector
}

func (d *Deployment) SetToleration(spec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	spec.Tolerations = tcp.Spec.ControlPlane.Deployment.Tolerations
}

func (d *Deployment) SetAffinity(spec *corev1.PodSpec, tcp *kamajiv1alpha1.TenantControlPlane) {
	spec.Affinity = tcp.Spec.ControlPlane.Deployment.Affinity
}
