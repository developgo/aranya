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
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

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

func NewAranyaCmd() *cobra.Command {
	ctx, exit := context.WithCancel(context.Background())
	config := &Config{}

	cmd := &cobra.Command{
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			configBytes, err := ioutil.ReadFile(configFile)
			if err != nil {
				return fmt.Errorf("failed to read config file %s: %v", configFile, err)
			}

			if err := yaml.Unmarshal(configBytes, config); err != nil {
				return fmt.Errorf("failed to unmarshal config file %s: %v", configFile, err)
			}

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
	// Get a config to talk to the api-server
	kubeConfig, err := rest.InClusterConfig()
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
			log.Printf(err.Error())
		}
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(kubeConfig, manager.Options{
		Namespace:          constant.WatchNamespace(),
		MetricsBindAddress: metricsAddress,
	})
	if err != nil {
		return err
	}

	// Setup Scheme for all resources
	if err := aranyaApis.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}

	// Setup all Controllers
	if err := aranyaController.AddControllersToManager(mgr, &config.VirtualNode); err != nil {
		return err
	}

	log.Printf("starting the controller")
	if err := mgr.Start(ctx.Done()); err != nil {
		return err
	}

	return nil
}
