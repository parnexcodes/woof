package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/parnexcodes/woof/internal/config"
	"github.com/parnexcodes/woof/internal/logging"
	"github.com/parnexcodes/woof/internal/output"
	"github.com/parnexcodes/woof/internal/uploader"
	providerpkg "github.com/parnexcodes/woof/pkg/providers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	providers     []string
	useAll        bool
	files         []string
	folders       []string
	retryAttempts int
	retryDelay    time.Duration
	progress      bool
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload files and directories to hosting providers",
	Long: `Upload files and directories to configured file hosting providers.
Supports parallel uploads, progress tracking, and multiple output formats.

Use --file/-f for files and --folder/-d for directories. Supports glob patterns for files.`,
	Args: cobra.NoArgs,
	RunE: runUpload,
}

func init() {
	uploadCmd.Flags().StringSliceVarP(&providers, "providers", "p", []string{}, "specific providers to use")
	uploadCmd.Flags().BoolVar(&useAll, "all", false, "use all available providers regardless of configuration")
	uploadCmd.Flags().StringSliceVarP(&files, "file", "f", []string{}, "files to upload (can be used multiple times, supports glob patterns)")
	uploadCmd.Flags().StringSliceVarP(&folders, "folder", "d", []string{}, "folders to upload (can be used multiple times)")
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

// expandGlobPatterns expands glob patterns in file paths and returns all matched files
func expandGlobPatterns(filePatterns []string) ([]string, error) {
	var result []string
	for _, pattern := range filePatterns {
		if strings.Contains(pattern, "*") || strings.Contains(pattern, "?") || strings.Contains(pattern, "[") {
			// Handle glob patterns
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid glob pattern '%s': %w", pattern, err)
			}
			result = append(result, matches...)
		} else {
			// Direct file path
			result = append(result, pattern)
		}
	}
	return result, nil
}

// validatePaths validates that file paths are actually files and folder paths are directories
func validatePaths(files []string, folders []string) error {
	for _, file := range files {
		if info, err := os.Stat(file); err != nil {
			if os.IsNotExist(err) {
				logging.FileValidation(file, "file_existence", fmt.Errorf("file does not exist"))
				return fmt.Errorf("file does not exist: %s", file)
			}
			logging.FileValidation(file, "file_check", err)
			return fmt.Errorf("error checking file %s: %w", file, err)
		} else if info.IsDir() {
			logging.FileValidation(file, "file_type", fmt.Errorf("path is directory"))
			return fmt.Errorf("path '%s' is a directory, but --file flag requires a file. Use --folder/-d for directories", file)
		} else {
			logging.FileValidation(file, "file_check", nil)
		}
	}

	for _, folder := range folders {
		if info, err := os.Stat(folder); err != nil {
			if os.IsNotExist(err) {
				logging.FileValidation(folder, "folder_existence", fmt.Errorf("directory does not exist"))
				return fmt.Errorf("directory does not exist: %s", folder)
			}
			logging.FileValidation(folder, "folder_check", err)
			return fmt.Errorf("error checking directory %s: %w", folder, err)
		} else if !info.IsDir() {
			logging.FileValidation(folder, "folder_type", fmt.Errorf("path is file"))
			return fmt.Errorf("path '%s' is a file, but --folder/-d flag requires a directory. Use --file/-f for files", folder)
		} else {
			logging.FileValidation(folder, "folder_check", nil)
		}
	}

	return nil
}

func runUpload(cmd *cobra.Command, args []string) error {
	// Initialize logging system with verbose flag
	logging.Init(viper.GetBool("verbose"), os.Stderr)

	// Validate flags
	if len(files) == 0 && len(folders) == 0 {
		return fmt.Errorf("no files or folders specified. Use --file/-f for files or --folder/-d for directories")
	}

	logging.FlagProcessing("files", len(files))
	logging.FlagProcessing("folders", len(folders))

	// Expand glob patterns for files
	expandedFiles, err := expandGlobPatterns(files)
	if err != nil {
		return err
	}

	// Validate paths
	if err := validatePaths(expandedFiles, folders); err != nil {
		return err
	}

	// Combine all paths for the uploader
	paths := append(expandedFiles, folders...)

	// Load configuration
	configSource := "CLI flags only"
	if viper.ConfigFileUsed() != "" {
		configSource = viper.ConfigFileUsed()
	}
	logging.ConfigLoad(configSource, nil)

	cfg, err := config.LoadConfig()
	if err != nil {
		logging.ErrorContext("config_load", err, map[string]interface{}{
			"source": configSource,
		})
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	logging.ConfigLoad("effective_values", map[string]interface{}{
		"concurrency": cfg.Concurrency,
		"verbose":     cfg.Verbose,
		"output":      cfg.Output,
		"providers_count": len(cfg.Providers),
	})

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

	// Get provider instances using the new hierarchy
	var providerList []uploader.Provider
	var providerMode string
	var providerNames []string

	if useAll {
		// Use all available providers regardless of configuration
		providerList, err = factory.CreateAllProviders()
		providerMode = "all"
	} else if len(providers) > 0 {
		// Use specified providers
		providerList, err = factory.CreateProvidersFromNames(providers, cfg.Providers)
		providerMode = "specified"
		providerNames = providers
	} else {
		// Use all enabled providers from configuration
		providerList, err = factory.CreateProviders(cfg.GetEnabledProviders())
		providerMode = "enabled"
	}

	if err != nil {
		return fmt.Errorf("failed to create providers: %w", err)
	}

	// Extract provider names for debug output
	for _, provider := range providerList {
		providerNames = append(providerNames, provider.Name())
	}

	logging.ProviderSelection(providerMode, providerNames)

	if len(providerList) == 0 {
		var helpMsg strings.Builder
		helpMsg.WriteString("no providers available. Options:\n")
		helpMsg.WriteString("  1. Use --all to try all available providers\n")
		helpMsg.WriteString("  2. Specify providers with --providers/-p flag\n")
		if viper.ConfigFileUsed() != "" {
			helpMsg.WriteString(fmt.Sprintf("  3. Configure providers in %s\n\n", viper.ConfigFileUsed()))
			helpMsg.WriteString("Example:\n  woof upload --all -f file.txt\n  woof upload --providers buzzheavier -d ./folder")
		} else {
			helpMsg.WriteString("  3. Configure providers in config file\n\n")
			helpMsg.WriteString("Example:\n  woof upload --all -f file.txt\n  woof upload --providers buzzheavier -d ./folder")
		}
		return fmt.Errorf("%s", helpMsg.String())
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
	resultCh, progressCh, err := upldr.Upload(ctx, paths, uploadConfig)
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