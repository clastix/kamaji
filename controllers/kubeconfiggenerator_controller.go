// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	clientcmdapiv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/client-go/util/workqueue"
	kubeadmconstants "k8s.io/kubernetes/cmd/kubeadm/app/constants"
	"k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/controllers/utils"
	"github.com/clastix/kamaji/internal/constants"
	"github.com/clastix/kamaji/internal/crypto"
	"github.com/clastix/kamaji/internal/resources"
	"github.com/clastix/kamaji/internal/utilities"
)

type KubeconfigGeneratorReconciler struct {
	Client            client.Client
	NotValidThreshold time.Duration
	CertificateChan   chan event.GenericEvent
}

//+kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=kubeconfiggenerators,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=kubeconfiggenerators/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kamaji.clastix.io,resources=kubeconfiggenerators/finalizers,verbs=update

func (r *KubeconfigGeneratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("reconciling resource")

	var generator kamajiv1alpha1.KubeconfigGenerator
	if err := r.Client.Get(ctx, req.NamespacedName, &generator); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("resource may have been deleted, skipping")

			return ctrl.Result{}, nil
		}

		logger.Error(err, "cannot retrieve the required resource")

		return ctrl.Result{}, err
	}

	if utils.IsPaused(&generator) {
		logger.Info("paused reconciliation, no further actions")

		return ctrl.Result{}, nil
	}

	status, err := r.handle(ctx, &generator)
	if err != nil {
		logger.Error(err, "cannot handle the request")

		return ctrl.Result{}, err
	}

	generator.Status = status

	if statusErr := r.Client.Status().Update(ctx, &generator); statusErr != nil {
		logger.Error(statusErr, "cannot update resource status")

		return ctrl.Result{}, statusErr
	}

	logger.Info("reconciling completed")

	return ctrl.Result{}, nil
}

func (r *KubeconfigGeneratorReconciler) handle(ctx context.Context, generator *kamajiv1alpha1.KubeconfigGenerator) (kamajiv1alpha1.KubeconfigGeneratorStatus, error) {
	nsSelector, nsErr := metav1.LabelSelectorAsSelector(&generator.Spec.NamespaceSelector)
	if nsErr != nil {
		return kamajiv1alpha1.KubeconfigGeneratorStatus{}, fmt.Errorf("NamespaceSelector contains an error: %w", nsErr)
	}

	var namespaceList corev1.NamespaceList
	if err := r.Client.List(ctx, &namespaceList, &client.ListOptions{LabelSelector: nsSelector}); err != nil {
		return kamajiv1alpha1.KubeconfigGeneratorStatus{}, fmt.Errorf("cannot filter Namespace objects using provided selector: %w", err)
	}

	var targets []kamajiv1alpha1.TenantControlPlane

	for _, ns := range namespaceList.Items {
		tcpSelector, tcpErr := metav1.LabelSelectorAsSelector(&generator.Spec.TenantControlPlaneSelector)
		if tcpErr != nil {
			return kamajiv1alpha1.KubeconfigGeneratorStatus{}, fmt.Errorf("TenantControlPlaneSelector contains an error: %w", tcpErr)
		}

		var tcpList kamajiv1alpha1.TenantControlPlaneList
		if err := r.Client.List(ctx, &tcpList, &client.ListOptions{Namespace: ns.GetName(), LabelSelector: tcpSelector}); err != nil {
			return kamajiv1alpha1.KubeconfigGeneratorStatus{}, fmt.Errorf("cannot filter TenantControlPlane objects using provided selector: %w", err)
		}

		targets = append(targets, tcpList.Items...)
	}

	sort.Slice(targets, func(i, j int) bool {
		return client.ObjectKeyFromObject(&targets[i]).String() < client.ObjectKeyFromObject(&targets[j]).String()
	})

	status := kamajiv1alpha1.KubeconfigGeneratorStatus{
		Resources:          len(targets),
		AvailableResources: len(targets),
	}

	for _, tcp := range targets {
		if err := r.process(ctx, generator, tcp); err != nil {
			status.Errors = append(status.Errors, *err)
			status.AvailableResources--
		}
	}

	return status, nil
}

