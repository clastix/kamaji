// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/clastix/kamaji-telemetry/api"
	telemetry "github.com/clastix/kamaji-telemetry/pkg/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
)

type TelemetryController struct {
	Client                  client.Client
	TelemetryClient         telemetry.Client
	LeaderElectionNamespace string
	LeaderElectionID        string
	KamajiVersion           string
	KubernetesVersion       string
}

func (m *TelemetryController) retrieveControllerUID(ctx context.Context) (string, error) {
	var defaultSvc corev1.Service
	if err := m.Client.Get(ctx, types.NamespacedName{Name: "kubernetes", Namespace: "default"}, &defaultSvc); err != nil {
		return "", fmt.Errorf("cannot start the telemetry controller: %w", err)
	}

	return string(defaultSvc.UID), nil
}

func (m *TelemetryController) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	uid, err := m.retrieveControllerUID(ctx)
	if err != nil {
		logger.Error(err, "cannot retrieve controller UID")

		return err
	}
	// First run to avoid waiting 5 minutes
	go m.collectStats(ctx, uid)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			go m.collectStats(ctx, uid)
		}
	}
}

func (m *TelemetryController) collectStats(ctx context.Context, uid string) {
	logger := log.FromContext(ctx)

	stats := api.Stats{
		UUID:                uid,
		KamajiVersion:       m.KamajiVersion,
		KubernetesVersion:   m.KubernetesVersion,
		TenantControlPlanes: api.TenantControlPlane{},
		Datastores:          api.Datastores{},
	}

	var tcpList kamajiv1alpha1.TenantControlPlaneList
	if err := m.Client.List(ctx, &tcpList); err != nil {
		logger.Error(err, "cannot list TenantControlPlane")

		return
	}

	for _, tcp := range tcpList.Items {
		switch {
		case ptr.Deref(tcp.Status.Kubernetes.Version.Status, kamajiv1alpha1.VersionProvisioning) == kamajiv1alpha1.VersionSleeping:
			stats.TenantControlPlanes.Sleeping++
		case tcp.Status.Kubernetes.Version.Status != nil && *tcp.Status.Kubernetes.Version.Status == kamajiv1alpha1.VersionNotReady:
			stats.TenantControlPlanes.NotReady++
		case tcp.Status.Kubernetes.Version.Status != nil && *tcp.Status.Kubernetes.Version.Status == kamajiv1alpha1.VersionUpgrading:
			stats.TenantControlPlanes.Upgrading++
		default:
			stats.TenantControlPlanes.Running++
		}
	}

	var datastoreList kamajiv1alpha1.DataStoreList
	if err := m.Client.List(ctx, &datastoreList); err != nil {
		logger.Error(err, "cannot list DataStores")

		return
	}

	for _, ds := range datastoreList.Items {
		switch ds.Spec.Driver {
		case kamajiv1alpha1.EtcdDriver:
			stats.Datastores.Etcd.ShardCount++
			stats.Datastores.Etcd.UsedByCount += len(ds.Status.UsedBy)
		case kamajiv1alpha1.KinePostgreSQLDriver:
			stats.Datastores.PostgreSQL.ShardCount++
			stats.Datastores.PostgreSQL.UsedByCount += len(ds.Status.UsedBy)
		case kamajiv1alpha1.KineMySQLDriver:
			stats.Datastores.MySQL.ShardCount++
			stats.Datastores.MySQL.UsedByCount += len(ds.Status.UsedBy)
		case kamajiv1alpha1.KineNatsDriver:
			stats.Datastores.NATS.ShardCount++
			stats.Datastores.NATS.UsedByCount += len(ds.Status.UsedBy)
		}
	}

	m.TelemetryClient.PushStats(ctx, stats)
}
