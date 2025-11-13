package providers

import (
	"fmt"
	"strings"

	"github.com/parnexcodes/woof/internal/config"
	"github.com/parnexcodes/woof/internal/logging"
	"github.com/parnexcodes/woof/internal/uploader"
	providerpkg "github.com/parnexcodes/woof/internal/providers"
	"github.com/parnexcodes/woof/pkg/providers/buzzheavier"
	"github.com/parnexcodes/woof/pkg/providers/gofile"
)

// Factory creates provider instances based on configuration
type Factory struct {
	wrapperConfig providerpkg.WrapperConfig
}

// FactoryConfig holds configuration for the factory
type FactoryConfig struct {
	EnableConsistencyWrapper bool                       `json:"enable_consistency_wrapper"`
	WrapperConfig            providerpkg.WrapperConfig    `json:"wrapper_config"`
}

// DefaultFactoryConfig returns sensible defaults for factory configuration
func DefaultFactoryConfig() FactoryConfig {
	return FactoryConfig{
		EnableConsistencyWrapper: true,
		WrapperConfig: providerpkg.DefaultWrapperConfig(),
	}
}

// NewFactory creates a new provider factory
func NewFactory() *Factory {
	return &Factory{
		wrapperConfig: providerpkg.DefaultWrapperConfig(),
	}
}

// NewFactoryWithConfig creates a new provider factory with custom configuration
func NewFactoryWithConfig(config FactoryConfig) *Factory {
	return &Factory{
		wrapperConfig: config.WrapperConfig,
	}
}

// CreateProvider creates a provider instance from configuration
func (f *Factory) CreateProvider(providerConfig config.ProviderConfig) (uploader.Provider, error) {
	return f.CreateProviderWithWrapper(providerConfig, DefaultFactoryConfig().EnableConsistencyWrapper)
}

// CreateProviderWithWrapper creates a provider with optional consistency wrapper
func (f *Factory) CreateProviderWithWrapper(providerConfig config.ProviderConfig, enableWrapper bool) (uploader.Provider, error) {
	logging.ProviderConfig(providerConfig.Name, providerConfig.Settings)

	// Create the base provider
	var provider uploader.Provider
	var err error

	switch strings.ToLower(providerConfig.Name) {
	case "buzzheavier":
		provider, err = buzzheavier.New(providerConfig.Settings)
		if err != nil {
			logging.ErrorContext("provider_creation", err, map[string]interface{}{
				"provider": providerConfig.Name,
				"settings": providerConfig.Settings,
			})
			return nil, fmt.Errorf("failed to create provider '%s': %w", providerConfig.Name, err)
		}
	case "gofile":
		provider, err = gofile.New(providerConfig.Settings)
		if err != nil {
			logging.ErrorContext("provider_creation", err, map[string]interface{}{
				"provider": providerConfig.Name,
				"settings": providerConfig.Settings,
			})
			return nil, fmt.Errorf("failed to create provider '%s': %w", providerConfig.Name, err)
		}
	default:
		err := fmt.Errorf("unknown provider: %s", providerConfig.Name)
		logging.ErrorContext("provider_creation", err, map[string]interface{}{
			"provider": providerConfig.Name,
		})
		return nil, err
	}

	// Apply consistency wrapper if enabled
	if enableWrapper {
		logging.ProviderConfig(provider.Name(), map[string]interface{}{
			"wrapper_enabled":         true,
			"validation_enabled":      f.wrapperConfig.PreUploadValidation,
			"auto_retry_enabled":      f.wrapperConfig.AutoRetry,
			"max_retries":             f.wrapperConfig.MaxRetries,
		})
		provider = providerpkg.NewConsistencyWrapper(provider, f.wrapperConfig)
	}

	return provider, nil
}

// CreateProviders creates multiple provider instances from configuration
func (f *Factory) CreateProviders(providerConfigs []config.ProviderConfig) ([]uploader.Provider, error) {
	return f.CreateProvidersWithWrapper(providerConfigs, DefaultFactoryConfig().EnableConsistencyWrapper)
}

