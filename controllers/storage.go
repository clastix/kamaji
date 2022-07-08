// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/clastix/kamaji/internal/sql"
	"github.com/clastix/kamaji/internal/types"
)

func (r *TenantControlPlaneReconciler) getStorageConnection(ctx context.Context) (sql.DBConnection, error) {
	// TODO: https://github.com/clastix/kamaji/issues/67
	switch r.Config.ETCDStorageType {
	case types.KineMySQL:
		secret := &corev1.Secret{}
		namespacedName := k8stypes.NamespacedName{Namespace: r.Config.KineMySQLSecretNamespace, Name: r.Config.KineMySQLSecretName}
		if err := r.Client.Get(ctx, namespacedName, secret); err != nil {
			return nil, err
		}

		rootCAs := x509.NewCertPool()
		if ok := rootCAs.AppendCertsFromPEM(secret.Data["ca.crt"]); !ok {
			return nil, fmt.Errorf("error creating root ca for mysql db connector")
		}

		certificate, err := tls.X509KeyPair(secret.Data["server.crt"], secret.Data["server.key"])
		if err != nil {
			return nil, err
		}

		return sql.GetDBConnection(
			sql.ConnectionConfig{
				SQLDriver: sql.MySQL,
				User:      "root",
				Password:  string(secret.Data["MYSQL_ROOT_PASSWORD"]),
				Host:      r.Config.KineMySQLHost,
				Port:      r.Config.KineMySQLPort,
				DBName:    "mysql",
				TLSConfig: &tls.Config{
					ServerName:   r.Config.KineMySQLHost,
					RootCAs:      rootCAs,
					Certificates: []tls.Certificate{certificate},
				},
			},
		)
	default:
		return nil, nil
	}
}
