// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package datastore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

// NATSConnection represents a connection to a NATS KV store.
type NATSConnection struct {
	js     nats.JetStreamContext
	conn   *nats.Conn
	config ConnectionConfig
}

// NewNATSConnection initializes a connection to NATS and sets up the KV store.
func NewNATSConnection(config ConnectionConfig) (*NATSConnection, error) {
	var endpoints string

	if len(config.Endpoints) > 1 {
		// comma separated list of endpoints
		var ep []string
		for _, e := range config.Endpoints {
			ep = append(ep, fmt.Sprintf("nats://%s", e.String()))
		}

		endpoints = strings.Join(ep, ",")
	} else {
		endpoints = fmt.Sprintf("nats://%s", config.Endpoints[0].String())
	}

	var conn *nats.Conn
	var err error
	var natsOpts []nats.Option

	if config.TLSConfig != nil {
		natsOpts = append(natsOpts, nats.Secure(config.TLSConfig))
	}

	if config.User != "" && config.Password != "" {
		natsOpts = append(natsOpts, nats.UserInfo(config.User, config.Password))
	}

	conn, err = nats.Connect(endpoints, natsOpts...)
	if err != nil {
		return nil, err
	}

	js, err := conn.JetStream()
	if err != nil {
		return nil, err
	}

	return &NATSConnection{
		js:     js,
		conn:   conn,
		config: config,
	}, nil
}

func (nc *NATSConnection) CreateUser(_ context.Context, _, _ string) error {
	return nil
}

func (nc *NATSConnection) CreateDB(_ context.Context, dbName string) error {
	_, err := nc.js.CreateKeyValue(&nats.KeyValueConfig{Bucket: dbName})
	if err != nil {
		return fmt.Errorf("unable to create the datastore: %w", err)
	}

	return nil
}

func (nc *NATSConnection) GrantPrivileges(_ context.Context, _, _ string) error {
	return nil
}

func (nc *NATSConnection) UserExists(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (nc *NATSConnection) DBExists(_ context.Context, dbName string) (bool, error) {
	_, err := nc.js.KeyValue(dbName)
	if err != nil {
		if errors.Is(err, nats.ErrBucketNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (nc *NATSConnection) GrantPrivilegesExists(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

func (nc *NATSConnection) DeleteUser(_ context.Context, _ string) error {
	return nil
}

func (nc *NATSConnection) DeleteDB(_ context.Context, dbName string) error {
	err := nc.js.DeleteKeyValue(dbName)

	return err
}

func (nc *NATSConnection) RevokePrivileges(_ context.Context, _, _ string) error {
	return nil
}

func (nc *NATSConnection) GetConnectionString() string {
	return nc.config.Endpoints[0].String()
}

func (nc *NATSConnection) Close() error {
	return nc.conn.Drain()
}

func (nc *NATSConnection) Check(_ context.Context) error {
	status := nc.conn.Status()

	if status != nats.CONNECTED {
		return errors.New("connection to NATS is not established")
	}

	return nil
}

func (nc *NATSConnection) Driver() string {
	return string(kamajiv1alpha1.KineNatsDriver)
}

func (nc *NATSConnection) GetConfig() ConnectionConfig {
	return nc.config
}

func (nc *NATSConnection) Migrate(ctx context.Context, tcp kamajiv1alpha1.TenantControlPlane, target Connection) error {
	targetClient := target.(*NATSConnection) //nolint:forcetypeassert
	dbName := tcp.Status.Storage.Setup.Schema

	targetKv, err := targetClient.js.KeyValue(dbName)
	if err != nil {
		return err
	}

	sourceKv, err := nc.js.KeyValue(dbName)
	if err != nil {
		return err
	}

	if err := target.Check(ctx); err != nil {
		return err
	}

	// copy all keys from source to target
	keys, err := sourceKv.Keys()
	if err != nil {
		return err
	}

	for _, key := range keys {
		entry, err := sourceKv.Get(key)
		if err != nil {
			return err
		}

		_, err = targetKv.Put(key, entry.Value())
		if err != nil {
			return err
		}
	}

	return nil
}
