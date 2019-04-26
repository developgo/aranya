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
	"arhat.dev/aranya/pkg/constant"
	aranyaController "arhat.dev/aranya/pkg/controller"
	"arhat.dev/aranya/pkg/virtualnode"
)

var configFile string

type ControllerConfig struct {
	Log struct {
		Level int    `json:"level" yaml:"level"`
		Dir   string `json:"dir" yaml:"dir"`
	} `json:"log" yaml:"log"`
}

type ServicesConfig struct {
	MetricsService struct {
		Address string `json:"address" yaml:"address"`
		Port    int32  `json:"port" yaml:"port"`
	} `json:"metrics" yaml:"metrics"`
}

type Config struct {
	Controller  ControllerConfig   `json:"controller" yaml:"controller"`
	VirtualNode virtualnode.Config `json:"virtualnode" yaml:"virtualnode"`
	Services    ServicesConfig     `json:"services" yaml:"services"`
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

	flags := cmd.PersistentFlags()
	// config file
	flags.StringVarP(&configFile, "config", "c", constant.DefaultAranyaConfigFile, "path to the aranya config file")
	// virtual node flags
	flags.DurationVar(&config.VirtualNode.Pod.Timers.ReSyncInterval, "pod-resync-interval", constant.DefaultPodReSyncInterval, "pod informer resync interval")
	flags.IntVar(&config.VirtualNode.Node.StatusUpdateRetryCount, "node-status-update-retry-count", constant.DefaultNodeStatusUpdateRetry, "retry count when node object status update fail")
	flags.DurationVar(&config.VirtualNode.Node.Timers.StatusSyncInterval, "node-status-sync-interval", constant.DefaultNodeStatusSyncInterval, "cluster node status update interval")
	flags.DurationVar(&config.VirtualNode.Stream.Timers.CreationTimeout, "stream-creation-timeout", constant.DefaultStreamCreationTimeout, "kubectl stream creation timeout (exec, attach, port-forward)")
	flags.DurationVar(&config.VirtualNode.Stream.Timers.IdleTimeout, "stream-idle-timeout", constant.DefaultStreamIdleTimeout, "kubectl stream idle timeout (exec, attach, port-forward)")
	flags.DurationVar(&config.VirtualNode.Connectivity.Timers.UnarySessionTimeout, "unary-session-timeout", constant.DefaultUnarySessionTimeout, "timeout duration for unary session")
	flags.DurationVar(&config.VirtualNode.Connectivity.Timers.ForcePodStatusSyncInterval, "force-pod-status-sync-interval", 0, "device pod status sync interval, reject device if operation failed")
	flags.DurationVar(&config.VirtualNode.Connectivity.Timers.ForceNodeStatusSyncInterval, "force-node-status-sync-interval", 0, "device node status sync interval, reject device if operation failed")
	// controller flags
	flags.IntVarP(&config.Controller.Log.Level, "log-level", "v", 0, "log level, higher level means more verbose")
	flags.StringVar(&config.Controller.Log.Dir, "log-dir", constant.DefaultAranyaLogDir, "save log files to this dir")
	// services flags
	flags.StringVar(&config.Services.MetricsService.Address, "metrics-service-address", "0.0.0.0", "")
	flags.Int32Var(&config.Services.MetricsService.Port, "metrics-service-port", 8383, "")

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
	if config.Services.MetricsService.Address != "" {
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
	if err := aranyaController.AddToManager(mgr, &config.VirtualNode); err != nil {
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
