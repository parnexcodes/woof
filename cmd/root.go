package cmd

import (
	"fmt"
	"os"

	"github.com/parnexcodes/woof/internal/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile     string
	verbose     bool
	concurrency int
	outputFormat string

	rootCmd = &cobra.Command{
		Use:   "woof",
		Short: "High-performance parallel file uploader",
		Long: `Woof is a high-performance CLI tool for uploading files and folders
to multiple file hosting services with support for parallel uploads,
JSON output piping, and extensive configuration options.`,
	}
)

// Execute executes the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (required to use YAML configuration)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().IntVarP(&concurrency, "concurrency", "c", 5, "maximum number of parallel uploads")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "output format (text, json)")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("concurrency", rootCmd.PersistentFlags().Lookup("concurrency"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))

	// Set default values
	viper.SetDefault("concurrency", 5)
	viper.SetDefault("output", "text")

	// Add subcommands
	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	// Only load config if explicitly specified via --config flag
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		viper.AutomaticEnv()

		if err := viper.ReadInConfig(); err == nil {
			if verbose {
				// Use logging system for config file announcement
				logging.Init(verbose, os.Stderr)
				logging.ConfigLoad(viper.ConfigFileUsed(), nil)
			}
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading config file: %v\n", err)
			os.Exit(1)
		}
	} else {
		// No config file specified - use CLI flags and defaults only
		viper.AutomaticEnv()
		if verbose {
			// Initialize logging to avoid nil pointer when no config file but verbose is set
			logging.Init(verbose, os.Stderr)
			logging.ConfigLoad("CLI flags only", nil)
		}
	}
}