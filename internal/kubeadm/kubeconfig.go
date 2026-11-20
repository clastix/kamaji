// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package kubeadm

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"time"

	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/kubeconfig"

	"github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/utilities"
)

func buildCertificateDirectoryWithCA(ca CertificatePrivateKeyPair, directory string) error {
	if err := os.MkdirAll(directory, os.FileMode(0o755)); err != nil {
		return err
	}

	certPath := path.Join(directory, kubeadmconstants.CACertName)
	if err := os.WriteFile(certPath, ca.Certificate, os.FileMode(0o600)); err != nil {
		return err
	}

	keyPath := path.Join(directory, kubeadmconstants.CAKeyName)

	return os.WriteFile(keyPath, ca.PrivateKey, os.FileMode(0o600))
}

// CreateKubeconfig returns the kubeconfig kubeadm generates for the given file name,
// with a client certificate valid from now rather than from the start time of the process,
// as kubeconfig.CreateKubeConfigFile would do.
func CreateKubeconfig(kubeconfigName string, ca CertificatePrivateKeyPair, config *Configuration) ([]byte, error) {
	return createKubeconfig(kubeconfigName, ca, config, time.Now())
}

func createKubeconfig(kubeconfigName string, ca CertificatePrivateKeyPair, config *Configuration, issuedAt time.Time) ([]byte, error) {
	clientName, organizations, localEndpoint, err := kubeconfigIdentity(kubeconfigName)
	if err != nil {
		return nil, err
	}

	if err = buildCertificateDirectoryWithCA(ca, config.InitConfiguration.CertificatesDir); err != nil {
		return nil, err
	}

	defer deleteCertificateDirectory(config.InitConfiguration.CertificatesDir)

	initConfiguration := config.InitConfiguration
	if localEndpoint {
		// Without a control plane endpoint, kubeadm uses the local API endpoint.
		initConfiguration.ControlPlaneEndpoint = ""
	}

	validity := kubeadmconstants.CertificateValidityPeriod
	if initConfiguration.CertificateValidityPeriod != nil {
		validity = initConfiguration.CertificateValidityPeriod.Duration
	}

	var out bytes.Buffer
	if err = kubeconfig.WriteKubeConfigWithClientCert(&out, &initConfiguration, clientName, organizations, issuedAt.UTC().Add(validity)); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// kubeconfigIdentity returns the client identity kubeadm uses for the given kubeconfig file,
// and whether it connects to the local API endpoint rather than the control plane endpoint.
func kubeconfigIdentity(kubeconfigName string) (clientName string, organizations []string, localEndpoint bool, err error) {
	switch kubeconfigName {
	case kubeadmconstants.AdminKubeConfigFileName:
		return "kubernetes-admin", []string{kubeadmconstants.ClusterAdminsGroupAndClusterRoleBinding}, false, nil
	case kubeadmconstants.SuperAdminKubeConfigFileName:
		return "kubernetes-super-admin", []string{kubeadmconstants.SystemPrivilegedGroup}, false, nil
	case kubeadmconstants.ControllerManagerKubeConfigFileName:
		return kubeadmconstants.ControllerManagerUser, nil, true, nil
	case kubeadmconstants.SchedulerKubeConfigFileName:
		return kubeadmconstants.SchedulerUser, nil, true, nil
	default:
		return "", nil, false, fmt.Errorf("unsupported kubeconfig file %q", kubeconfigName)
	}
}

func IsKubeconfigCAValid(in, caCrt []byte) bool {
	kc, err := utilities.DecodeKubeconfigYAML(in)
	if err != nil {
		return false
	}

	for _, cluster := range kc.Clusters {
		if !bytes.Equal(cluster.Cluster.CertificateAuthorityData, caCrt) {
			return false
		}
	}

	return true
}

func IsKubeconfigValid(bytes []byte, expirationThreshold time.Duration) bool {
	kc, err := utilities.DecodeKubeconfigYAML(bytes)
	if err != nil {
		return false
	}

	ok, _ := crypto.IsValidCertificateKeyPairBytes(kc.AuthInfos[0].AuthInfo.ClientCertificateData, kc.AuthInfos[0].AuthInfo.ClientKeyData, expirationThreshold)

	return ok
}