func (r *KubeconfigGeneratorReconciler) process(ctx context.Context, generator *kamajiv1alpha1.KubeconfigGenerator, tcp kamajiv1alpha1.TenantControlPlane) *kamajiv1alpha1.KubeconfigGeneratorStatusError {
	statusErr := kamajiv1alpha1.KubeconfigGeneratorStatusError{
		Resource: client.ObjectKeyFromObject(&tcp).String(),
	}

	var adminSecret corev1.Secret

	if tcp.Status.KubeConfig.Admin.SecretName == "" {
		statusErr.Message = "the admin kubeconfig is not yet generated"

		return &statusErr
	}

	if err := r.Client.Get(ctx, types.NamespacedName{Name: tcp.Status.KubeConfig.Admin.SecretName, Namespace: tcp.GetNamespace()}, &adminSecret); err != nil {
		statusErr.Message = fmt.Sprintf("an error occurred retrieving the admin Kubeconfig: %s", err.Error())

		return &statusErr
	}

	kubeconfigTmpl, kcErr := utilities.DecodeKubeconfig(adminSecret, generator.Spec.ControlPlaneEndpointFrom)
	if kcErr != nil {
		statusErr.Message = fmt.Sprintf("unable to decode Kubeconfig template: %s", kcErr.Error())

		return &statusErr
	}

	uMap, uErr := runtime.DefaultUnstructuredConverter.ToUnstructured(&tcp)
	if uErr != nil {
		statusErr.Message = fmt.Sprintf("cannot convert the resource to a map: %s", uErr)

		return &statusErr
	}

	var user string
	groups := sets.New[string]()

	for _, group := range generator.Spec.Groups {
		switch {
		case group.StringValue != "":
			groups.Insert(group.StringValue)
		case group.FromDefinition != "":
			v, ok, vErr := unstructured.NestedString(uMap, strings.Split(group.FromDefinition, ".")...)
			switch {
			case vErr != nil:
				statusErr.Message = fmt.Sprintf("cannot run NestedString %q due to an error: %s", group.FromDefinition, vErr.Error())

				return &statusErr
			case !ok:
				statusErr.Message = fmt.Sprintf("provided dot notation %q is not found", group.FromDefinition)

				return &statusErr
			default:
				groups.Insert(v)
			}
		default:
			statusErr.Message = "at least a StringValue or FromDefinition Group value must be provided"

			return &statusErr
		}
	}

	switch {
	case generator.Spec.User.StringValue != "":
		user = generator.Spec.User.StringValue
	case generator.Spec.User.FromDefinition != "":
		v, ok, vErr := unstructured.NestedString(uMap, strings.Split(generator.Spec.User.FromDefinition, ".")...)

		switch {
		case vErr != nil:
			statusErr.Message = fmt.Sprintf("cannot run NestedString %q due to an error: %s", generator.Spec.User.FromDefinition, vErr.Error())

			return &statusErr
		case !ok:
			statusErr.Message = fmt.Sprintf("provided dot notation %q is not found", generator.Spec.User.FromDefinition)

			return &statusErr
		default:
			user = v
		}
	default:
		statusErr.Message = "at least a StringValue or FromDefinition for the user field must be provided"

		return &statusErr
	}

	var resultSecret corev1.Secret
	resultSecret.SetName(tcp.Name + "-" + generator.Name)
	resultSecret.SetNamespace(tcp.Namespace)

	objectKey := client.ObjectKeyFromObject(&resultSecret)

	if err := r.Client.Get(ctx, objectKey, &resultSecret); err != nil {
		if !apierrors.IsNotFound(err) {
			statusErr.Message = fmt.Sprintf("the secret %q cannot be generated", objectKey.String())

			return &statusErr
		}

		if generateErr := r.generate(ctx, generator, &resultSecret, kubeconfigTmpl, &tcp, groups, user); generateErr != nil {
			statusErr.Message = fmt.Sprintf("an error occurred generating the %q Secret: %s", objectKey.String(), generateErr.Error())

			return &statusErr
		}

		return nil
	}

	isValid, validateErr := r.isValid(&resultSecret, kubeconfigTmpl, groups, user)
	switch {
	case !isValid:
		if generateErr := r.generate(ctx, generator, &resultSecret, kubeconfigTmpl, &tcp, groups, user); generateErr != nil {
			statusErr.Message = fmt.Sprintf("an error occurred regenerating the %q Secret: %s", objectKey.String(), generateErr.Error())

			return &statusErr
		}

		return nil
	case validateErr != nil:
		statusErr.Message = fmt.Sprintf("an error occurred checking validation for %q Secret: %s", objectKey.String(), validateErr.Error())

		return &statusErr
	default:
		return nil
	}
}

