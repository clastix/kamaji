// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/clastix/kamaji/internal/sql"
	"github.com/clastix/kamaji/internal/types"
)

func (r *TenantControlPlaneReconciler) getStorageConnection(ctx context.Context) (sql.DBConnection, error) {
	var driver sql.Driver
	var dbName string

	// TODO: https://github.com/clastix/kamaji/issues/67
	switch r.Config.ETCDStorageType {
	case types.ETCD:
		return nil, nil
	case types.KineMySQL:
		driver = sql.MySQL
		dbName = "mysql"
	case types.KinePostgreSQL:
		driver = sql.PostgreSQL
	default:
		return nil, nil
	}

	secret := &corev1.Secret{}
	namespacedName := k8stypes.NamespacedName{Namespace: r.Config.KineSecretNamespace, Name: r.Config.KineSecretName}
	if err := r.Client.Get(ctx, namespacedName, secret); err != nil {
		return nil, err
	}

	if t := "kamaji.clastix.io/kine"; string(secret.Type) != t {
		return nil, fmt.Errorf("expecting a secret of type %s", t)
	}

	keys := []string{"ca.crt", "server.crt", "server.key", "username", "password"}

	if secret.Data == nil {
		return nil, fmt.Errorf("the Kine secret %s/%s is missing all the required keys (%s)", secret.GetNamespace(), secret.GetName(), strings.Join(keys, ","))
	}

	for _, key := range keys {
		if _, ok := secret.Data[key]; !ok {
			return nil, fmt.Errorf("missing required key %s for the Kine secret %s/%s", key, secret.GetNamespace(), secret.GetName())
		}
	}

	rootCAs := x509.NewCertPool()
	if ok := rootCAs.AppendCertsFromPEM(secret.Data["ca.crt"]); !ok {
		return nil, fmt.Errorf("error create root CA for the DB connector")
	}

	certificate, err := tls.X509KeyPair(secret.Data["server.crt"], secret.Data["server.key"])
	if err != nil {
		return nil, errors.Wrap(err, "cannot retrieve x.509 key pair from the Kine Secret")
	}

	return sql.GetDBConnection(
		sql.ConnectionConfig{
			SQLDriver: driver,
			User:      string(secret.Data["username"]),
			Password:  string(secret.Data["password"]),
			Host:      r.Config.KineHost,
			Port:      r.Config.KinePort,
			DBName:    dbName,
			TLSConfig: &tls.Config{
				ServerName:   r.Config.KineHost,
				RootCAs:      rootCAs,
				Certificates: []tls.Certificate{certificate},
			},
		},
	)
}
