// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"flag"
	"fmt"
	"log"

	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	config  *viper.Viper
	cfgFile string
)

const (
	envPrefix                        = "KAMAJI"
	defaultETCDStorageType           = "etcd"
	defaultETCDCASecretName          = "etcd-certs"
	defaultETCDCASecretNamespace     = "kamaji-system"
	defaultETCDEndpoints             = "etcd-server:2379"
	defaultETCDCompactionInterval    = "0"
	defaultETCDClientSecretName      = "root-client-certs"
	defaultETCDClientSecretNamespace = "kamaji-system"
	defaultTmpDirectory              = "/tmp/kamaji"
	defaultKineMySQLSecretName       = "mysql-config"
	defaultKineMySQLSecretNamespace  = "kamaji-system"
	defaultKineMySQLHost             = "localhost"
	defaultKineMySQLPort             = 3306
	defaultKineImage                 = "rancher/kine:v0.9.2-amd64"
)

func InitConfig() (*viper.Viper, error) {
	config = viper.New()

	// Setup flags with standard "flag" module
	flag.StringVar(&cfgFile, "config-file", "kamaji.yaml", "Configuration file alternative.")
	flag.String("metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.Bool("leader-elect", false, "Enable leader election for controller manager. "+
		"Enabling this will ensure there is only one active controller manager.")
	flag.String("etcd-storage-type", defaultETCDStorageType, "Type of storage for ETCD (i.e etcd, kine-mysql, kine-postgres)")
	flag.String("etcd-ca-secret-name", defaultETCDCASecretName, "Name of the secret which contains CA's certificate and private key.")
	flag.String("etcd-ca-secret-namespace", defaultETCDCASecretNamespace, "Namespace of the secret which contains CA's certificate and private key.")
	flag.String("etcd-client-secret-name", defaultETCDClientSecretName, "Name of the secret which contains ETCD client certificates")
	flag.String("etcd-client-secret-namespace", defaultETCDClientSecretNamespace, "Name of the namespace where the secret which contains ETCD client certificates is")
	flag.String("etcd-endpoints", defaultETCDEndpoints, "Comma-separated list with ETCD endpoints (i.e. https://etcd-0.etcd.kamaji-system.svc.cluster.local,https://etcd-1.etcd.kamaji-system.svc.cluster.local,https://etcd-2.etcd.kamaji-system.svc.cluster.local)")
	flag.String("etcd-compaction-interval", defaultETCDCompactionInterval, "ETCD Compaction interval (i.e. \"5m0s\"). (default: \"0\" (disabled))")
	flag.String("tmp-directory", defaultTmpDirectory, "Directory which will be used to work with temporary files.")
	flag.String("kine-mysql-secret-name", defaultKineMySQLSecretName, "Name of the secret which contains MySQL (Kine) configuration.")
	flag.String("kine-mysql-secret-namespace", defaultKineMySQLSecretNamespace, "Name of the namespace where the secret which contains MySQL (Kine) configuration.")
	flag.String("kine-mysql-host", defaultKineMySQLHost, "Host where MySQL (Kine) is working")
	flag.Int("kine-mysql-port", defaultKineMySQLPort, "Port where MySQL (Kine) is working")
	flag.String("kine-image", defaultKineImage, "Container image along with tag to use for the Kine sidecar container (used only if etcd-storage-type is set to one of kine strategies)")

	// Setup zap configuration
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Add flag set to pflag in order to parse with viper
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	if err := config.BindPFlags(pflag.CommandLine); err != nil {
		log.Printf("error binding flags: %s", err)

		return nil, err
	}

	// Setup environment variables
	if err := config.BindEnv("metrics-bind-address", fmt.Sprintf("%s_METRICS_BIND_ADDRESS", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("health-probe-bind-address", fmt.Sprintf("%s_HEALTH_PROBE_BIND_ADDRESS", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("leader-elect", fmt.Sprintf("%s_LEADER_ELECTION", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("etcd-storage-type", fmt.Sprintf("%s_ETCD_STORAGE_TYPE", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("etcd-ca-secret-name", fmt.Sprintf("%s_ETCD_CA_SECRET_NAME", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("etcd-ca-secret-namespace", fmt.Sprintf("%s_ETCD_CA_SECRET_NAMESPACE", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("etcd-client-secret-name", fmt.Sprintf("%s_ETCD_CLIENT_SECRET_NAME", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("etcd-client-secret-namespace", fmt.Sprintf("%s_ETCD_CLIENT_SECRET_NAMESPACE", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("etcd-endpoints", fmt.Sprintf("%s_ETCD_ENDPOINTS", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("etcd-compaction-interval", fmt.Sprintf("%s_ETCD_COMPACTION_INTERVAL", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("tmp-directory", fmt.Sprintf("%s_TMP_DIRECTORY", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("kine-mysql-secret-name", fmt.Sprintf("%s_KINE_MYSQL_SECRET_NAME", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("kine-mysql-secret-namespace", fmt.Sprintf("%s_KINE_MYSQL_SECRET_NAMESPACE", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("kine-mysql-host", fmt.Sprintf("%s_KINE_MYSQL_HOST", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("kine-mysql-port", fmt.Sprintf("%s_KINE_MYSQL_PORT", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("kine-image", fmt.Sprintf("%s_KINE_IMAGE", envPrefix)); err != nil {
		return nil, err
	}

	// Setup config file
	if cfgFile != "" {
		// Using flag-passed config file
		config.SetConfigFile(cfgFile)
	} else {
		// Using default config file
		config.AddConfigPath(".")
		config.SetConfigName("kamaji")
	}
	config.SetConfigType("yaml")
	if err := config.ReadInConfig(); err != nil {
		if errors.Is(err, &viper.ConfigParseError{}) {
			log.Printf("error parsing config file: %v", err)

			return nil, err
		}
		log.Println("No config file used")

		return nil, err
	}

	log.Printf("Using config file: %v", viper.ConfigFileUsed())

	return config, nil
}
