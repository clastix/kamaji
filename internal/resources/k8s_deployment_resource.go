// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	quantity "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta3"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/resources/konnectivity"
	"github.com/clastix/kamaji/internal/types"
	"github.com/clastix/kamaji/internal/utilities"
)

const (
	konnectivityEgressSelectorConfigurationPath = "/etc/kubernetes/konnectivity/configurations/egress-selector-configuration.yaml"
	konnectivityServerName                      = "konnectivity-server"
	konnectivityServerPath                      = "/run/konnectivity"
	konnectivityUDSName                         = "konnectivity-uds"
)

type KubernetesDeploymentResource struct {
	resource               *appsv1.Deployment
	Client                 client.Client
	ETCDStorageType        types.ETCDStorageType
	ETCDEndpoints          []string
	ETCDCompactionInterval string
	Name                   string
}

func (r *KubernetesDeploymentResource) isStatusEqual(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return r.resource.Status.String() == tenantControlPlane.Status.Kubernetes.Deployment.DeploymentStatus.String()
}

func (r *KubernetesDeploymentResource) ShouldStatusBeUpdated(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return !r.isStatusEqual(tenantControlPlane) || tenantControlPlane.Spec.Kubernetes.Version != tenantControlPlane.Status.Kubernetes.Version.Version
}

func (r *KubernetesDeploymentResource) ShouldCleanup(plane *kamajiv1alpha1.TenantControlPlane) bool {
	return false
}

func (r *KubernetesDeploymentResource) CleanUp(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (bool, error) {
	return tenantControlPlane.Spec.Addons.Konnectivity == nil, nil
}

func (r *KubernetesDeploymentResource) Define(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	r.resource = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tenantControlPlane.GetName(),
			Namespace: tenantControlPlane.GetNamespace(),
			Labels:    utilities.CommonLabels(tenantControlPlane.GetName()),
		},
	}

	r.Name = "deployment"

	return nil
}

