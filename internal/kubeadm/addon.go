// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/component-base/config/v1alpha1"
	kubeproxyconfig "k8s.io/kube-proxy/config/v1alpha1"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/dns"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/proxy"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
	"k8s.io/utils/pointer"

	"github.com/clastix/kamaji/internal/utilities"
)

const (
	kubeSystemNamespace = "kube-system"
	kubeProxyName       = "kube-proxy"
	coreDNSName         = "coredns"
	kubeDNSName         = "kube-dns"
)

func AddCoreDNS(client kubernetes.Interface, config *Configuration) error {
	return dns.EnsureDNSAddon(&config.InitConfiguration.ClusterConfiguration, client)
}

func RemoveCoreDNSAddon(ctx context.Context, client kubernetes.Interface) error {
	var result error

	if err := removeCoreDNSService(ctx, client); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		result = err
	}

	if err := removeCoreDNSDeployment(ctx, client); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		result = err
	}

	if err := removeCoreDNSConfigMap(ctx, client); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		result = err
	}

	return result
}

func removeCoreDNSService(ctx context.Context, client kubernetes.Interface) error {
	name, _ := getCoreDNSServiceName(ctx)
	opts := metav1.DeleteOptions{}

	return client.CoreV1().Services(kubeSystemNamespace).Delete(ctx, name, opts)
}

func removeCoreDNSDeployment(ctx context.Context, client kubernetes.Interface) error {
	name, _ := getCoreDNSDeploymentName(ctx)
	opts := metav1.DeleteOptions{}

	return client.AppsV1().Deployments(kubeSystemNamespace).Delete(ctx, name, opts)
}

func removeCoreDNSConfigMap(ctx context.Context, client kubernetes.Interface) error {
	name, _ := getCoreDNSConfigMapName(ctx)

	opts := metav1.DeleteOptions{}

	return client.CoreV1().ConfigMaps(kubeSystemNamespace).Delete(ctx, name, opts)
}

func getCoreDNSServiceName(ctx context.Context) (string, error) {
	// TODO: Currently, DNS is installed using kubeadm phases, therefore we know the name.
	// Implement a method for future approaches
	return kubeDNSName, nil
}

func getCoreDNSDeploymentName(ctx context.Context) (string, error) {
	// TODO: Currently, DNS is installed using kubeadm phases, therefore we know the name.
	// Implement a method for future approaches
	return coreDNSName, nil
}

func getCoreDNSConfigMapName(ctx context.Context) (string, error) {
	// TODO: Currently, DNS is installed using kubeadm phases, therefore we know the name.
	// Implement a method for future approaches
	return coreDNSName, nil
}

func AddKubeProxy(client kubernetes.Interface, config *Configuration) error {
	if err := proxy.CreateServiceAccount(client); err != nil {
		return errors.Wrap(err, "error when creating kube-proxy service account")
	}

	if err := createKubeProxyConfigMap(client, config); err != nil {
		return err
	}

	if err := createKubeProxyAddon(client); err != nil {
		return err
	}

	if err := proxy.CreateRBACRules(client); err != nil {
		return errors.Wrap(err, "error when creating kube-proxy RBAC rules")
	}

	return nil
}

func RemoveKubeProxy(ctx context.Context, client kubernetes.Interface) error {
	var result error

	if err := removeKubeProxyDaemonSet(ctx, client); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		result = err
	}

	if err := removeKubeProxyConfigMap(ctx, client); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		result = err
	}

	if err := removeKubeProxyRBAC(ctx, client); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		result = err
	}

	return result
}

func removeKubeProxyDaemonSet(ctx context.Context, client kubernetes.Interface) error {
	name, _ := getKubeProxyDaemonSetName(ctx)

	opts := metav1.DeleteOptions{}

	return client.AppsV1().DaemonSets(kubeSystemNamespace).Delete(ctx, name, opts)
}

