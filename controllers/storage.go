// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"

	"github.com/pkg/errors"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/sql"
)

func (r *TenantControlPlaneReconciler) getStorageConnection(ctx context.Context, ds kamajiv1alpha1.DataStore) (sql.DBConnection, error) {
	var driver sql.Driver
	var dbName string

	// TODO: https://github.com/clastix/kamaji/issues/67
	switch ds.Spec.Driver {
	case kamajiv1alpha1.EtcdDriver:
		return nil, nil
	case kamajiv1alpha1.KineMySQLDriver:
		driver = sql.MySQL
		dbName = "mysql"
	case kamajiv1alpha1.KinePostgreSQLDriver:
		driver = sql.PostgreSQL
	default:
		return nil, nil
	}

	ca, err := ds.Spec.TLSConfig.CertificateAuthority.Certificate.GetContent(ctx, r.Client)
	if err != nil {
		return nil, err
	}

	crt, err := ds.Spec.TLSConfig.ClientCertificate.Certificate.GetContent(ctx, r.Client)
	if err != nil {
		return nil, err
	}

	key, err := ds.Spec.TLSConfig.ClientCertificate.PrivateKey.GetContent(ctx, r.Client)
	if err != nil {
		return nil, err
	}

	rootCAs := x509.NewCertPool()
	if ok := rootCAs.AppendCertsFromPEM(ca); !ok {
		return nil, fmt.Errorf("error create root CA for the DB connector")
	}

	certificate, err := tls.X509KeyPair(crt, key)
	if err != nil {
		return nil, errors.Wrap(err, "cannot retrieve x.509 key pair from the Kine Secret")
	}

	var user, password string
	if auth := ds.Spec.BasicAuth; auth != nil {
		u, err := auth.Username.GetContent(ctx, r.Client)
		if err != nil {
			return nil, err
		}
		user = string(u)

		p, err := auth.Password.GetContent(ctx, r.Client)
		if err != nil {
			return nil, err
		}
		password = string(p)
	}

	host, stringPort, err := net.SplitHostPort(ds.Spec.Endpoints[0])
	if err != nil {
		return nil, errors.Wrap(err, "cannot retrieve host-port pair from DataStore endpoints")
	}

	port, err := strconv.Atoi(stringPort)
	if err != nil {
		return nil, errors.Wrap(err, "cannot convert port from string for the given DataStore")
	}

	return sql.GetDBConnection(
		sql.ConnectionConfig{
			SQLDriver: driver,
			User:      user,
			Password:  password,
			Host:      host,
			Port:      port,
			DBName:    dbName,
			TLSConfig: &tls.Config{
				ServerName:   host,
				RootCAs:      rootCAs,
				Certificates: []tls.Certificate{certificate},
			},
		},
	)
}