func (r *KubernetesDeploymentResource) mutate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) controllerutil.MutateFn {
	maxSurge := intstr.FromString("100%")

	maxUnavailable := intstr.FromInt(0)

	address, err := tenantControlPlane.GetControlPlaneAddress(ctx, r.Client)
	if err != nil {
		return func() error {
			return errors.Wrap(err, "cannot create TenantControlPlane Deployment")
		}
	}

	return func() error {
		labels := utilities.MergeMaps(r.resource.GetLabels(), tenantControlPlane.Spec.ControlPlane.Deployment.AdditionalMetadata.Labels)
		r.resource.SetLabels(labels)

		annotations := utilities.MergeMaps(r.resource.GetAnnotations(), tenantControlPlane.Spec.ControlPlane.Deployment.AdditionalMetadata.Annotations)
		r.resource.SetAnnotations(annotations)

		r.resource.Spec.Replicas = pointer.Int32(tenantControlPlane.Spec.ControlPlane.Deployment.Replicas)
		r.resource.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"kamaji.clastix.io/soot": tenantControlPlane.GetName(),
			},
		}
		r.resource.Spec.Template.ObjectMeta = metav1.ObjectMeta{
			Labels: map[string]string{
				"kamaji.clastix.io/soot": tenantControlPlane.GetName(),
				"component.kamaji.clastix.io/api-server-certificate": func() (hash string) {
					hash, _ = utilities.SecretHashValue(ctx, r.Client, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.APIServer.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/api-server-kubelet-client-certificate": func() (hash string) {
					hash, _ = utilities.SecretHashValue(ctx, r.Client, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.APIServerKubeletClient.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/ca": func() (hash string) {
					hash, _ = utilities.SecretHashValue(ctx, r.Client, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.CA.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/controller-manager-kubeconfig": func() (hash string) {
					hash, _ = utilities.SecretHashValue(ctx, r.Client, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.KubeConfig.ControllerManager.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/front-proxy-ca-certificate": func() (hash string) {
					hash, _ = utilities.SecretHashValue(ctx, r.Client, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.FrontProxyCA.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/front-proxy-client-certificate": func() (hash string) {
					hash, _ = utilities.SecretHashValue(ctx, r.Client, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.FrontProxyClient.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/service-account": func() (hash string) {
					hash, _ = utilities.SecretHashValue(ctx, r.Client, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.SA.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/scheduler-kubeconfig": func() (hash string) {
					hash, _ = utilities.SecretHashValue(ctx, r.Client, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.KubeConfig.Scheduler.SecretName)

					return
				}(),
			},
		}
		r.resource.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: "etc-kubernetes-pki",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								Secret: secretProjection(tenantControlPlane.Status.Certificates.APIServer.SecretName, constants.APIServerCertName, constants.APIServerKeyName),
							},
							{
								Secret: secretProjection(tenantControlPlane.Status.Certificates.CA.SecretName, constants.CACertName, constants.CAKeyName),
							},
							{
								Secret: secretProjection(tenantControlPlane.Status.Certificates.APIServerKubeletClient.SecretName, constants.APIServerKubeletClientCertName, constants.APIServerKubeletClientKeyName),
							},
							{
								Secret: secretProjection(tenantControlPlane.Status.Certificates.FrontProxyCA.SecretName, constants.FrontProxyCACertName, constants.FrontProxyCAKeyName),
							},
							{
								Secret: secretProjection(tenantControlPlane.Status.Certificates.FrontProxyClient.SecretName, constants.FrontProxyClientCertName, constants.FrontProxyClientKeyName),
							},
							{
								Secret: secretProjection(tenantControlPlane.Status.Certificates.SA.SecretName, constants.ServiceAccountPublicKeyName, constants.ServiceAccountPrivateKeyName),
							},
						},
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			{
				Name: "etc-ca-certificates",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  tenantControlPlane.Status.Certificates.CA.SecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			{
				Name: "etc-ssl-certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  tenantControlPlane.Status.Certificates.CA.SecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			{
				Name: "usr-share-ca-certificates",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  tenantControlPlane.Status.Certificates.CA.SecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			{
				Name: "usr-local-share-ca-certificates",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  tenantControlPlane.Status.Certificates.CA.SecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			{
				Name: "scheduler-kubeconfig",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  tenantControlPlane.Status.KubeConfig.Scheduler.SecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			{
				Name: "controller-manager-kubeconfig",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  tenantControlPlane.Status.KubeConfig.ControllerManager.SecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
		}

		if len(r.resource.Spec.Template.Spec.Containers) < 3 {
			r.resource.Spec.Template.Spec.Containers = make([]corev1.Container, 3)
		}

		r.syncKubeApiServer(tenantControlPlane, address)
		r.syncScheduler(tenantControlPlane)
		r.syncControllerManager(tenantControlPlane)

		r.resource.Spec.Strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxUnavailable: &maxUnavailable,
				MaxSurge:       &maxSurge,
			},
		}

		r.customizeStorage(ctx, &r.resource.Spec.Template, *tenantControlPlane)

		if err := r.reconcileKonnectivity(&r.resource.Spec.Template.Spec, *tenantControlPlane); err != nil {
			return err
		}

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	}
}

func (r *KubernetesDeploymentResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	return utilities.CreateOrUpdateWithConflict(ctx, r.Client, r.resource, r.mutate(ctx, tenantControlPlane))
}

func (r *KubernetesDeploymentResource) GetName() string {
	return r.Name
}

func (r *KubernetesDeploymentResource) UpdateTenantControlPlaneStatus(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) error {
	switch {
	case !r.isProgressingUpgrade():
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionReady
		tenantControlPlane.Status.Kubernetes.Version.Version = tenantControlPlane.Spec.Kubernetes.Version
	case r.isUpgrading(tenantControlPlane):
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionUpgrading
	case r.isProvisioning(tenantControlPlane):
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionProvisioning
	case r.isNotReady():
		tenantControlPlane.Status.Kubernetes.Version.Status = &kamajiv1alpha1.VersionNotReady
	}

	tenantControlPlane.Status.Kubernetes.Deployment = kamajiv1alpha1.KubernetesDeploymentStatus{
		DeploymentStatus: r.resource.Status,
		Name:             r.resource.GetName(),
		Namespace:        r.resource.GetNamespace(),
		LastUpdate:       metav1.Now(),
	}

	return nil
}

func (r *KubernetesDeploymentResource) isProgressingUpgrade() bool {
	if r.resource.ObjectMeta.GetGeneration() != r.resource.Status.ObservedGeneration {
		return true
	}

	if r.resource.Status.UnavailableReplicas > 0 {
		return true
	}

	return false
}

func (r *KubernetesDeploymentResource) isUpgrading(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return len(tenantControlPlane.Status.Kubernetes.Version.Version) > 0 &&
		tenantControlPlane.Spec.Kubernetes.Version != tenantControlPlane.Status.Kubernetes.Version.Version &&
		r.isProgressingUpgrade()
}

func (r *KubernetesDeploymentResource) isProvisioning(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) bool {
	return len(tenantControlPlane.Status.Kubernetes.Version.Version) == 0
}

func (r *KubernetesDeploymentResource) isNotReady() bool {
	return r.resource.Status.ReadyReplicas == 0
}

func (r *KubernetesDeploymentResource) reconcileKonnectivity(podSpec *corev1.PodSpec, tenantControlPlane kamajiv1alpha1.TenantControlPlane) error {
	if tenantControlPlane.Spec.Addons.Konnectivity == nil {
		return nil
	}

	return r.addKonnectivity(podSpec, tenantControlPlane)
}

func (r *KubernetesDeploymentResource) addKonnectivity(podSpec *corev1.PodSpec, tenantControlPlane kamajiv1alpha1.TenantControlPlane) error {
	flags := r.buildKonnectivityFlags()
	podSpec.Containers[0].Command = append(podSpec.Containers[0].Command, flags...)

	volumes := r.buildKonnectivityVolumes(tenantControlPlane)
	podSpec.Volumes = append(podSpec.Volumes, volumes...)

	volumeMounts := r.buildKonnectivityVolumeMounts()
	podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, volumeMounts...)

	container := r.buildKonnectivityServerContainer(tenantControlPlane)
	podSpec.Containers = append(podSpec.Containers, container)

	return nil
}

func (r *KubernetesDeploymentResource) buildKonnectivityFlags() []string {
	return []string{
		fmt.Sprintf("--egress-selector-config-file=%s", konnectivityEgressSelectorConfigurationPath),
	}
}

func (r *KubernetesDeploymentResource) buildKonnectivityVolumes(tenantControlPlane kamajiv1alpha1.TenantControlPlane) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: konnectivityUDSName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: "Memory",
				},
			},
		},
		{
			Name: "egress-selector-configuration",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: tenantControlPlane.Status.Addons.Konnectivity.EgressSelectorConfiguration,
					},
					DefaultMode: pointer.Int32Ptr(420),
				},
			},
		},
		{
			Name: "konnectivity-server-kubeconfig",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  tenantControlPlane.Status.Addons.Konnectivity.Kubeconfig.SecretName,
					DefaultMode: pointer.Int32Ptr(420),
				},
			},
		},
	}
}

