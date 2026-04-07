// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"crypto/x509"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/utils"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/metrics"
	"github.com/clastix/kamaji/internal/utilities"
)

type CertificateLifecycle struct {
	Channel   chan event.GenericEvent
	Deadline  time.Duration
	EnqueueFn func(secret *corev1.Secret)
	Metrics   *metrics.Recorder

	client client.Client
}

func (s *CertificateLifecycle) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)
	defer func(c context.Context) {
		metricCtx, cancelMetricCtx := metrics.NewRefreshContextFrom(c)
		defer cancelMetricCtx()

		if err := s.refreshCertificatesMetrics(metricCtx); err != nil {
			logger.WithName("metrics").Error(err, "cannot refresh certificate status gauges")
		}
	}(ctx)

	logger.Info("starting CertificateLifecycle handling")

	var secret corev1.Secret
	if err := s.client.Get(ctx, request.NamespacedName, &secret); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info("resource may have been deleted, skipping")

			return reconcile.Result{}, nil
		}

		logger.Error(err, "cannot retrieve the required resource")

		return reconcile.Result{}, err
	}

	if utils.IsPaused(&secret) {
		logger.Info("paused reconciliation, no further actions")

		return reconcile.Result{}, nil
	}

	checkType, ok := secret.GetLabels()[constants.ControllerLabelResource]
	if !ok {
		logger.Info("missing controller label, shouldn't happen")

		return reconcile.Result{}, nil
	}

	var crt *x509.Certificate
	var err error

	switch checkType {
	case utilities.CertificateX509Label:
		crt, err = s.extractCertificateFromBareSecret(secret)
	case utilities.CertificateKubeconfigLabel:
		crt, err = s.extractCertificateFromKubeconfig(secret)
	default:
		return reconcile.Result{}, fmt.Errorf("unsupported strategy, %q", checkType)
	}

	if err != nil {
		logger.Error(err, "skipping reconciliation")

		return reconcile.Result{}, nil
	}

	deadline := time.Now().Add(s.Deadline)

	if deadline.After(crt.NotAfter) {
		logger.Info("certificate near expiration, must be rotated")

		s.EnqueueFn(&secret)

		logger.Info("certificate rotation triggered")

		return reconcile.Result{}, nil
	}

	after := crt.NotAfter.Sub(deadline)

	logger.Info("certificate is still valid, enqueuing back", "after", after.String())

	return reconcile.Result{RequeueAfter: after}, nil
}

func (s *CertificateLifecycle) EnqueueForTenantControlPlane(secret *corev1.Secret) {
	for _, or := range secret.GetOwnerReferences() {
		if or.Kind != "TenantControlPlane" {
			continue
		}

		s.Channel <- event.GenericEvent{Object: &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      or.Name,
				Namespace: secret.Namespace,
			},
		}}
	}
}

func (s *CertificateLifecycle) EnqueueForKubeconfigGenerator(secret *corev1.Secret) {
	for _, or := range secret.GetOwnerReferences() {
		if or.Kind != "KubeconfigGenerator" {
			continue
		}

		s.Channel <- event.GenericEvent{Object: &kamajiv1alpha1.TenantControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name: or.Name,
			},
		}}
	}
}

func (s *CertificateLifecycle) extractCertificateFromBareSecret(secret corev1.Secret) (*x509.Certificate, error) {
	var crt *x509.Certificate
	var err error

	for _, v := range secret.Data {
		if crt, err = crypto.ParseCertificateBytes(v); err == nil {
			break
		}
	}

	if crt == nil {
		return nil, fmt.Errorf("none of the provided keys is containing a valid x509 certificate")
	}

	return crt, nil
}

func (s *CertificateLifecycle) extractCertificateFromKubeconfig(secret corev1.Secret) (*x509.Certificate, error) {
	var kc *clientcmdapiv1.Config
	var err error

	for k := range secret.Data {
		if kc, err = utilities.DecodeKubeconfig(secret, k); err == nil {
			break
		}
	}

	if kc == nil {
		return nil, fmt.Errorf("none of the provided keys is containing a valid kubeconfig")
	}

	crt, err := crypto.ParseCertificateBytes(kc.AuthInfos[0].AuthInfo.ClientCertificateData)
	if err != nil {
		return nil, fmt.Errorf("cannot parse kubeconfig certificate bytes: %w", err)
	}

	return crt, nil
}

