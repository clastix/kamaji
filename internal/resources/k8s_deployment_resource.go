// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package resources

import (
	"context"
	"crypto/md5"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	quantity "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta3"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubernetesDeploymentResource struct {
	resource               *appsv1.Deployment
	Client                 client.Client
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
	return false, nil
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

// secretHashValue function returns the md5 value for the given secret:
// this will trigger a new rollout in case of value change.
func (r *KubernetesDeploymentResource) secretHashValue(ctx context.Context, namespace, name string) (string, error) {
	secret := &corev1.Secret{}

	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret); err != nil {
		return "", errors.Wrap(err, "cannot retrieve *corev1.Secret for resource version retrieval")
	}
	// Go access map values in random way, it means we have to sort them
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

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (r *KubernetesDeploymentResource) CreateOrUpdate(ctx context.Context, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (controllerutil.OperationResult, error) {
	maxSurge := intstr.FromString("100%")

	maxUnavailable := intstr.FromInt(0)

	etcdEndpoints := make([]string, len(r.ETCDEndpoints))
	for i, v := range r.ETCDEndpoints {
		etcdEndpoints[i] = fmt.Sprintf("https://%s", v)
	}

	address, err := tenantControlPlane.GetAddress(ctx, r.Client)
	if err != nil {
		return controllerutil.OperationResultNone, errors.Wrap(err, "cannot create TenantControlPlane Deployment")
	}

	return controllerutil.CreateOrUpdate(ctx, r.Client, r.resource, func() error {
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
					hash, _ = r.secretHashValue(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.APIServer.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/api-server-kubelet-client-certificate": func() (hash string) {
					hash, _ = r.secretHashValue(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.APIServerKubeletClient.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/ca": func() (hash string) {
					hash, _ = r.secretHashValue(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.CA.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/controller-manager-kubeconfig": func() (hash string) {
					hash, _ = r.secretHashValue(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.KubeConfig.ControllerManager.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/etcd-ca-certificates": func() (hash string) {
					hash, _ = r.secretHashValue(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.ETCD.CA.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/etcd-certificates": func() (hash string) {
					hash, _ = r.secretHashValue(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.ETCD.APIServer.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/front-proxy-ca-certificate": func() (hash string) {
					hash, _ = r.secretHashValue(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.FrontProxyCA.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/front-proxy-client-certificate": func() (hash string) {
					hash, _ = r.secretHashValue(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.FrontProxyClient.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/service-account": func() (hash string) {
					hash, _ = r.secretHashValue(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.Certificates.SA.SecretName)

					return
				}(),
				"component.kamaji.clastix.io/scheduler-kubeconfig": func() (hash string) {
					hash, _ = r.secretHashValue(ctx, tenantControlPlane.GetNamespace(), tenantControlPlane.Status.KubeConfig.Scheduler.SecretName)

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

		r.resource.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:  "kube-apiserver",
				Image: fmt.Sprintf("k8s.gcr.io/kube-apiserver:%s", tenantControlPlane.Spec.Kubernetes.Version),
				Command: []string{
					"kube-apiserver",
					"--allow-privileged=true",
					"--authorization-mode=Node,RBAC",
					fmt.Sprintf("--advertise-address=%s", address),
					fmt.Sprintf("--client-ca-file=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.CACertName)),
					fmt.Sprintf("--enable-admission-plugins=%s", strings.Join(tenantControlPlane.Spec.Kubernetes.AdmissionControllers.ToSlice(), ",")),
					"--enable-bootstrap-token-auth=true",
					fmt.Sprintf("--etcd-compaction-interval=%s", r.ETCDCompactionInterval),
					fmt.Sprintf("--etcd-cafile=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.EtcdCACertName)),
					fmt.Sprintf("--etcd-certfile=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerEtcdClientCertName)),
					fmt.Sprintf("--etcd-keyfile=%s", path.Join(v1beta3.DefaultCertificatesDir, constants.APIServerEtcdClientKeyName)),
					fmt.Sprintf("--etcd-servers=%s", strings.Join(etcdEndpoints, ",")),
					fmt.Sprintf("--etcd-prefix=/%s", tenantControlPlane.GetName()),
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
				},
				Resources: corev1.ResourceRequirements{
					Limits: nil,
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: quantity.MustParse("250m"),
					},
				},
				LivenessProbe: &corev1.Probe{
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
				},
				ReadinessProbe: &corev1.Probe{
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
				},
				StartupProbe: &corev1.Probe{
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
				},
				TerminationMessagePath:   "/dev/termination-log",
				TerminationMessagePolicy: "File",
				ImagePullPolicy:          corev1.PullAlways,
				VolumeMounts: []corev1.VolumeMount{
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
				},
			},
			{
				Name:  "kube-scheduler",
				Image: fmt.Sprintf("k8s.gcr.io/kube-scheduler:%s", tenantControlPlane.Spec.Kubernetes.Version),
				Command: []string{
					"kube-scheduler",
					"--authentication-kubeconfig=/etc/kubernetes/scheduler.conf",
					"--authorization-kubeconfig=/etc/kubernetes/scheduler.conf",
					"--bind-address=0.0.0.0",
					"--kubeconfig=/etc/kubernetes/scheduler.conf",
					"--leader-elect=true",
				},
				Resources: corev1.ResourceRequirements{
					Limits: nil,
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: quantity.MustParse("100m"),
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "scheduler-kubeconfig",
						ReadOnly:  true,
						MountPath: "/etc/kubernetes",
					},
				},
				LivenessProbe: &corev1.Probe{
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
				},
				StartupProbe: &corev1.Probe{
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
				},
				TerminationMessagePath:   "/dev/termination-log",
				TerminationMessagePolicy: "File",
				ImagePullPolicy:          corev1.PullIfNotPresent,
			},
			{
				Name:  "kube-controller-manager",
				Image: fmt.Sprintf("k8s.gcr.io/kube-controller-manager:%s", tenantControlPlane.Spec.Kubernetes.Version),
				Command: []string{
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
				},
				Resources: corev1.ResourceRequirements{
					Limits: nil,
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: quantity.MustParse("200m"),
					},
				},
				VolumeMounts: []corev1.VolumeMount{
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
				},
				LivenessProbe: &corev1.Probe{
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
				},
				StartupProbe: &corev1.Probe{
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
				},
				TerminationMessagePath:   "/dev/termination-log",
				TerminationMessagePolicy: "File",
				ImagePullPolicy:          corev1.PullIfNotPresent,
			},
		}
		r.resource.Spec.Strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appsv1.RollingUpdateDeployment{
				MaxUnavailable: &maxUnavailable,
				MaxSurge:       &maxSurge,
			},
		}

		return controllerutil.SetControllerReference(tenantControlPlane, r.resource, r.Client.Scheme())
	})
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
