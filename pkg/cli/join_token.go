// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/pubkeypin"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/clastix/kamaji/api/v1alpha1"
)

func (h *Helper) joinTokenKubeadmFlavour(url, token, secret, sha string) string {
	return fmt.Sprintf("kubeadm join %s --token %s.%s --discovery-token-ca-cert-hash %s", url, token, secret, sha)
}

func (h *Helper) joinTokenYakiFlavour(k8sVersion, url, token, secret, sha string) string {
	return fmt.Sprintf("wget -O- https://goyaki.clastix.io | sudo KUBERNETES_VERSION=%s JOIN_URL=%s JOIN_TOKEN=%s.%s JOIN_TOKEN_CACERT_HASH=%s bash -s join", k8sVersion, url, token, secret, sha)
}

// JoinToken returns the command to let a worker node join a given Tenant Control Plane.
// The variable flavour allows specifying which bootstrap tool to format to:
// currently supported yaki, and kubeadm.
func (h *Helper) JoinToken(ctx context.Context, namespace, name string, skipExpired bool, flavour string) (string, error) {
	kubeconfig, data, kcErr := h.GetKubeconfig(ctx, namespace, name, "admin.conf")
	if kcErr != nil {
		return "", fmt.Errorf("cannot retrieve kubeconfig for address retrieval, %w", kcErr)
	}

	var clusterAddress string
	var clusterCACertificate []byte

	for _, cluster := range kubeconfig.Clusters {
		clusterAddress = cluster.Server
		clusterCACertificate = cluster.CertificateAuthorityData

		break
	}

	if clusterAddress == "" {
		return "", errors.New("cannot retrieve server address, the provided kubeconfig seems empty")
	}

	u, uErr := url.Parse(clusterAddress)
	if uErr != nil {
		return "", fmt.Errorf("cannot parse cluster address, %w", uErr)
	}

	clusterAddress = fmt.Sprintf("%s:%s", u.Hostname(), u.Port())

	config, cErr := clientcmd.RESTConfigFromKubeConfig(data)
	if cErr != nil {
		return "", fmt.Errorf("cannot create rest client config, %w", cErr)
	}

	config.Timeout = 10 * time.Second
	tntClient, clientErr := client.New(config, client.Options{})
	if clientErr != nil {
		return "", fmt.Errorf("cannot create Tenant Control Plane client, %w", clientErr)
	}

	var tcp v1alpha1.TenantControlPlane
	if err := h.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &tcp); err != nil {
		if k8serrors.IsNotFound(err) {
			return "", fmt.Errorf("instance %s/%s does not exist", namespace, name)
		}

		return "", fmt.Errorf("cannot retrieve instance, %w", err)
	}

	if tcp.Status.KubeadmPhase.BootstrapToken.Checksum == "" {
		return "", fmt.Errorf("bootstrap for %s/%s has not yet completed", namespace, name)
	}

	var tokenCACertHash string

	pemContent, _ := pem.Decode(clusterCACertificate)
	if crt, err := x509.ParseCertificate(pemContent.Bytes); crt != nil && err == nil {
		tokenCACertHash = pubkeypin.Hash(crt)
	}

	if tokenCACertHash == "" {
		return "", errors.New("cannot parse certificate from Certificate Authority for hash generation")
	}

	var kubeadmSecret corev1.SecretList
	if err := tntClient.List(ctx, &kubeadmSecret, client.InNamespace("kube-system")); err != nil {
		return "", fmt.Errorf("cannot retrieve bootstrap tokens due to an error, %w", err)
	}

	var token *corev1.Secret
	for _, secret := range kubeadmSecret.Items {
		if secret.Type != "bootstrap.kubernetes.io/token" {
			continue
		}

		if !skipExpired {
			if t, tErr := time.Parse("2006-01-02T15:04:05Z", string(secret.Data["expiration"])); tErr != nil || t.Before(time.Now()) {
				continue
			}
		}

		token = ptr.To(secret)

		break
	}

	if token == nil {
		return "", fmt.Errorf("no available bootstrap.kubernetes.io/token Secret ojects")
	}

	tokenID := string(token.Data["token-id"])
	tokenSecret := string(token.Data["token-secret"])

	switch flavour {
	case "yaki":
		return h.joinTokenYakiFlavour(tcp.Status.Kubernetes.Version.Version, clusterAddress, tokenID, tokenSecret, tokenCACertHash), nil
	case "kubeadm":
		return h.joinTokenKubeadmFlavour(clusterAddress, tokenID, tokenSecret, tokenCACertHash), nil
	default:
		return "", fmt.Errorf("unknown flavour %q, currently supported %q, %q", flavour, "yaki", "kubeadm")
	}
}
