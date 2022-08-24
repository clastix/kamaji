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
	envPrefix           = "KAMAJI"
	defaultTmpDirectory = "/tmp/kamaji"
	defaultKineImage    = "rancher/kine:v0.9.2-amd64"
	defaultDataStore    = "etcd"
)

func InitConfig() (*viper.Viper, error) {
	config = viper.New()

	// Setup flags with standard "flag" module
	flag.StringVar(&cfgFile, "config-file", "kamaji.yaml", "Configuration file alternative.")
	flag.String("metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.Bool("leader-elect", false, "Enable leader election for controller manager. "+
		"Enabling this will ensure there is only one active controller manager.")
	flag.String("tmp-directory", defaultTmpDirectory, "Directory which will be used to work with temporary files.")
	flag.String("kine-image", defaultKineImage, "Container image along with tag to use for the Kine sidecar container (used only if etcd-storage-type is set to one of kine strategies)")
	flag.String("datastore", defaultDataStore, "The default DataStore that should be used by Kamaji to setup the required storage")

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
	if err := config.BindEnv("tmp-directory", fmt.Sprintf("%s_TMP_DIRECTORY", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("kine-image", fmt.Sprintf("%s_KINE_IMAGE", envPrefix)); err != nil {
		return nil, err
	}
	if err := config.BindEnv("datastore", fmt.Sprintf("%s_DATASTORE", envPrefix)); err != nil {
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
