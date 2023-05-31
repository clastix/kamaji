// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"fmt"
)

func (r *RegistrySettings) buildContainerImage(name, tag string) string {
	image := fmt.Sprintf("%s/%s:%s", r.Registry, name, tag)

	if len(r.TagSuffix) > 0 {
		image += r.TagSuffix
	}

	return image
}

func (r *RegistrySettings) KubeAPIServerImage(version string) string {
	return r.buildContainerImage(r.APIServerImage, version)
}

func (r *RegistrySettings) KubeSchedulerImage(version string) string {
	return r.buildContainerImage(r.SchedulerImage, version)
}

func (r *RegistrySettings) KubeControllerManagerImage(version string) string {
	return r.buildContainerImage(r.ControllerManagerImage, version)
}
