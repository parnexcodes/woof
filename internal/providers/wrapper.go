package providers

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/parnexcodes/woof/internal/logging"
	"github.com/sirupsen/logrus"
)

// Provider interface for consistency wrapper (minimal interface to avoid circular dependency)
type Provider interface {
	Name() string
	Upload(ctx context.Context, filePath string, file io.Reader, size int64) (*ProviderResponse, error)
	ValidateFile(ctx context.Context, filePath string, size int64) error
	GetMaxFileSize() int64
	GetSupportedExtensions() []string
}

// ConsistencyWrapper wraps providers to ensure standardized behavior
type ConsistencyWrapper struct {
	provider Provider
	config   WrapperConfig
}

// WrapperConfig defines configuration for the consistency wrapper
type WrapperConfig struct {
	// Enable automatic validation before upload
	PreUploadValidation bool `json:"pre_upload_validation"`

	// Enable response validation
	ValidateResponses bool `json:"validate_responses"`

	// Enable automatic retries for retryable errors
	AutoRetry bool `json:"auto_retry"`

	// Maximum retry attempts
	MaxRetries int `json:"max_retries"`

	// Delay between retries
	RetryDelay time.Duration `json:"retry_delay"`

	// Enable response enhancement (add standard metadata)
	EnhanceResponses bool `json:"enhance_responses"`

	// Enable provider capability checking
	CheckCapabilities bool `json:"check_capabilities"`
}

// DefaultWrapperConfig returns a sensible default configuration
func DefaultWrapperConfig() WrapperConfig {
	return WrapperConfig{
		PreUploadValidation: true,
		ValidateResponses:    true,
		AutoRetry:           true,
		MaxRetries:          3,
		RetryDelay:          2 * time.Second,
		EnhanceResponses:    true,
		CheckCapabilities:   true,
	}
}

// NewConsistencyWrapper creates a new consistency wrapper for a provider
func NewConsistencyWrapper(provider Provider, config WrapperConfig) *ConsistencyWrapper {
	return &ConsistencyWrapper{
		provider: provider,
		config:   config,
	}
}

// Name returns the wrapped provider's name
func (cw *ConsistencyWrapper) Name() string {
	return cw.provider.Name()
}

// GetMaxFileSize returns the wrapped provider's max file size
func (cw *ConsistencyWrapper) GetMaxFileSize() int64 {
	return cw.provider.GetMaxFileSize()
}

// GetSupportedExtensions returns the wrapped provider's supported extensions
func (cw *ConsistencyWrapper) GetSupportedExtensions() []string {
	return cw.provider.GetSupportedExtensions()
}

// ValidateFile validates a file using the wrapped provider's validation
func (cw *ConsistencyWrapper) ValidateFile(ctx context.Context, filePath string, size int64) error {
	return cw.provider.ValidateFile(ctx, filePath, size)
}

// Upload wraps the provider's Upload method with consistency features
func (cw *ConsistencyWrapper) Upload(ctx context.Context, filePath string, file io.Reader, size int64) (*ProviderResponse, error) {
	logging.Debug("Provider upload start", logrus.Fields{
		"provider": cw.provider.Name(),
		"filepath": filePath,
		"size":     size,
		"validation_enabled": cw.config.PreUploadValidation,
		"auto_retry_enabled":  cw.config.AutoRetry,
	})

	// Pre-upload validation if enabled
	if cw.config.PreUploadValidation {
		if err := cw.validateUploadCapability(ctx, filePath, size); err != nil {
			logging.ErrorContext("validation_failed", fmt.Errorf("%s", err), map[string]interface{}{
				"provider": cw.provider.Name(),
				"filepath": filePath,
				"error":    err.Error(),
			})
			return nil, err
		}
	}

	// Upload with optional retry logic
	var response *ProviderResponse
	var err error

	if cw.config.AutoRetry {
		response, err = cw.uploadWithRetry(ctx, filePath, file, size)
	} else {
		response, err = cw.provider.Upload(ctx, filePath, file, size)
	}

	// Add metadata if enabled
	if err == nil && cw.config.EnhanceResponses && response != nil {
		response = cw.addMetadata(response, filePath, size)
	}

	// Validate response if enabled
	if err == nil && cw.config.ValidateResponses && response != nil {
		if validationErr := cw.validateResponse(response); validationErr != nil {
			logging.ErrorContext("validation_failed", fmt.Errorf("%s", validationErr), map[string]interface{}{
				"provider": cw.provider.Name(),
				"filepath": filePath,
				"error":    validationErr.Error(),
			})
			return response, validationErr
		}
	}

	logging.Debug("Provider upload complete", logrus.Fields{
		"provider": cw.provider.Name(),
		"filepath": filePath,
		"success":  err == nil,
		"has_response": response != nil,
	})

	return response, err
}