func (r *KubeconfigGeneratorReconciler) generate(ctx context.Context, generator *kamajiv1alpha1.KubeconfigGenerator, secret *corev1.Secret, tmpl *clientcmdapiv1.Config, tcp *kamajiv1alpha1.TenantControlPlane, groups sets.Set[string], user string) error {
	_, config, err := resources.GetKubeadmManifestDeps(ctx, r.Client, tcp)
	if err != nil {
		return err
	}

	clientCertConfig := pkiutil.CertConfig{
		Config: certutil.Config{
			CommonName:   user,
			Organization: groups.UnsortedList(),
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		NotAfter:            util.StartTimeUTC().Add(kubeadmconstants.CertificateValidityPeriod),
		EncryptionAlgorithm: config.InitConfiguration.ClusterConfiguration.EncryptionAlgorithmType(),
	}

	var caSecret corev1.Secret
	if caErr := r.Client.Get(ctx, types.NamespacedName{Namespace: tcp.Namespace, Name: tcp.Status.Certificates.CA.SecretName}, &caSecret); caErr != nil {
		return fmt.Errorf("cannot retrieve Certificate Authority: %w", caErr)
	}

	caCert, crtErr := crypto.ParseCertificateBytes(caSecret.Data[kubeadmconstants.CACertName])
	if crtErr != nil {
		return fmt.Errorf("cannot parse Certificate Authority certificate: %w", crtErr)
	}

	caKey, keyErr := crypto.ParsePrivateKeyBytes(caSecret.Data[kubeadmconstants.CAKeyName])
	if keyErr != nil {
		return fmt.Errorf("cannot parse Certificate Authority key: %w", keyErr)
	}

	clientCert, clientKey, err := pkiutil.NewCertAndKey(caCert, caKey, &clientCertConfig)

	contextUserName := generator.Name

	for name := range tmpl.AuthInfos {
		tmpl.AuthInfos[name].Name = contextUserName
		tmpl.AuthInfos[name].AuthInfo.ClientCertificateData = pkiutil.EncodeCertPEM(clientCert)
		tmpl.AuthInfos[name].AuthInfo.ClientKeyData, err = keyutil.MarshalPrivateKeyToPEM(clientKey)
		if err != nil {
			return fmt.Errorf("cannot marshal private key to PEM: %w", err)
		}
	}

	for name := range tmpl.Contexts {
		tmpl.Contexts[name].Name = contextUserName
		tmpl.Contexts[name].Context.AuthInfo = contextUserName
	}

	tmpl.CurrentContext = contextUserName

	_, err = utilities.CreateOrUpdateWithConflict(ctx, r.Client, secret, func() error {
		labels := secret.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}

		labels[kamajiv1alpha1.ManagedByLabel] = generator.Name
		labels[kamajiv1alpha1.ManagedForLabel] = tcp.Name
		labels[constants.ControllerLabelResource] = utilities.CertificateKubeconfigLabel

		secret.SetLabels(labels)

		if secret.Data == nil {
			secret.Data = make(map[string][]byte)
		}

		secret.Data["value"], err = utilities.EncodeToYaml(tmpl)
		if err != nil {
			return fmt.Errorf("cannot encode generated Kubeconfig to YAML: %w", err)
		}

		if utilities.IsRotationRequested(secret) {
			utilities.SetLastRotationTimestamp(secret)
		}

		if orErr := controllerutil.SetOwnerReference(tcp, secret, r.Client.Scheme()); orErr != nil {
			return orErr
		}

		return ctrl.SetControllerReference(tcp, secret, r.Client.Scheme())
	})
	if err != nil {
		return fmt.Errorf("cannot create or update generated Kubeconfig: %w", err)
	}

	return nil
}

func (r *KubeconfigGeneratorReconciler) isValid(secret *corev1.Secret, tmpl *clientcmdapiv1.Config, groups sets.Set[string], user string) (bool, error) {
	if utilities.IsRotationRequested(secret) {
		return false, nil
	}

	concrete, decodeErr := utilities.DecodeKubeconfig(*secret, "value")
	if decodeErr != nil {
		return false, decodeErr
	}
	// Checking Certificate Authority validity
	switch {
	case len(concrete.Clusters) != len(tmpl.Clusters):
		return false, nil
	default:
		for i := range tmpl.Clusters {
			if !bytes.Equal(tmpl.Clusters[i].Cluster.CertificateAuthorityData, concrete.Clusters[i].Cluster.CertificateAuthorityData) {
				return false, nil
			}

			if tmpl.Clusters[i].Cluster.Server != concrete.Clusters[i].Cluster.Server {
				return false, nil
			}
		}
	}

	for _, auth := range concrete.AuthInfos {
		valid, vErr := crypto.IsValidCertificateKeyPairBytes(auth.AuthInfo.ClientCertificateData, auth.AuthInfo.ClientKeyData, r.NotValidThreshold)
		if vErr != nil {
			return false, vErr
		}
		if !valid {
			return false, nil
		}

		crt, crtErr := crypto.ParseCertificateBytes(auth.AuthInfo.ClientCertificateData)
		if crtErr != nil {
			return false, crtErr
		}

		if crt.Subject.CommonName != user {
			return false, nil
		}

		if !sets.New[string](crt.Subject.Organization...).Equal(groups) {
			return false, nil
		}
	}

	return true, nil
}

func (r *KubeconfigGeneratorReconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kamajiv1alpha1.KubeconfigGenerator{}).
		WatchesRawSource(source.Channel(r.CertificateChan, handler.Funcs{GenericFunc: func(_ context.Context, genericEvent event.TypedGenericEvent[client.Object], w workqueue.TypedRateLimitingInterface[reconcile.Request]) {
			w.AddRateLimited(ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: genericEvent.Object.GetName(),
				},
			})
		}})).
		Watches(&corev1.Secret{}, handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, object client.Object) []ctrl.Request {
			if object.GetLabels() == nil {
				return nil
			}

			v, found := object.GetLabels()[kamajiv1alpha1.ManagedByLabel]
			if !found {
				return nil
			}

			return []ctrl.Request{
				{
					NamespacedName: types.NamespacedName{
						Name: v,
					},
				},
			}
		})).
		Complete(r)
}
