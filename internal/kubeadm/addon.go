// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"context"
	"io"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/dns"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/addons/proxy"
)

const (
	kubeSystemNamespace = "kube-system"
	kubeProxyName       = "kube-proxy"
	coreDNSName         = "coredns"
	kubeDNSName         = "kube-dns"
)

func AddCoreDNS(client kubernetes.Interface, config *Configuration) error {
	// We're passing the values from the parameters here because they wouldn't be hashed by the YAML encoder:
	// the struct kubeadm.ClusterConfiguration hasn't struct tags, and it wouldn't be hashed properly.
	if opts := config.Parameters.CoreDNSOptions; opts != nil {
		config.InitConfiguration.DNS.ImageRepository = opts.Repository
		config.InitConfiguration.DNS.ImageTag = opts.Tag
	}

	return dns.EnsureDNSAddon(&config.InitConfiguration.ClusterConfiguration, client, io.Discard, false)
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

func AddKubeProxy(client kubernetes.Interface, config *Configuration) (err error) {
	// This is a workaround since the function EnsureProxyAddon is picking repository and tag from the InitConfiguration
	// struct, although is counterintuitive
	config.InitConfiguration.ClusterConfiguration.CIImageRepository = config.Parameters.KubeProxyOptions.Repository
	config.InitConfiguration.KubernetesVersion = config.Parameters.KubeProxyOptions.Tag

	err = proxy.EnsureProxyAddon(&config.InitConfiguration.ClusterConfiguration, &config.InitConfiguration.LocalAPIEndpoint, client, io.Discard, false)

	return
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
