package providers

import (
	"fmt"
	"strings"

	"github.com/parnexcodes/woof/internal/config"
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
	switch strings.ToLower(providerConfig.Name) {
	case "buzzheavier":
		return buzzheavier.New(providerConfig.Settings)
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerConfig.Name)
	}
}

// CreateProviders creates multiple provider instances from configuration
func (f *Factory) CreateProviders(providerConfigs []config.ProviderConfig) ([]uploader.Provider, error) {
	var providers []uploader.Provider

	for _, providerConfig := range providerConfigs {
		if !providerConfig.Enabled {
			continue
		}

		provider, err := f.CreateProvider(providerConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create provider '%s': %w", providerConfig.Name, err)
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