func (r *KubernetesDeploymentResource) buildKonnectivityVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      konnectivityUDSName,
			ReadOnly:  false,
			MountPath: konnectivityServerPath,
		},
		{
			Name:      "egress-selector-configuration",
			ReadOnly:  true,
			MountPath: "/etc/kubernetes/konnectivity/configurations",
		},
	}
}

func (r *KubernetesDeploymentResource) buildKonnectivityServerContainer(tenantControlPlane kamajiv1alpha1.TenantControlPlane) corev1.Container {
	return corev1.Container{
		Name:    konnectivityServerName,
		Image:   fmt.Sprintf("%s:%s", tenantControlPlane.Spec.Addons.Konnectivity.ServerImage, tenantControlPlane.Spec.Addons.Konnectivity.Version),
		Command: []string{"/proxy-server"},
		Args: []string{
			"-v=8",
			"--logtostderr=true",
			fmt.Sprintf("--uds-name=%s/konnectivity-server.socket", konnectivityServerPath),
			"--cluster-cert=/etc/kubernetes/pki/apiserver.crt",
			"--cluster-key=/etc/kubernetes/pki/apiserver.key",
			"--mode=grpc",
			"--server-port=0",
			fmt.Sprintf("--agent-port=%d", tenantControlPlane.Spec.Addons.Konnectivity.ProxyPort),
			"--admin-port=8133",
			"--health-port=8134",
			"--agent-namespace=kube-system",
			fmt.Sprintf("--agent-service-account=%s", konnectivity.AgentName),
			"--kubeconfig=/etc/kubernetes/konnectivity-server.conf",
			fmt.Sprintf("--authentication-audience=%s", konnectivity.CertCommonName),
			fmt.Sprintf("--server-count=%d", tenantControlPlane.Spec.ControlPlane.Deployment.Replicas),
		},
		LivenessProbe: &corev1.Probe{
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
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: quantity.MustParse("100m"),
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "agentport",
				ContainerPort: tenantControlPlane.Spec.Addons.Konnectivity.ProxyPort,
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
		},
		VolumeMounts: []corev1.VolumeMount{
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
				Name:      "konnectivity-uds",
				MountPath: konnectivityServerPath,
				ReadOnly:  false,
			},
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
		ImagePullPolicy:          corev1.PullAlways,
	}
}

