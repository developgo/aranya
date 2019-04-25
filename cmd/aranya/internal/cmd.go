/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	clientConfig "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	aranyaApis "arhat.dev/aranya/pkg/apis"
	aranyaController "arhat.dev/aranya/pkg/controller"
)

const DefaultConfigFile = "/etc/aranya/config.yaml"

var configFile string

type Config struct {
	Controller aranyaController.Config `json:"controller" yaml:"controller"`
	Services   struct {
		MetricsService *struct {
			Address string `json:"address" yaml:"address"`
			Port    int32  `json:"port" yaml:"port"`
		} `json:"metrics" yaml:"metrics"`
	} `json:"services" yaml:"services"`
}

var log = logf.Log.WithName("cmd")

func NewAranyaCmd() *cobra.Command {
	ctx, exit := context.WithCancel(context.Background())
	config := &Config{}

	cmd := &cobra.Command{
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			configBytes, err := ioutil.ReadFile(configFile)
			if err != nil {
				return fmt.Errorf("failed to read config file %s: %v", configFile, err)
			}

			if err := yaml.Unmarshal(configBytes, config); err != nil {
				return fmt.Errorf("failed to unmarshal config file %s: %v", configFile, err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(ctx, config)
		},
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		exitCount := 0
		for sig := range sigCh {
			switch sig {
			case os.Interrupt, syscall.SIGTERM:
				exitCount++
				if exitCount == 1 {
					exit()
				} else {
					os.Exit(1)
				}
			case syscall.SIGHUP:
				// force reload
			}
		}
	}()

	cmd.PersistentFlags().StringVarP(&configFile, "config", "c", DefaultConfigFile, "set the path to configuration file")

	return cmd
}

func run(ctx context.Context, config *Config) error {
	logf.SetLogger(zap.Logger())
	printVersion()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		return err
	}

	// Get a config to talk to the api-server
	kubeConfig, err := clientConfig.GetConfig()
	if err != nil {
		return err
	}

	// Become the leader before proceeding
	err = leader.Become(ctx, "aranya-lock")
	if err != nil {
		return err
	}

	metricsAddress := ""
	if config.Services.MetricsService != nil {
		metricsAddress = fmt.Sprintf("%s:%d", config.Services.MetricsService.Address, config.Services.MetricsService.Port)

		// Create Service object to expose the metrics port.
		_, err = metrics.ExposeMetricsPort(ctx, config.Services.MetricsService.Port)
		if err != nil {
			log.Info(err.Error())
		}
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(kubeConfig, manager.Options{
		Namespace:          namespace,
		MetricsBindAddress: metricsAddress,
	})
	if err != nil {
		return err
	}

	log.Info("registering components")

	// Setup Scheme for all resources
	if err := aranyaApis.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	// Setup all Controllers
	if err := aranyaController.AddToManager(mgr); err != nil {
		return err
	}

	log.Info("starting the controller")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		return err
	}

	return nil
}

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("Version of operator-sdk: %v", sdkVersion.Version))
}
