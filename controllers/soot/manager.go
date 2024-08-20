// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package soot

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/finalizers"
	"github.com/clastix/kamaji/controllers/soot/controllers"
	"github.com/clastix/kamaji/controllers/utils"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/utilities"
)

type sootItem struct {
	triggers []chan event.GenericEvent
	cancelFn context.CancelFunc
}

type sootMap map[string]sootItem

type Manager struct {
	client  client.Client
	sootMap sootMap
	// sootManagerErrChan is the channel that is going to be used
	// when the soot manager cannot start due to any kind of problem.
	sootManagerErrChan chan event.GenericEvent

	MigrateCABundle         []byte
	MigrateServiceName      string
	MigrateServiceNamespace string
	AdminClient             client.Client
}

// retrieveTenantControlPlane is the function used to let an underlying controller of the soot manager
// to retrieve its parent TenantControlPlane definition, required to understand which actions must be performed.
func (m *Manager) retrieveTenantControlPlane(ctx context.Context, request reconcile.Request) utils.TenantControlPlaneRetrievalFn {
	return func() (*kamajiv1alpha1.TenantControlPlane, error) {
		tcp := &kamajiv1alpha1.TenantControlPlane{}

		if err := m.client.Get(ctx, request.NamespacedName, tcp); err != nil {
			return nil, err
		}

		return tcp, nil
	}
}

// If the TenantControlPlane is deleted we have to free up memory by stopping the soot manager:
// this is made possible by retrieving the cancel function of the soot manager context to cancel it.
func (m *Manager) cleanup(ctx context.Context, req reconcile.Request, tenantControlPlane *kamajiv1alpha1.TenantControlPlane) (err error) {
	if tenantControlPlane != nil && controllerutil.ContainsFinalizer(tenantControlPlane, finalizers.SootFinalizer) {
		defer func() {
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				tcp, tcpErr := m.retrieveTenantControlPlane(ctx, req)()
				if tcpErr != nil {
					return tcpErr
				}

				controllerutil.RemoveFinalizer(tcp, finalizers.SootFinalizer)

				return m.AdminClient.Update(ctx, tcp)
			})
		}()
	}

	tcpName := req.NamespacedName.String()

	v, ok := m.sootMap[tcpName]
	if !ok {
		return nil
	}

	v.cancelFn()

	delete(m.sootMap, tcpName)

	return nil
}