func (r *KubernetesDeploymentResource) customizeStorage(ctx context.Context, podTemplate *corev1.PodTemplateSpec, tenantControlPlane kamajiv1alpha1.TenantControlPlane) {
	switch r.ETCDStorageType {
	case types.ETCD:
		r.customizeETCDStorage(ctx, podTemplate, tenantControlPlane)
	case types.KineMySQL:
		r.customizeKineMySQLStorage(ctx, podTemplate, tenantControlPlane)
	default:
		return
	}
}

func (r *KubernetesDeploymentResource) customizeETCDStorage(ctx context.Context, podTemplate *corev1.PodTemplateSpec, tenantControlPlane kamajiv1alpha1.TenantControlPlane) {
	labels := map[string]string{
		"component.kamaji.clastix.io/etcd-ca-certificates": func() (hash string) {
			hash, _ = utilities.SecretHashValue(ctx, r.Client, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.ETCD.CA.SecretName)

			return
		}(),
		"component.kamaji.clastix.io/etcd-certificates": func() (hash string) {
			hash, _ = utilities.SecretHashValue(ctx, r.Client, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.ETCD.APIServer.SecretName)

			return
		}(),
	}

	podTemplate.SetLabels(
		utilities.MergeMaps(labels, podTemplate.Labels),
	)

	commands := []string{
		fmt.Sprintf("--etcd-compaction-interval=%s", r.ETCDCompactionInterval),
		fmt.Sprintf("--etcd-cafile=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.EtcdCACertName)),
		fmt.Sprintf("--etcd-certfile=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerEtcdClientCertName)),
		fmt.Sprintf("--etcd-keyfile=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerEtcdClientKeyName)),
		fmt.Sprintf("--etcd-prefix=/%s", tenantControlPlane.GetName()),
	}

	podTemplate.Spec.Containers[0].Command = append(podTemplate.Spec.Containers[0].Command, commands...)

	volumeProjections := []corev1.VolumeProjection{
		{
			Secret: secretProjection(tenantControlPlane.Status.Certificates.ETCD.APIServer.SecretName, constants.APIServerEtcdClientCertName, constants.APIServerEtcdClientKeyName),
		},
		{
			Secret: &corev1.SecretProjection{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: tenantControlPlane.Status.Certificates.ETCD.CA.SecretName,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  constants.CACertName,
						Path: constants.EtcdCACertName,
					},
				},
			},
		},
	}

	podTemplate.Spec.Volumes[0].VolumeSource.Projected.Sources = append(podTemplate.Spec.Volumes[0].VolumeSource.Projected.Sources, volumeProjections...)
}

func (r *KubernetesDeploymentResource) customizeKineMySQLStorage(ctx context.Context, podTemplate *corev1.PodTemplateSpec, tenantControlPlane kamajiv1alpha1.TenantControlPlane) {
	volume := corev1.Volume{
		Name: "mysql-config",
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName:  tenantControlPlane.Status.Storage.KineMySQL.Certificate.SecretName,
				DefaultMode: pointer.Int32Ptr(420),
			},
		},
	}

	podTemplate.Spec.Volumes = append(podTemplate.Spec.Volumes, volume)

	container := corev1.Container{
		Name: "kine",
		// TODO: parameter.
		Image: fmt.Sprintf("%s:%s", "rancher/kine", "v0.9.2-amd64"),
		Args: []string{
			"--endpoint=mysql://$(MYSQL_USER):$(MYSQL_PASSWORD)@tcp($(MYSQL_HOST):$(MYSQL_PORT))/$(MYSQL_SCHEMA)",
			"--ca-file=/kine/ca.crt",
			"--cert-file=/kine/server.crt",
			"--key-file=/kine/server.key",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volume.Name,
				MountPath: "/kine",
				ReadOnly:  true,
			},
		},
		Env: []corev1.EnvVar{
			{Name: "GODEBUG", Value: "x509ignoreCN=0"},
		},
		EnvFrom: []corev1.EnvFromSource{
			{
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: tenantControlPlane.Status.Storage.KineMySQL.Config.SecretName,
					},
				},
			},
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 2379,
				Name:          "server",
				Protocol:      corev1.ProtocolTCP,
			},
		},
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: "File",
		ImagePullPolicy:          corev1.PullAlways,
	}

	podTemplate.Spec.Containers = append(podTemplate.Spec.Containers, container)
}