// validateUploadCapability checks if the provider can handle the upload
func (cw *ConsistencyWrapper) validateUploadCapability(ctx context.Context, filePath string, size int64) error {
	// Check provider capabilities first
	if cw.config.CheckCapabilities {
		// Check file size limits
		maxSize := cw.provider.GetMaxFileSize()
		if maxSize > 0 && size > maxSize {
			return NewFileTooLargeError(
				fmt.Sprintf("file size %d bytes exceeds provider %s maximum %d bytes", size, cw.provider.Name(), maxSize),
				nil,
			)
		}

		// Check file extensions
		extensions := cw.provider.GetSupportedExtensions()
		if len(extensions) > 0 {
			supported := false
			for _, ext := range extensions {
				if strings.ToLower(ext) == "*" {
					supported = true
					break
				}
			}

			if !supported {
				// Check specific file extension
				fileExt := cw.getFileExtension(filePath)
				for _, ext := range extensions {
					if strings.ToLower(ext) == fileExt {
						supported = true
						break
					}
				}

				if !supported {
					return NewUnsupportedError(
						fmt.Sprintf("file extension %s not supported by provider %s. Supported: %v", fileExt, cw.provider.Name(), extensions),
						nil,
					)
				}
			}
		}
	}

	// Use provider's own validation
	return cw.provider.ValidateFile(ctx, filePath, size)
}

// uploadWithRetry implements retry logic for uploads
func (cw *ConsistencyWrapper) uploadWithRetry(ctx context.Context, filePath string, file io.Reader, size int64) (*ProviderResponse, error) {
	var lastError error

	for attempt := 0; attempt <= cw.config.MaxRetries; attempt++ {
		if attempt > 0 {
			logging.Debug("Provider retry attempt", logrus.Fields{
				"provider": cw.provider.Name(),
				"attempt": attempt,
				"max_retries": cw.config.MaxRetries,
				"filepath": filePath,
			})

			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, NewTemporaryError("context cancelled during retry", ctx.Err())
			case <-time.After(cw.config.RetryDelay * time.Duration(attempt-1)):
				// Exponential backoff
			}
		}

		response, err := cw.provider.Upload(ctx, filePath, file, size)

		if err != nil {
			lastError = err

			// Check if error is retryable
			if !cw.isRetryableError(err) {
				logging.Debug("Provider non-retryable error", logrus.Fields{
					"provider": cw.provider.Name(),
					"attempt": attempt,
					"filepath": filePath,
					"error": err.Error(),
				})
				return nil, err
			}

			logging.Debug("Provider retryable error", logrus.Fields{
				"provider": cw.provider.Name(),
				"attempt": attempt,
				"filepath": filePath,
				"error": err.Error(),
			})
			continue
		}

		// Success
		if attempt > 0 {
			logging.Debug("Provider retry success", logrus.Fields{
				"provider": cw.provider.Name(),
				"attempt": attempt,
				"filepath": filePath,
			})
		}

		return response, nil
	}

	// All retries failed
	logging.ErrorContext("all_retries_failed", fmt.Errorf("retry exhaustion"), map[string]interface{}{
		"provider": cw.provider.Name(),
		"max_retries": cw.config.MaxRetries,
		"filepath": filePath,
		"final_error": lastError.Error(),
	})

	return nil, NewTemporaryError(
		fmt.Sprintf("all %d retry attempts failed", cw.config.MaxRetries+1),
		lastError,
	)
}

// addMetadata adds standard metadata and ensures response consistency
func (cw *ConsistencyWrapper) addMetadata(response *ProviderResponse, filePath string, size int64) *ProviderResponse {
	// Ensure metadata exists
	if response.Metadata == nil {
		response.Metadata = make(map[string]string)
	}

	// Add standard metadata
	response.Metadata["wrapper_provider"] = cw.provider.Name()
	response.Metadata["wrapper_version"] = "1.0"
	response.Metadata["upload_timestamp"] = time.Now().Format(time.RFC3339)
	response.Metadata["original_filepath"] = filePath
	response.Metadata["upload_size"] = fmt.Sprintf("%d", size)

	// Ensure URL is set
	if response.URL == "" && response.DownloadURL != "" {
		response.URL = response.DownloadURL
	}

	// Log metadata addition
	logging.Debug("Provider metadata added", logrus.Fields{
		"provider": cw.provider.Name(),
		"filepath": filePath,
		"added_metadata": len(response.Metadata),
	})

	return response
}

// validateResponse ensures the response meets minimum requirements
func (cw *ConsistencyWrapper) validateResponse(response *ProviderResponse) error {
	if response == nil {
		return NewAPIError("NULL_RESPONSE", "provider returned null response", nil)
	}

	if response.URL == "" {
		return NewAPIError("MISSING_URL", "provider response missing download URL", nil)
	}

	return nil
}

// isRetryableError checks if an error should be retried
func (cw *ConsistencyWrapper) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errorType := GetErrorType(err)
	switch errorType {
	case ErrorTypeNetwork, ErrorTypeTemporary, ErrorTypeQuota:
		return true
	case ErrorTypeAPI, ErrorTypeAuthentication, ErrorTypeFileTooLarge, ErrorTypeUnsupported:
		return false
	default:
		// For unknown errors, assume retryable for network-related issues
		return strings.Contains(strings.ToLower(err.Error()), "connection") ||
			   strings.Contains(strings.ToLower(err.Error()), "timeout") ||
			   strings.Contains(strings.ToLower(err.Error()), "temporary")
	}
}

// getFileExtension extracts file extension from path
func (cw *ConsistencyWrapper) getFileExtension(filePath string) string {
	if lastDot := strings.LastIndex(filePath, "."); lastDot != -1 && lastDot < len(filePath)-1 {
		return strings.ToLower(filePath[lastDot:])
	}
	return ""
}