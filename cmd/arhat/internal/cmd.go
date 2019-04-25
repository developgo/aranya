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
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/client"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtime"
)

const DefaultConfigFile = "/etc/arhat/config.yaml"

var configFile string

// Config for arhat
type Config struct {
	Agent        client.Config       `json:"agent" yaml:"agent"`
	Connectivity connectivity.Config `json:"connectivity" yaml:"connectivity"`
	Runtime      runtime.Config      `json:"runtime" yaml:"runtime"`
}

func NewArhatCmd() *cobra.Command {
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

	cmd.PersistentFlags().StringVarP(&configFile, "config", "c", DefaultConfigFile, "set the path to configuration file")
	_ = cmd.MarkPersistentFlagRequired("config")

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