func (r *KubernetesDeploymentResource) syncKubeApiServer(tenantControlPlane *kamajiv1alpha1.TenantControlPlane, address string) {
	r.resource.Spec.Template.Spec.Containers[0].Name = "kube-apiserver"
	r.resource.Spec.Template.Spec.Containers[0].Image = fmt.Sprintf("k8s.gcr.io/kube-apiserver:%s", tenantControlPlane.Spec.Kubernetes.Version)
	r.resource.Spec.Template.Spec.Containers[0].Command = []string{
		"kube-apiserver",
		"--allow-privileged=true",
		"--authorization-mode=Node,RBAC",
		fmt.Sprintf("--advertise-address=%s", address),
		fmt.Sprintf("--client-ca-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName)),
		fmt.Sprintf("--enable-admission-plugins=%s", strings.Join(tenantControlPlane.Spec.Kubernetes.AdmissionControllers.ToSlice(), ",")),
		"--enable-bootstrap-token-auth=true",
		fmt.Sprintf("--etcd-servers=%s", strings.Join(r.ETCDEndpoints, ",")),
		fmt.Sprintf("--service-cluster-ip-range=%s", tenantControlPlane.Spec.NetworkProfile.ServiceCIDR),
		fmt.Sprintf("--kubelet-client-certificate=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerKubeletClientCertName)),
		fmt.Sprintf("--kubelet-client-key=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerKubeletClientKeyName)),
		"--kubelet-preferred-address-types=Hostname,InternalIP,ExternalIP",
		fmt.Sprintf("--proxy-client-cert-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.FrontProxyClientCertName)),
		fmt.Sprintf("--proxy-client-key-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.FrontProxyClientKeyName)),
		"--requestheader-allowed-names=front-proxy-client",
		"--requestheader-extra-headers-prefix=X-Remote-Extra-",
		"--requestheader-group-headers=X-Remote-Group",
		"--requestheader-username-headers=X-Remote-User",
		fmt.Sprintf("--secure-port=%d", tenantControlPlane.Spec.NetworkProfile.Port),
		fmt.Sprintf("--service-account-issuer=https://localhost:%d", tenantControlPlane.Spec.NetworkProfile.Port),
		fmt.Sprintf("--service-account-key-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.ServiceAccountPublicKeyName)),
		fmt.Sprintf("--service-account-signing-key-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.ServiceAccountPrivateKeyName)),
		fmt.Sprintf("--tls-cert-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerCertName)),
		fmt.Sprintf("--tls-private-key-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerKeyName)),
	}
	r.resource.Spec.Template.Spec.Containers[0].LivenessProbe = &corev1.Probe{
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
	r.resource.Spec.Template.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
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
	r.resource.Spec.Template.Spec.Containers[0].StartupProbe = &corev1.Probe{
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
	r.resource.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
	r.resource.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
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
}

func (r *KubernetesDeploymentResource) syncScheduler(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) {
	r.resource.Spec.Template.Spec.Containers[1].Name = "kube-scheduler"
	r.resource.Spec.Template.Spec.Containers[1].Image = fmt.Sprintf("k8s.gcr.io/kube-scheduler:%s", tenantControlPlane.Spec.Kubernetes.Version)
	r.resource.Spec.Template.Spec.Containers[1].Command = []string{
		"kube-scheduler",
		"--authentication-kubeconfig=/etc/kubernetes/scheduler.conf",
		"--authorization-kubeconfig=/etc/kubernetes/scheduler.conf",
		"--bind-address=0.0.0.0",
		"--kubeconfig=/etc/kubernetes/scheduler.conf",
		"--leader-elect=true",
	}
	r.resource.Spec.Template.Spec.Containers[1].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "scheduler-kubeconfig",
			ReadOnly:  true,
			MountPath: "/etc/kubernetes",
		},
	}
	r.resource.Spec.Template.Spec.Containers[1].LivenessProbe = &corev1.Probe{
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
	r.resource.Spec.Template.Spec.Containers[1].StartupProbe = &corev1.Probe{
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
	r.resource.Spec.Template.Spec.Containers[1].ImagePullPolicy = corev1.PullAlways
}

func (r *KubernetesDeploymentResource) syncControllerManager(tenantControlPlane *kamajiv1alpha1.TenantControlPlane) {
	r.resource.Spec.Template.Spec.Containers[2].Name = "kube-controller-manager"
	r.resource.Spec.Template.Spec.Containers[2].Image = fmt.Sprintf("k8s.gcr.io/kube-controller-manager:%s", tenantControlPlane.Spec.Kubernetes.Version)
	r.resource.Spec.Template.Spec.Containers[2].Command = []string{
		"kube-controller-manager",
		"--allocate-node-cidrs=true",
		"--authentication-kubeconfig=/etc/kubernetes/controller-manager.conf",
		"--authorization-kubeconfig=/etc/kubernetes/controller-manager.conf",
		"--bind-address=0.0.0.0",
		fmt.Sprintf("--client-ca-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName)),
		fmt.Sprintf("--cluster-name=%s", tenantControlPlane.GetName()),
		fmt.Sprintf("--cluster-signing-cert-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName)),
		fmt.Sprintf("--cluster-signing-key-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.CAKeyName)),
		"--controllers=*,bootstrapsigner,tokencleaner",
		"--kubeconfig=/etc/kubernetes/controller-manager.conf",
		"--leader-elect=true",
		fmt.Sprintf("--service-cluster-ip-range=%s", tenantControlPlane.Spec.NetworkProfile.ServiceCIDR),
		fmt.Sprintf("--cluster-cidr=%s", tenantControlPlane.Spec.NetworkProfile.PodCIDR),
		fmt.Sprintf("--requestheader-client-ca-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.FrontProxyCACertName)),
		fmt.Sprintf("--root-ca-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName)),
		fmt.Sprintf("--service-account-private-key-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.ServiceAccountPrivateKeyName)),
		"--use-service-account-credentials=true",
	}
	r.resource.Spec.Template.Spec.Containers[2].VolumeMounts = []corev1.VolumeMount{
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
	r.resource.Spec.Template.Spec.Containers[2].LivenessProbe = &corev1.Probe{
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
	r.resource.Spec.Template.Spec.Containers[2].StartupProbe = &corev1.Probe{
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
	r.resource.Spec.Template.Spec.Containers[2].ImagePullPolicy = corev1.PullAlways
}
