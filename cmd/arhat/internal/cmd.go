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
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
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
	flags.BoolVar(&config.Agent.Features.AllowHostAttach, "allow-host-attach", false, "allow kubectl attach to host tty")
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
	// connectivity flags
	flags.Float64Var(&config.Connectivity.BackoffFactor, "backoff-factor", 1.5, "backoff factor to apply")
	flags.DurationVar(&config.Connectivity.InitialBackoff, "initial-backoff", time.Second, "initial backoff when connectivity failure")
	flags.DurationVar(&config.Connectivity.MaxBackoff, "max-backoff", 30*time.Second, "max backoff when connectivity failure")
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

	wait, maxWait, factor := config.Connectivity.InitialBackoff, config.Connectivity.MaxBackoff, config.Connectivity.BackoffFactor
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for !exiting() {
		// connect to aranya
		ag, err := internalclient.New(ctx, &config.Agent, &config.Connectivity, rt)
		if err != nil {
			log.Printf("failed to get connectivity client: %v", err)
		} else {
			// start to sync
			if err = ag.Start(ctx); err != nil {
				log.Printf("failed to communicate with aranya: %v", err)
			}
		}

		if err != nil {
			// backoff when error happened
			timer.Reset(wait)
			select {
			case <-ctx.Done():
				return nil
			case <-timer.C:
			}

			wait = time.Duration(float64(wait) * factor)
			if wait > maxWait {
				wait = maxWait
			}
		} else {
			// reset backoff
			wait = config.Connectivity.InitialBackoff
		}
	}

	return nil
}
