// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/bootstraptoken/clusterinfo"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/bootstraptoken/node"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/apiclient"
)

func BootstrapToken(client kubernetes.Interface, config *Configuration) error {
	initConfiguration := config.InitConfiguration

	if err := node.UpdateOrCreateTokens(client, false, initConfiguration.BootstrapTokens); err != nil {
		return errors.Wrap(err, "error updating or creating token")
	}

	if err := node.AllowBootstrapTokensToGetNodes(client); err != nil {
		return errors.Wrap(err, "error allowing bootstrap tokens to get Nodes")
	}

	if err := node.AllowBootstrapTokensToPostCSRs(client); err != nil {
		return errors.Wrap(err, "error allowing bootstrap tokens to post CSRs")
	}

	if err := node.AutoApproveNodeBootstrapTokens(client); err != nil {
		return errors.Wrap(err, "error auto-approving node bootstrap tokens")
	}

	if err := node.AutoApproveNodeCertificateRotation(client); err != nil {
		return err
	}

	bootstrapConfig := &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"": {
				Server:                   config.Kubeconfig.Clusters[0].Cluster.Server,
				CertificateAuthorityData: config.Kubeconfig.Clusters[0].Cluster.CertificateAuthorityData,
			},
		},
	}
	bootstrapBytes, err := clientcmd.Write(*bootstrapConfig)
	if err != nil {
		return err
	}

	err = apiclient.CreateOrUpdate[*corev1.ConfigMap](client.CoreV1().ConfigMaps(metav1.NamespacePublic), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstrapapi.ConfigMapClusterInfo,
			Namespace: metav1.NamespacePublic,
		},
		Data: map[string]string{
			bootstrapapi.KubeConfigKey: string(bootstrapBytes),
		},
	})
	if err != nil {
		return err
	}

	if err := clusterinfo.CreateClusterInfoRBACRules(client); err != nil {
		return errors.Wrap(err, "error creating clusterinfo RBAC rules")
	}

	return nil
}
