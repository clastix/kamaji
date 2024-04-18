// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"fmt"
	"runtime"

	"github.com/pkg/errors"
	versionutil "k8s.io/apimachinery/pkg/util/version"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/upgrade"
)

type kamajiKubeVersionGetter struct {
	upgrade.VersionGetter
}

func NewKamajiKubeVersionGetter(restClient kubernetes.Interface) upgrade.VersionGetter {
	kubeVersionGetter := upgrade.NewOfflineVersionGetter(upgrade.NewKubeVersionGetter(restClient), KubeadmVersion)

	return &kamajiKubeVersionGetter{VersionGetter: kubeVersionGetter}
}

func (k kamajiKubeVersionGetter) ClusterVersion() (string, *versionutil.Version, error) {
	return k.VersionGetter.ClusterVersion()
}

func (k kamajiKubeVersionGetter) KubeadmVersion() (string, *versionutil.Version, error) {
	kubeadmVersionInfo := apimachineryversion.Info{
		GitVersion: KubeadmVersion,
		GoVersion:  runtime.Version(),
		Compiler:   runtime.Compiler,
		Platform:   fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	kubeadmVersion, err := versionutil.ParseSemantic(kubeadmVersionInfo.String())
	if err != nil {
		return "", nil, errors.Wrap(err, "Couldn't parse kubeadm version")
	}

	return kubeadmVersionInfo.String(), kubeadmVersion, nil
}

func (k kamajiKubeVersionGetter) VersionFromCILabel(ciVersionLabel, description string) (string, *versionutil.Version, error) {
	return k.VersionGetter.VersionFromCILabel(ciVersionLabel, description)
}

func (k kamajiKubeVersionGetter) KubeletVersions() (map[string][]string, error) {
	return k.VersionGetter.KubeletVersions()
}