func removeKubeProxyConfigMap(ctx context.Context, client kubernetes.Interface) error {
	name, _ := getKubeProxyConfigMapName(ctx)

	opts := metav1.DeleteOptions{}

	return client.CoreV1().ConfigMaps(kubeSystemNamespace).Delete(ctx, name, opts)
}

func removeKubeProxyRBAC(ctx context.Context, client kubernetes.Interface) error {
	// TODO: Currently, kube-proxy is installed using kubeadm phases, therefore, name is the same.
	name, _ := getKubeProxyRBACName(ctx)

	opts := metav1.DeleteOptions{}
	var result error

	if err := client.RbacV1().RoleBindings(kubeSystemNamespace).Delete(ctx, name, opts); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		result = err
	}

	if err := client.RbacV1().Roles(kubeSystemNamespace).Delete(ctx, name, opts); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		result = err
	}

	if err := client.CoreV1().ServiceAccounts(kubeSystemNamespace).Delete(ctx, name, opts); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		result = err
	}

	return result
}

func getKubeProxyRBACName(ctx context.Context) (string, error) {
	// TODO: Currently, kube-proxy is installed using kubeadm phases, therefore we know the name.
	// Implement a method for future approaches
	return kubeProxyName, nil
}

func getKubeProxyDaemonSetName(ctx context.Context) (string, error) {
	// TODO: Currently, kube-proxy is installed using kubeadm phases, therefore we know the name.
	// Implement a method for future approaches
	return kubeProxyName, nil
}

func getKubeProxyConfigMapName(ctx context.Context) (string, error) {
	// TODO: Currently, kube-proxy is installed using kubeadm phases, therefore we know the name.
	// Implement a method for future approaches
	return kubeProxyName, nil
}

func createKubeProxyConfigMap(client kubernetes.Interface, config *Configuration) error {
	configConf, err := getKubeproxyConfigmapContent(config)
	if err != nil {
		return err
	}

	kubeconfigConf, err := getKubeproxyKubeconfigContent(config)
	if err != nil {
		return err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeadmconstants.KubeProxyConfigMap,
			Namespace: "kube-system",
			Labels: map[string]string{
				"app": "kube-proxy",
			},
		},
		Data: map[string]string{
			kubeadmconstants.KubeProxyConfigMapKey: string(configConf),
			"kubeconfig.conf":                      string(kubeconfigConf),
		},
	}

	return apiclient.CreateOrUpdateConfigMap(client, configMap)
}

func createKubeProxyAddon(client kubernetes.Interface) error {
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-proxy",
			Namespace: "kube-system",
			Labels: map[string]string{
				"k8s-app": "kube-proxy",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			RevisionHistoryLimit: pointer.Int32(10),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "kube-proxy",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app": "kube-proxy",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Command: []string{
								"/usr/local/bin/kube-proxy",
								"--config=/var/lib/kube-proxy/config.conf",
								"--hostname-override=$(NODE_NAME)",
							},
							Env: []corev1.EnvVar{
								{
									Name: "NODE_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "spec.nodeName",
										},
									},
								},
							},
							Image:           "k8s.gcr.io/kube-proxy:v1.21.2",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Name:            "kube-proxy",
							SecurityContext: &corev1.SecurityContext{
								Privileged: pointer.Bool(true),
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: "File",
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/var/lib/kube-proxy",
									Name:      "kube-proxy",
								},
								{
									MountPath: "/run/xtables.lock",
									Name:      "xtables-lock",
								},
								{
									MountPath: "/lib/modules",
									Name:      "lib-modules",
									ReadOnly:  true,
								},
							},
						},
					},
					DNSPolicy:   corev1.DNSClusterFirst,
					HostNetwork: true,
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
					Tolerations: []corev1.Toleration{
						{Operator: corev1.TolerationOpExists},
					},
					PriorityClassName:             "system-node-critical",
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 "default-scheduler",
					ServiceAccountName:            "kube-proxy",
					TerminationGracePeriodSeconds: pointer.Int64(30),
					Volumes: []corev1.Volume{
						{
							Name: "kube-proxy",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									DefaultMode: pointer.Int32(420),
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kube-proxy",
									},
								},
							},
						},
						{
							Name: "xtables-lock",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/xtables.lock",
									Type: (*corev1.HostPathType)(pointer.String(string(corev1.HostPathFileOrCreate))),
								},
							},
						},
						{
							Name: "lib-modules",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/lib/modules",
									Type: (*corev1.HostPathType)(pointer.String(string(corev1.HostPathUnset))),
								},
							},
						},
					},
				},
			},
		},
	}

	return apiclient.CreateOrUpdateDaemonSet(client, daemonSet)
}

