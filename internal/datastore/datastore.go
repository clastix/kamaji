// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

type ConnectionEndpoint struct {
	Host string
	Port int
}

func (r ConnectionEndpoint) String() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type ConnectionConfig struct {
	User       string
	Password   string
	Endpoints  []ConnectionEndpoint
	DBName     string
	TLSConfig  *tls.Config
	Parameters map[string][]string
}

func NewConnectionConfig(ctx context.Context, client client.Client, ds kamajiv1alpha1.DataStore) (*ConnectionConfig, error) {
	ca, err := ds.Spec.TLSConfig.CertificateAuthority.Certificate.GetContent(ctx, client)
	if err != nil {
		return nil, err
	}

	crt, err := ds.Spec.TLSConfig.ClientCertificate.Certificate.GetContent(ctx, client)
	if err != nil {
		return nil, err
	}

	key, err := ds.Spec.TLSConfig.ClientCertificate.PrivateKey.GetContent(ctx, client)
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
		u, err := auth.Username.GetContent(ctx, client)
		if err != nil {
			return nil, err
		}
		user = string(u)

		p, err := auth.Password.GetContent(ctx, client)
		if err != nil {
			return nil, err
		}
		password = string(p)
	}

	eps := make([]ConnectionEndpoint, 0, len(ds.Spec.Endpoints))

	for _, ep := range ds.Spec.Endpoints {
		host, stringPort, err := net.SplitHostPort(ep)
		if err != nil {
			return nil, errors.Wrap(err, "cannot retrieve host-port pair from DataStore endpoints")
		}

		port, err := strconv.Atoi(stringPort)
		if err != nil {
			return nil, errors.Wrap(err, "cannot convert port from string for the given DataStore")
		}

		eps = append(eps, ConnectionEndpoint{
			Host: host,
			Port: port,
		})
	}

	return &ConnectionConfig{
		User:      user,
		Password:  password,
		Endpoints: eps,
		TLSConfig: &tls.Config{
			RootCAs:      rootCAs,
			Certificates: []tls.Certificate{certificate},
		},
	}, nil
}

func (config ConnectionConfig) getDataSourceNameUserPassword() string {
	if config.User == "" {
		return ""
	}

	if config.Password == "" {
		return fmt.Sprintf("%s@", config.User)
	}

	return fmt.Sprintf("%s:%s@", config.User, config.Password)
}
