package internal

import (
	"arhat.dev/aranya/pkg/node/connectivity"
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	cmdConnectivity "arhat.dev/aranya/cmd/arhat/internal/connectivity"
	cmdRuntime "arhat.dev/aranya/cmd/arhat/internal/runtime"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
)

const (
	DefaultConfigFile = "/etc/arhat/config.yml"
)

var (
	configFile string
)

type Config struct {
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
	rt, err := cmdRuntime.GetRuntime(ctx, &config.Runtime)
	if err != nil {
		return fmt.Errorf("create runtime failed: %v", err)
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
		client, err := cmdConnectivity.GetConnectivityClient(ctx, &config.Connectivity, rt)
		if err != nil {
			// TODO: log error
		}

		if err := client.Run(ctx); err != nil {

		}
	}
}