func getKubeproxyConfigmapContent(config *Configuration) ([]byte, error) {
	zeroDuration := metav1.Duration{Duration: 0}
	oneSecondDuration := metav1.Duration{Duration: time.Second}

	kubeProxyConfiguration := kubeproxyconfig.KubeProxyConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeProxyConfiguration",
			APIVersion: "kubeproxy.config.k8s.io/v1alpha1",
		},
		BindAddress:         "0.0.0.0",
		BindAddressHardFail: false,
		ClientConnection: v1alpha1.ClientConnectionConfiguration{
			AcceptContentTypes: "",
			Burst:              0,
			ContentType:        "",
			Kubeconfig:         "/var/lib/kube-proxy/kubeconfig.conf",
			QPS:                0,
		},
		ClusterCIDR:      config.Parameters.TenantControlPlanePodCIDR,
		ConfigSyncPeriod: zeroDuration,
		Conntrack: kubeproxyconfig.KubeProxyConntrackConfiguration{
			MaxPerCore:            pointer.Int32(0),
			Min:                   nil,
			TCPCloseWaitTimeout:   nil,
			TCPEstablishedTimeout: nil,
		},
		DetectLocalMode:    "",
		EnableProfiling:    false,
		HealthzBindAddress: "",
		HostnameOverride:   "",
		IPTables: kubeproxyconfig.KubeProxyIPTablesConfiguration{
			MasqueradeAll: false,
			MasqueradeBit: nil,
			MinSyncPeriod: oneSecondDuration,
			SyncPeriod:    zeroDuration,
		},
		IPVS: kubeproxyconfig.KubeProxyIPVSConfiguration{
			ExcludeCIDRs:  nil,
			MinSyncPeriod: zeroDuration,
			Scheduler:     "",
			StrictARP:     false,
			SyncPeriod:    zeroDuration,
			TCPTimeout:    zeroDuration,
			TCPFinTimeout: zeroDuration,
			UDPTimeout:    zeroDuration,
		},
		MetricsBindAddress:          "",
		Mode:                        "iptables",
		NodePortAddresses:           nil,
		OOMScoreAdj:                 nil,
		PortRange:                   "",
		ShowHiddenMetricsForVersion: "",
		UDPIdleTimeout:              zeroDuration,
		Winkernel: kubeproxyconfig.KubeProxyWinkernelConfiguration{
			EnableDSR:   false,
			NetworkName: "",
			SourceVip:   "",
		},
	}

	return utilities.EncondeToYaml(&kubeProxyConfiguration)
}

func getKubeproxyKubeconfigContent(config *Configuration) ([]byte, error) {
	kubeconfig := clientcmdapi.Config{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: []clientcmdapi.NamedCluster{
			{
				Name: "default",
				Cluster: clientcmdapi.Cluster{
					CertificateAuthority: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
					Server:               fmt.Sprintf("https://%s:%d", config.Parameters.TenantControlPlaneAddress, config.Parameters.TenantControlPlanePort),
				},
			},
		},
		Contexts: []clientcmdapi.NamedContext{
			{
				Context: clientcmdapi.Context{
					Cluster:   "default",
					Namespace: "default",
					AuthInfo:  "default",
				},
			},
		},
		AuthInfos: []clientcmdapi.NamedAuthInfo{
			{
				Name: "default",
				AuthInfo: clientcmdapi.AuthInfo{
					TokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
				},
			},
		},
	}

	return utilities.EncondeToYaml(&kubeconfig)
}