func (s *CertificateLifecycle) SetupWithManager(mgr controllerruntime.Manager) error {
	s.client = mgr.GetClient()

	if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		metricCtx, cancelMetricCtx := metrics.NewRefreshContextFrom(ctx)
		defer cancelMetricCtx()

		if err := s.refreshCertificatesMetrics(metricCtx); err != nil {
			controllerruntime.Log.WithName("metrics").Error(err, "cannot initialize certificate status gauges")
		}

		return nil
	})); err != nil {
		return err
	}

	supportedStrategies := sets.New[string](utilities.CertificateX509Label, utilities.CertificateKubeconfigLabel)

	return controllerruntime.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			labels := object.GetLabels()

			if labels == nil {
				return false
			}

			value, ok := labels[constants.ControllerLabelResource]
			if !ok {
				return false
			}

			return supportedStrategies.Has(value)
		}))).
		Complete(s)
}

func (s *CertificateLifecycle) refreshCertificatesMetrics(ctx context.Context) error {
	metricsRecorder := s.metricsRecorder()
	metricsRecorder.ResetCertificatesStatusCounts()

	countsByTenantControlPlane := map[k8stypes.NamespacedName]map[string]map[string]int{}

	var tenantControlPlaneList kamajiv1alpha1.TenantControlPlaneList
	if err := s.client.List(ctx, &tenantControlPlaneList); err != nil {
		return err
	}

	for i := range tenantControlPlaneList.Items {
		tcp := tenantControlPlaneList.Items[i]
		namespacedName := k8stypes.NamespacedName{Namespace: tcp.GetNamespace(), Name: tcp.GetName()}
		countsByTenantControlPlane[namespacedName] = metrics.NewCertificateStatusCounts()
	}

	var secretList corev1.SecretList
	if err := s.client.List(ctx, &secretList); err != nil {
		return err
	}

	deadline := time.Now().Add(s.Deadline)

	for i := range secretList.Items {
		secret := secretList.Items[i]
		labels := secret.GetLabels()
		if labels == nil {
			continue
		}

		tenantControlPlaneName, ok := labels[constants.ControlPlaneLabelKey]
		if !ok || tenantControlPlaneName == "" {
			continue
		}

		strategy, ok := certificateStrategyFromLabel(labels[constants.ControllerLabelResource])
		if !ok {
			continue
		}

		namespacedName := k8stypes.NamespacedName{Namespace: secret.GetNamespace(), Name: tenantControlPlaneName}
		if _, exists := countsByTenantControlPlane[namespacedName]; !exists {
			countsByTenantControlPlane[namespacedName] = metrics.NewCertificateStatusCounts()
		}

		var (
			crt *x509.Certificate
			err error
		)

		switch strategy {
		case metrics.CertificateStrategyX509:
			crt, err = s.extractCertificateFromBareSecret(secret)
		case metrics.CertificateStrategyKubeconfig:
			crt, err = s.extractCertificateFromKubeconfig(secret)
		default:
			continue
		}

		if err != nil {
			countsByTenantControlPlane[namespacedName][metrics.CertificateStatusInvalid][strategy]++

			continue
		}

		if deadline.After(crt.NotAfter) {
			countsByTenantControlPlane[namespacedName][metrics.CertificateStatusExpiring][strategy]++

			continue
		}

		countsByTenantControlPlane[namespacedName][metrics.CertificateStatusValid][strategy]++
	}

	for namespacedName, counts := range countsByTenantControlPlane {
		metricsRecorder.SetCertificatesStatusCounts(namespacedName.Namespace, namespacedName.Name, counts)
	}

	return nil
}

func certificateStrategyFromLabel(label string) (string, bool) {
	switch label {
	case utilities.CertificateX509Label:
		return metrics.CertificateStrategyX509, true
	case utilities.CertificateKubeconfigLabel:
		return metrics.CertificateStrategyKubeconfig, true
	default:
		return "", false
	}
}

func (s *CertificateLifecycle) metricsRecorder() *metrics.Recorder {
	if s.Metrics == nil {
		s.Metrics = metrics.DefaultRecorder()
	}

	return s.Metrics
}