// CreateProvidersWithWrapper creates multiple providers with optional consistency wrapper
func (f *Factory) CreateProvidersWithWrapper(providerConfigs []config.ProviderConfig, enableWrapper bool) ([]uploader.Provider, error) {
	var providers []uploader.Provider

	for _, providerConfig := range providerConfigs {
		if !providerConfig.Enabled {
			logging.ProviderConfig(providerConfig.Name, map[string]interface{}{"enabled": false})
			continue
		}

		provider, err := f.CreateProviderWithWrapper(providerConfig, enableWrapper)
		if err != nil {
			return nil, err
		}

		providers = append(providers, provider)
	}

	return providers, nil
}

// CreateProvidersFromNames creates providers for a specific list of provider names
func (f *Factory) CreateProvidersFromNames(providerNames []string, allConfigs []config.ProviderConfig) ([]uploader.Provider, error) {
	nameSet := make(map[string]bool)
	for _, name := range providerNames {
		nameSet[strings.ToLower(name)] = true
	}

	var selectedConfigs []config.ProviderConfig
	for _, config := range allConfigs {
		if nameSet[strings.ToLower(config.Name)] {
			selectedConfigs = append(selectedConfigs, config)
			delete(nameSet, strings.ToLower(config.Name))
		}
	}

	// Check if any requested providers were not found
	if len(nameSet) > 0 {
		var missing []string
		for name := range nameSet {
			missing = append(missing, name)
		}
		return nil, fmt.Errorf("unknown providers: %v", missing)
	}

	return f.CreateProviders(selectedConfigs)
}

// CreateAllProviders creates all available providers with consistency wrapper enabled
func (f *Factory) CreateAllProviders() ([]uploader.Provider, error) {
	return f.CreateAllProvidersWithWrapper(DefaultFactoryConfig().EnableConsistencyWrapper)
}

// CreateAllProvidersWithWrapper creates all available providers with optional consistency wrapper
func (f *Factory) CreateAllProvidersWithWrapper(enableWrapper bool) ([]uploader.Provider, error) {
	// Define all available providers with default settings
	var providers []uploader.Provider

	// BuzzHeavier provider with default settings
	logging.ProviderConfig("buzzheavier", map[string]interface{}{"mode": "all_providers_defaults"})
	buzzProvider, err := buzzheavier.New(map[string]interface{}{})
	if err != nil {
		logging.ErrorContext("create_all_providers", err, map[string]interface{}{
			"provider": "buzzheavier",
		})
		return nil, fmt.Errorf("failed to create buzzheavier provider: %w", err)
	}

	// Apply consistency wrapper if enabled
	if enableWrapper {
		logging.ProviderConfig(buzzProvider.Name(), map[string]interface{}{
			"wrapper_enabled":         true,
			"validation_enabled":      f.wrapperConfig.PreUploadValidation,
			"auto_retry_enabled":      f.wrapperConfig.AutoRetry,
			"max_retries":             f.wrapperConfig.MaxRetries,
		})
		providers = append(providers, providerpkg.NewConsistencyWrapper(buzzProvider, f.wrapperConfig))
	} else {
		providers = append(providers, buzzProvider)
	}

	// GoFile provider with default settings
	logging.ProviderConfig("gofile", map[string]interface{}{"mode": "all_providers_defaults"})
	gofileProvider, err := gofile.New(map[string]interface{}{})
	if err != nil {
		logging.ErrorContext("create_all_providers", err, map[string]interface{}{
			"provider": "gofile",
		})
		return nil, fmt.Errorf("failed to create gofile provider: %w", err)
	}

	// Apply consistency wrapper if enabled
	if enableWrapper {
		logging.ProviderConfig(gofileProvider.Name(), map[string]interface{}{
			"wrapper_enabled":         true,
			"validation_enabled":      f.wrapperConfig.PreUploadValidation,
			"auto_retry_enabled":      f.wrapperConfig.AutoRetry,
			"max_retries":             f.wrapperConfig.MaxRetries,
		})
		providers = append(providers, providerpkg.NewConsistencyWrapper(gofileProvider, f.wrapperConfig))
	} else {
		providers = append(providers, gofileProvider)
	}

	return providers, nil
}