func (m *Manager) Reconcile(ctx context.Context, request reconcile.Request) (res reconcile.Result, err error) {
	// Retrieving the TenantControlPlane:
	// in case of deletion, we must be sure to properly remove from the memory the soot manager.
	tcp := &kamajiv1alpha1.TenantControlPlane{}
	if err = m.client.Get(ctx, request.NamespacedName, tcp); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, m.cleanup(ctx, request, nil)
		}

		return reconcile.Result{}, err
	}
	// Handling finalizer if the TenantControlPlane is marked for deletion:
	// the clean-up function is already taking care to stop the manager, if this exists.
	if tcp.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(tcp, finalizers.SootFinalizer) {
			return reconcile.Result{}, m.cleanup(ctx, request, tcp)
		}

		return reconcile.Result{}, nil
	}

	tcpStatus := *tcp.Status.Kubernetes.Version.Status
	// Triggering the reconciliation of the underlying controllers of
	// the soot manager if this is already registered.
	v, ok := m.sootMap[request.String()]
	if ok {
		switch {
		case tcpStatus == kamajiv1alpha1.VersionCARotating:
			// The TenantControlPlane CA has been rotated, it means the running manager
			// must be restarted to avoid certificate signed by unknown authority errors.
			return reconcile.Result{}, m.cleanup(ctx, request, tcp)
		case tcpStatus == kamajiv1alpha1.VersionNotReady:
			// The TenantControlPlane is in non-ready mode, or marked for deletion:
			// we don't want to pollute with messages due to broken connection.
			// Once the TCP will be ready again, the event will be intercepted and the manager started back.
			return reconcile.Result{}, m.cleanup(ctx, request, tcp)
		default:
			for _, trigger := range v.triggers {
				trigger <- event.GenericEvent{Object: tcp}
			}
		}

		return reconcile.Result{}, nil
	}
	// No need to start a soot manager if the TenantControlPlane is not ready:
	// enqueuing back is not required since we're going to get that event once ready.
	if tcpStatus == kamajiv1alpha1.VersionNotReady || tcpStatus == kamajiv1alpha1.VersionCARotating {
		log.FromContext(ctx).Info("skipping start of the soot manager for a not ready instance")

		return reconcile.Result{}, nil
	}
	// Setting the finalizer for the soot manager:
	// upon deletion the soot manager will be shut down prior the Deployment, avoiding logs pollution.
	if !controllerutil.ContainsFinalizer(tcp, finalizers.SootFinalizer) {
		_, finalizerErr := utilities.CreateOrUpdateWithConflict(ctx, m.AdminClient, tcp, func() error {
			controllerutil.AddFinalizer(tcp, finalizers.SootFinalizer)

			return nil
		})

		return reconcile.Result{Requeue: true}, finalizerErr
	}
	// Generating the manager and starting it:
	// in case of any error, reconciling the request to start it back from the beginning.
	tcpRest, err := utilities.GetRESTClientConfig(ctx, m.client, tcp)
	if err != nil {
		return reconcile.Result{}, err
	}

	tcpCtx, tcpCancelFn := context.WithCancel(ctx)
	defer func() {
		// If the reconciliation fails, we don't need to get a potential dangling goroutine.
		if err != nil {
			tcpCancelFn()
		}
	}()

	mgr, err := controllerruntime.NewManager(tcpRest, controllerruntime.Options{
		Logger: log.Log.WithName(fmt.Sprintf("soot_%s_%s", tcp.GetNamespace(), tcp.GetName())),
		Scheme: m.client.Scheme(),
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		NewClient: func(config *rest.Config, options client.Options) (client.Client, error) {
			return client.New(config, client.Options{
				Scheme: m.client.Scheme(),
			})
		},
	})
	if err != nil {
		return reconcile.Result{}, err
	}
	//
	// Register all the controllers of the soot here:
	//
	migrate := &controllers.Migrate{
		WebhookNamespace:          m.MigrateServiceNamespace,
		WebhookServiceName:        m.MigrateServiceName,
		WebhookCABundle:           m.MigrateCABundle,
		GetTenantControlPlaneFunc: m.retrieveTenantControlPlane(tcpCtx, request),
	}
	if err = migrate.SetupWithManager(mgr); err != nil {
		return reconcile.Result{}, err
	}

	konnectivityAgent := &controllers.KonnectivityAgent{
		AdminClient:               m.AdminClient,
		GetTenantControlPlaneFunc: m.retrieveTenantControlPlane(tcpCtx, request),
	}
	if err = konnectivityAgent.SetupWithManager(mgr); err != nil {
		return reconcile.Result{}, err
	}

	kubeProxy := &controllers.KubeProxy{
		AdminClient:               m.AdminClient,
		GetTenantControlPlaneFunc: m.retrieveTenantControlPlane(tcpCtx, request),
	}
	if err = kubeProxy.SetupWithManager(mgr); err != nil {
		return reconcile.Result{}, err
	}

	coreDNS := &controllers.CoreDNS{
		AdminClient:               m.AdminClient,
		GetTenantControlPlaneFunc: m.retrieveTenantControlPlane(tcpCtx, request),
	}
	if err = coreDNS.SetupWithManager(mgr); err != nil {
		return reconcile.Result{}, err
	}

	uploadKubeadmConfig := &controllers.KubeadmPhase{
		GetTenantControlPlaneFunc: m.retrieveTenantControlPlane(tcpCtx, request),
		Phase: &resources.KubeadmPhase{
			Client: m.AdminClient,
			Phase:  resources.PhaseUploadConfigKubeadm,
		},
	}
	if err = uploadKubeadmConfig.SetupWithManager(mgr); err != nil {
		return reconcile.Result{}, err
	}

	uploadKubeletConfig := &controllers.KubeadmPhase{
		GetTenantControlPlaneFunc: m.retrieveTenantControlPlane(tcpCtx, request),
		Phase: &resources.KubeadmPhase{
			Client: m.AdminClient,
			Phase:  resources.PhaseUploadConfigKubelet,
		},
	}
	if err = uploadKubeletConfig.SetupWithManager(mgr); err != nil {
		return reconcile.Result{}, err
	}

	bootstrapToken := &controllers.KubeadmPhase{
		GetTenantControlPlaneFunc: m.retrieveTenantControlPlane(tcpCtx, request),
		Phase: &resources.KubeadmPhase{
			Client: m.AdminClient,
			Phase:  resources.PhaseBootstrapToken,
		},
	}
	if err = bootstrapToken.SetupWithManager(mgr); err != nil {
		return reconcile.Result{}, err
	}

	kubeadmRbac := &controllers.KubeadmPhase{
		GetTenantControlPlaneFunc: m.retrieveTenantControlPlane(tcpCtx, request),
		Phase: &resources.KubeadmPhase{
			Client: m.AdminClient,
			Phase:  resources.PhaseClusterAdminRBAC,
		},
	}
	if err = kubeadmRbac.SetupWithManager(mgr); err != nil {
		return reconcile.Result{}, err
	}
	// Starting the manager
	go func() {
		if err = mgr.Start(tcpCtx); err != nil {
			log.FromContext(ctx).Error(err, "unable to start soot manager")
			// When the manager cannot start we're enqueuing back the request to take advantage of the backoff factor
			// of the queue: this is a goroutine and cannot return an error since the manager is running on its own,
			// using the sootManagerErrChan channel we can trigger a reconciliation although the TCP hadn't any change.
			m.sootManagerErrChan <- event.GenericEvent{Object: tcp}
		}
	}()

	m.sootMap[request.NamespacedName.String()] = sootItem{
		triggers: []chan event.GenericEvent{
			migrate.TriggerChannel,
			konnectivityAgent.TriggerChannel,
			kubeProxy.TriggerChannel,
			coreDNS.TriggerChannel,
			uploadKubeadmConfig.TriggerChannel,
			uploadKubeletConfig.TriggerChannel,
			bootstrapToken.TriggerChannel,
		},
		cancelFn: tcpCancelFn,
	}

	return reconcile.Result{Requeue: true}, nil
}

func (m *Manager) SetupWithManager(mgr manager.Manager) error {
	m.client = mgr.GetClient()
	m.sootManagerErrChan = make(chan event.GenericEvent)
	m.sootMap = make(map[string]sootItem)

	return controllerruntime.NewControllerManagedBy(mgr).
		WithOptions(controller.TypedOptions[reconcile.Request]{SkipNameValidation: ptr.To(true)}).
		WatchesRawSource(source.Channel(m.sootManagerErrChan, &handler.EnqueueRequestForObject{})).
		For(&kamajiv1alpha1.TenantControlPlane{}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			obj := object.(*kamajiv1alpha1.TenantControlPlane) //nolint:forcetypeassert
			// status is required to understand if we have to start or stop the soot manager
			if obj.Status.Kubernetes.Version.Status == nil {
				return false
			}

			if *obj.Status.Kubernetes.Version.Status == kamajiv1alpha1.VersionProvisioning {
				return false
			}

			return true
		}))).
		Complete(m)
}
