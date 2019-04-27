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
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	internalclient "arhat.dev/aranya/cmd/arhat/internal/client"
	internalruntime "arhat.dev/aranya/cmd/arhat/internal/runtime"
	"arhat.dev/aranya/pkg/connectivity/client"
	"arhat.dev/aranya/pkg/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/constant"
)

var configFile string

// Config for arhat
type Config struct {
	Agent        client.AgentConfig        `json:"agent" yaml:"agent"`
	Connectivity client.ConnectivityConfig `json:"connectivity" yaml:"connectivity"`
	Runtime      runtime.Config            `json:"runtime" yaml:"runtime"`
}

func NewArhatCmd() *cobra.Command {
	ctx, exit := context.WithCancel(context.Background())
	config := &Config{}

	cmd := &cobra.Command{
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.ParseFlags(args); err != nil {
				return err
			}

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
	flags.StringVarP(&configFile, "config", "c", constant.DefaultArhatConfigFile, "path to the arhat config file")
	// agent flags
	flags.StringVar(&config.Agent.Log.Dir, "log-dir", constant.DefaultArhatLogDir, "save log files to this dir")
	flags.IntVarP(&config.Agent.Log.Level, "log-level", "v", 0, "log level, higher level means more verbose")
	flags.BoolVar(&config.Agent.Features.AllowHostExec, "allow-host-exec", false, "allow kubectl exec issued commands to execute in host")
	flags.BoolVar(&config.Agent.Features.AllowHostAttach, "allow-host-exec", false, "allow kubectl attach to host tty")
	flags.BoolVar(&config.Agent.Features.AllowHostLog, "allow-host-log", false, "allow kubectl to read arhat's log")
	flags.BoolVar(&config.Agent.Features.AllowHostPortForward, "allow-host-port-forward", false, "allow kubectl to port-forward to host")
	flags.DurationVar(&config.Agent.Node.Timers.StatusSyncInterval, "node-status-sync-interval", 0, "periodically sync node status")
	flags.DurationVar(&config.Agent.Pod.Timers.StatusSyncInterval, "pod-status-sync-interval", 0, "periodically sync node status")
	flags.IntVar(&config.Agent.Pod.MaxPodCount, "max-pod-count", 1, "max allowed pods running in this device")
	// runtime flags
	flags.StringVar(&config.Runtime.DataDir, "data-dir", constant.DefaultArhatDataDir, "pod data dir, store values from Kubernetes ConfigMap and Secret")
	flags.StringVar(&config.Runtime.PauseImage, "pause-image", constant.DefaultPauseImage, "pause container image to claim linux namespaces")
	flags.StringVar(&config.Runtime.PauseCommand, "pause-command", constant.DefaultPauseCommand, "pause container command")
	flags.StringVarP(&config.Runtime.ManagementNamespace, "management-namespace", "n", constant.DefaultManagementNamespace, "container runtime namespace for container management")
	// commandline flags are not sufficient to run arhat properly,
	// so mark config flag as required
	_ = cmd.MarkPersistentFlagRequired("config")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, os.Kill, syscall.SIGHUP)
	go func() {
		for sig := range sigCh {
			switch sig {
			case os.Interrupt, os.Kill:
				// cleanup
				exit()
			case syscall.SIGHUP:
				// force reload
			}
		}
	}()

	return cmd
}

func run(ctx context.Context, config *Config) error {
	rt, err := internalruntime.New(ctx, &config.Runtime)
	if err != nil {
		return fmt.Errorf("failed to get runtime: %v", err)
	}

	exiting := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}

	for !exiting() {
		ag, err := internalclient.New(ctx, &config.Agent, &config.Connectivity, rt)
		if err != nil {
			log.Printf("failed to get connectivity client: %v", err)
			return err
		}

		if err = ag.Start(ctx); err != nil {
			log.Printf("failed to run sync loop: %v", err)
			return err
		}

		// TODO: backoff
		time.Sleep(time.Second)
	}

	return nil
}
