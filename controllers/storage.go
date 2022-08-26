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
	"github.com/clastix/kamaji/internal/datastore"
)

func (r *TenantControlPlaneReconciler) getStorageConnection(ctx context.Context, ds kamajiv1alpha1.DataStore) (datastore.Connection, error) {
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

	eps := make([]datastore.ConnectionEndpoint, 0, len(ds.Spec.Endpoints))

	for _, ep := range ds.Spec.Endpoints {
		host, stringPort, err := net.SplitHostPort(ep)
		if err != nil {
			return nil, errors.Wrap(err, "cannot retrieve host-port pair from DataStore endpoints")
		}

		port, err := strconv.Atoi(stringPort)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert port from string for the given DataStore")
		}

		eps = append(eps, datastore.ConnectionEndpoint{
			Host: host,
			Port: port,
		})
	}

	cc := datastore.ConnectionConfig{
		User:      user,
		Password:  password,
		Endpoints: eps,
		TLSConfig: &tls.Config{
			RootCAs:      rootCAs,
			Certificates: []tls.Certificate{certificate},
		},
	}

	switch ds.Spec.Driver {
	case kamajiv1alpha1.KineMySQLDriver:
		cc.TLSConfig.ServerName = cc.Endpoints[0].Host

		return datastore.NewMySQLConnection(cc)
	case kamajiv1alpha1.KinePostgreSQLDriver:
		cc.TLSConfig.ServerName = cc.Endpoints[0].Host

		return datastore.NewPostgreSQLConnection(cc)
	case kamajiv1alpha1.EtcdDriver:
		return datastore.NewETCDConnection(cc)
	default:
		return nil, fmt.Errorf("%s is not a valid driver", ds.Spec.Driver)
	}
}
