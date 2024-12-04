// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

type RotateCertOptions map[string]string

func (opts RotateCertOptions) Keys() []string {
	keys := make([]string, 0, len(opts))

	for k := range opts {
		keys = append(keys, k)
	}

	return keys
}

var RotateCertificatesMap = RotateCertOptions{
	"APIServer":              "api-server-certificate",
	"APIServerKubeletClient": "api-server-kubelet-client-certificate",
	"FrontProxyCA":           "front-proxy-ca-certificate",
	"FrontProxyClient":       "front-proxy-client-certificate",
	"Konnectivity":           "konnectivity-certificate",
}

// RotateCertificate performs the rotation of Tenant Control Plane certificates.
// Despite rotation is currently managed by the CertificateLifeCycle controller,
// this action could be used to generate a new pair of ones.
// When rotating the API Server one, a reload of the Control Plane pods will be triggered.
func (h *Helper) RotateCertificate(ctx context.Context, namespace, name string, certificates RotateCertOptions) error {
	for cert, suffix := range certificates {
		var secret corev1.Secret
		secret.Name = fmt.Sprintf("%s-%s", name, suffix)
		secret.Namespace = namespace

		if err := h.Client.Delete(ctx, &secret); err != nil {
			if k8serrors.IsNotFound(err) {
				continue
			}

			return fmt.Errorf("unable to rotate %s certificate: %w", cert, err)
		}
	}

	return nil
}
