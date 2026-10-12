package cli

import (
	"fmt"
	"os"

	"github.com/srikarmaddena/logdiff/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	noColor bool
)

var rootCmd = &cobra.Command{
	Use:     "logdiff",
	Short:   "High-speed log differ for CI regression testing",
	Version: version.Version,
	Long: `logdiff ingests two log files, masks dynamic tokens, clusters log events
by pattern, and surfaces structural regressions between CI runs.`,
}

// Execute is the entry point called from main.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default $HOME/.logdiff.yaml)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colour output")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
		}
		viper.SetConfigType("yaml")
		viper.SetConfigName(".logdiff")
	}
	viper.AutomaticEnv()
	viper.ReadInConfig() //nolint:errcheck
}
