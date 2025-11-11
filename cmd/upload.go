package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/parnexcodes/woof/internal/config"
	"github.com/parnexcodes/woof/internal/output"
	"github.com/parnexcodes/woof/internal/uploader"
	providerpkg "github.com/parnexcodes/woof/pkg/providers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	providers     []string
	retryAttempts int
	retryDelay    time.Duration
	progress      bool
)

var uploadCmd = &cobra.Command{
	Use:   "upload [files/directories...]",
	Short: "Upload files and directories to hosting providers",
	Long: `Upload one or more files or directories to configured file hosting providers.
Supports parallel uploads, progress tracking, and multiple output formats.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runUpload,
}

func init() {
	uploadCmd.Flags().StringSliceVarP(&providers, "providers", "p", []string{}, "specific providers to use (default: all configured)")
	uploadCmd.Flags().IntVar(&retryAttempts, "retry-attempts", 3, "number of retry attempts per file")
	uploadCmd.Flags().DurationVar(&retryDelay, "retry-delay", 2*time.Second, "delay between retry attempts")
	uploadCmd.Flags().BoolVar(&progress, "progress", true, "show upload progress")

	viper.BindPFlag("providers", uploadCmd.Flags().Lookup("providers"))
	viper.BindPFlag("retry-attempts", uploadCmd.Flags().Lookup("retry-attempts"))
	viper.BindPFlag("retry-delay", uploadCmd.Flags().Lookup("retry-delay"))
	viper.BindPFlag("progress", uploadCmd.Flags().Lookup("progress"))

	viper.SetDefault("retry-attempts", 3)
	viper.SetDefault("retry-delay", 2*time.Second)
	viper.SetDefault("progress", true)
}

func runUpload(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	// Create uploader
	upldr := uploader.NewDefaultUploader()

	// Create provider factory
	factory := providerpkg.NewFactory()

	// Get provider instances
	var providerList []uploader.Provider
	if len(providers) > 0 {
		// Use specified providers
		providerList, err = factory.CreateProvidersFromNames(providers, cfg.Providers)
	} else {
		// Use all enabled providers
		providerList, err = factory.CreateProviders(cfg.GetEnabledProviders())
	}

	if err != nil {
		return fmt.Errorf("failed to create providers: %w", err)
	}

	if len(providerList) == 0 {
		return fmt.Errorf("no providers enabled. Configure providers in %s or specify with -p flag", viper.ConfigFileUsed())
	}

	uploadConfig := uploader.UploadConfig{
		Concurrency:   viper.GetInt("concurrency"),
		Providers:     providerList,
		OutputFormat:  viper.GetString("output"),
		Verbose:       viper.GetBool("verbose"),
		RetryAttempts: cfg.Upload.RetryAttempts,
		RetryDelay:    cfg.Upload.RetryDelay,
	}

	// Start uploads
	resultCh, progressCh, err := upldr.Upload(ctx, args, uploadConfig)
	if err != nil {
		return fmt.Errorf("failed to start upload: %w", err)
	}

	// Create output handler
	outputHandler, err := output.NewHandler(viper.GetString("output"))
	if err != nil {
		return fmt.Errorf("failed to create output handler: %w", err)
	}

	// Handle progress and results
	progressConfig := loadUploadConfig()
	return handleUploadOutputs(ctx, resultCh, progressCh, outputHandler, progressConfig.Progress)
}

func loadUploadConfig() struct {
	RetryAttempts int
	RetryDelay    time.Duration
	Progress      bool
} {
	return struct {
		RetryAttempts int
		RetryDelay    time.Duration
		Progress      bool
	}{
		RetryAttempts: viper.GetInt("retry-attempts"),
		RetryDelay:    viper.GetDuration("retry-delay"),
		Progress:      viper.GetBool("progress"),
	}
}


func handleUploadOutputs(ctx context.Context, resultCh <-chan uploader.UploadResult, progressCh <-chan uploader.ProgressInfo, outputHandler output.Handler, showProgress bool) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case result, ok := <-resultCh:
			if !ok {
				return nil // All results processed
			}
			if err := outputHandler.HandleResult(result); err != nil {
				return err
			}

		case progress, ok := <-progressCh:
			if !ok || !showProgress {
				continue
			}
			if err := outputHandler.HandleProgress(progress); err != nil {
				return err
			}
		}
	}
}