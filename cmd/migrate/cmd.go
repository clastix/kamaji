// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	kamajiv1alpha1 "github.com/clastix/kamaji/api/v1alpha1"
	"github.com/clastix/kamaji/internal/datastore"
)

func NewCmd(scheme *runtime.Scheme) *cobra.Command {
	// CLI flags
	var (
		tenantControlPlane string
		targetDataStore    string
		timeout            time.Duration
	)

	cmd := &cobra.Command{
		Use:          "migrate",
		Short:        "Migrate the data of a TenantControlPlane to another compatible DataStore",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancelFn := context.WithTimeout(context.Background(), timeout)
			defer cancelFn()

			log := ctrl.Log

			log.Info("generating the controller-runtime client")

			client, err := ctrlclient.New(ctrl.GetConfigOrDie(), ctrlclient.Options{
				Scheme: scheme,
			})
			if err != nil {
				return err
			}

			parts := strings.Split(tenantControlPlane, string(types.Separator))
			if len(parts) != 2 {
				return fmt.Errorf("non well-formed namespaced name for the tenant control plane, expected <NAMESPACE>/NAME, fot %s", tenantControlPlane)
			}

			log.Info("retrieving the TenantControlPlane")

			tcp := &kamajiv1alpha1.TenantControlPlane{}
			if err = client.Get(ctx, types.NamespacedName{Namespace: parts[0], Name: parts[1]}, tcp); err != nil {
				return err
			}

			log.Info("retrieving the TenantControlPlane used DataStore")

			originDs := &kamajiv1alpha1.DataStore{}
			if err = client.Get(ctx, types.NamespacedName{Name: tcp.Status.Storage.DataStoreName}, originDs); err != nil {
				return err
			}

			log.Info("retrieving the target DataStore")

			targetDs := &kamajiv1alpha1.DataStore{}
			if err = client.Get(ctx, types.NamespacedName{Name: targetDataStore}, targetDs); err != nil {
				return err
			}

			if tcp.Status.Storage.Driver != string(targetDs.Spec.Driver) {
				return fmt.Errorf("migration between DataStore with different driver is not supported")
			}

			if tcp.Status.Storage.DataStoreName == targetDs.GetName() {
				return fmt.Errorf("cannot migrate to the same DataStore")
			}

			log.Info("generating the origin storage connection")

			originConnection, err := datastore.NewStorageConnection(ctx, client, *originDs)
			if err != nil {
				return err
			}
			defer originConnection.Close()

			log.Info("generating the target storage connection")

			targetConnection, err := datastore.NewStorageConnection(ctx, client, *targetDs)
			if err != nil {
				return err
			}
			defer targetConnection.Close()
			// Start migrating from the old Datastore to the new one
			log.Info("migration from origin to target started")

			if err = originConnection.Migrate(ctx, *tcp, targetConnection); err != nil {
				return fmt.Errorf("unable to migrate data from %s to %s: %w", originDs.GetName(), targetDs.GetName(), err)
			}

			log.Info("migration completed")

			return nil
		},
	}

	cmd.Flags().StringVar(&tenantControlPlane, "tenant-control-plane", "", "Namespaced-name of the TenantControlPlane that must be migrated (e.g.: default/test)")
	cmd.Flags().StringVar(&targetDataStore, "target-datastore", "", "Name of the Datastore to which the TenantControlPlane will be migrated")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Amount of time for the context timeout")

	_ = cmd.MarkFlagRequired("tenant-control-plane")
	_ = cmd.MarkFlagRequired("target-datastore")

	return cmd
}
