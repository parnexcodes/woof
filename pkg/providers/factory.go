package providers

import (
	"fmt"
	"strings"

	"github.com/parnexcodes/woof/internal/config"
	"github.com/parnexcodes/woof/internal/logging"
	"github.com/parnexcodes/woof/internal/uploader"
	"github.com/parnexcodes/woof/pkg/providers/buzzheavier"
)

// Factory creates provider instances based on configuration
type Factory struct{}

// NewFactory creates a new provider factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateProvider creates a provider instance from configuration
func (f *Factory) CreateProvider(providerConfig config.ProviderConfig) (uploader.Provider, error) {
	logging.ProviderConfig(providerConfig.Name, providerConfig.Settings)

	switch strings.ToLower(providerConfig.Name) {
	case "buzzheavier":
		provider, err := buzzheavier.New(providerConfig.Settings)
		if err != nil {
			logging.ErrorContext("provider_creation", err, map[string]interface{}{
				"provider": providerConfig.Name,
				"settings": providerConfig.Settings,
			})
			return nil, fmt.Errorf("failed to create provider '%s': %w", providerConfig.Name, err)
		}
		return provider, nil
	default:
		err := fmt.Errorf("unknown provider: %s", providerConfig.Name)
		logging.ErrorContext("provider_creation", err, map[string]interface{}{
			"provider": providerConfig.Name,
		})
		return nil, err
	}
}

// CreateProviders creates multiple provider instances from configuration
func (f *Factory) CreateProviders(providerConfigs []config.ProviderConfig) ([]uploader.Provider, error) {
	var providers []uploader.Provider

	for _, providerConfig := range providerConfigs {
		if !providerConfig.Enabled {
			logging.ProviderConfig(providerConfig.Name, map[string]interface{}{"enabled": false})
			continue
		}

		provider, err := f.CreateProvider(providerConfig)
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

// CreateAllProviders creates all available providers regardless of enabled status
func (f *Factory) CreateAllProviders() ([]uploader.Provider, error) {
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
	providers = append(providers, buzzProvider)

	return providers, nil
}