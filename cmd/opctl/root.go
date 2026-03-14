package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zlc-ai/opc-platform/internal/config"
)

var (
	cfgFile string
	verbose bool
	output  string
)

var rootCmd = &cobra.Command{
	Use:   "opctl",
	Short: "OPC Platform CLI - AI Agent Kubernetes",
	Long: `opctl is the command-line interface for OPC Platform.
Manage AI Agent clusters like Kubernetes manages containers.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		config.InitLogger(verbose)
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.opc/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose/debug output")
	rootCmd.PersistentFlags().StringVarP(&output, "output", "o", "table", "output format: json|yaml|table")
}

func initConfig() {
	if err := config.EnsureConfigDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not create config directory: %v\n", err)
	}

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(config.GetConfigDir())
	}

	viper.AutomaticEnv()

	// Set defaults.
	viper.SetDefault("defaultOutput", "table")
	viper.SetDefault("logLevel", "info")
	viper.SetDefault("stateDir", config.GetStateDir())

	if err := viper.ReadInConfig(); err != nil {
		// Config file not found is fine; other errors are warnings.
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "Warning: error reading config file: %v\n", err)
		}
	}
}
