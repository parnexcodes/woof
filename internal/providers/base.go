package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/parnexcodes/woof/internal/logging"
)

// EnhancedProvider interface extends the base Provider with additional capabilities
type EnhancedProvider interface {
	Name() string
	Upload(ctx context.Context, filePath string, file io.Reader, size int64) (*ProviderResponse, error)
	ValidateFile(ctx context.Context, filePath string, size int64) error
	GetMaxFileSize() int64
	GetSupportedExtensions() []string
	GetTimeout() time.Duration
}

// BaseProvider provides common functionality for all providers
type BaseProvider struct {
	name     string
	client   *http.Client
	timeout  time.Duration
	maxSize  int64
	supportedExtensions map[string]bool
}

// NewBaseProvider creates a new base provider with common configuration
func NewBaseProvider(name string, timeout time.Duration, maxSize int64, extensions []string) *BaseProvider {
	supportedExts := make(map[string]bool)
	for _, ext := range extensions {
		supportedExts[strings.ToLower(ext)] = true
	}

	client := &http.Client{
		Timeout: timeout,
	}

	return &BaseProvider{
		name:   name,
		client: client,
		timeout: timeout,
		maxSize: maxSize,
		supportedExtensions: supportedExts,
	}
}

// Name returns the provider name
func (bp *BaseProvider) Name() string {
	return bp.name
}

// GetMaxFileSize returns the maximum file size supported by the provider
func (bp *BaseProvider) GetMaxFileSize() int64 {
	return bp.maxSize
}

// GetSupportedExtensions returns the list of supported file extensions
func (bp *BaseProvider) GetSupportedExtensions() []string {
	var extensions []string
	for ext := range bp.supportedExtensions {
		extensions = append(extensions, ext)
	}
	return extensions
}

// GetTimeout returns the HTTP timeout for the provider
func (bp *BaseProvider) GetTimeout() time.Duration {
	return bp.timeout
}

// ValidateFile validates a file before upload
func (bp *BaseProvider) ValidateFile(ctx context.Context, filePath string, size int64) error {
	// Check file size
	if bp.maxSize > 0 && size > bp.maxSize {
		logFields := map[string]interface{}{
			"provider":     bp.name,
			"file_size":    size,
			"max_size":     bp.maxSize,
			"file_path":    filePath,
		}
		logging.ErrorContext("file_too_large", fmt.Errorf("file too large"), logFields)
		return NewFileTooLargeError(
			fmt.Sprintf("file size %d bytes exceeds maximum %d bytes", size, bp.maxSize),
			nil,
		)
	}

	// Check file extension if restrictions exist
	if len(bp.supportedExtensions) > 0 {
		ext := strings.ToLower(filepath.Ext(filePath))
		if !bp.supportedExtensions[ext] && !bp.supportedExtensions["*"] {
			var supported []string
			for ext := range bp.supportedExtensions {
				supported = append(supported, ext)
			}

			logFields := map[string]interface{}{
				"provider":    bp.name,
				"extension":   ext,
				"supported":   supported,
				"file_path":   filePath,
			}
			logging.ErrorContext("unsupported_extension", fmt.Errorf("unsupported file extension"), logFields)
			return NewUnsupportedError(
				fmt.Sprintf("file extension %s is not supported. Supported: %v", ext, supported),
				nil,
			)
		}
	}

	return nil
}

// MakeRequest creates and executes an HTTP request with common headers and logging
func (bp *BaseProvider) MakeRequest(ctx context.Context, method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		logging.ErrorContext("http_request_create", err, map[string]interface{}{
			"provider": bp.name,
			"method":   method,
			"url":      url,
		})
		return nil, NewNetworkError(
			fmt.Sprintf("failed to create request: %s", method),
			err,
		)
	}

	// Set headers
	req.Header.Set("User-Agent", "woof/1.0")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Log the request
	logging.HTTPRequest(method, url, headers)

	// Execute request
	resp, err := bp.client.Do(req)
	if err != nil {
		logging.ErrorContext("http_request", err, map[string]interface{}{
			"provider": bp.name,
			"url":      url,
		})
		return nil, NewNetworkError(
			fmt.Sprintf("request failed: %s", url),
			err,
		)
	}

	return resp, nil
}

// ParseResponse parses a JSON response body into the target structure
func (bp *BaseProvider) ParseResponse(resp *http.Response, target interface{}) ([]byte, error) {
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logging.ErrorContext("http_response_read", err, map[string]interface{}{
			"provider":     bp.name,
			"status_code":  resp.StatusCode,
			"response_len": len(body),
		})
		return nil, NewNetworkError("failed to read response body", err)
	}

	// Log the response
	duration := time.Since(time.Now()) // This should be passed in from caller
	logging.HTTPResponse(resp.StatusCode, string(body), duration)

	// Check for non-success status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logging.ErrorContext("api_error", fmt.Errorf("API returned status %d", resp.StatusCode), map[string]interface{}{
			"provider":     bp.name,
			"status_code":  resp.StatusCode,
			"response":     string(body),
		})
		return body, NewAPIError(
			fmt.Sprintf("%d", resp.StatusCode),
			fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
			nil,
		)
	}

	// Parse JSON body if target is provided
	if target != nil && len(body) > 0 {
		if err := json.Unmarshal(body, target); err != nil {
			logging.ErrorContext("json_parse", err, map[string]interface{}{
				"provider": bp.name,
				"response": string(body),
			})
			return body, NewAPIError("JSON_PARSE_ERROR", "failed to parse API response", err)
		}
	}

	return body, nil
}

// ValidateURL validates that a URL is not empty
func (bp *BaseProvider) ValidateURL(url string) error {
	if url == "" {
		return NewAPIError("MISSING_URL", "provider response missing download URL", nil)
	}
	return nil
}

// CreateSuccessResponse creates a success response
func (bp *BaseProvider) CreateSuccessResponse(url, downloadURL, deleteURL, id string, expires *time.Time, metadata map[string]string, providerData interface{}) *ProviderResponse {
	if metadata == nil {
		metadata = make(map[string]string)
	}

	return &ProviderResponse{
		URL:          url,
		DownloadURL:  downloadURL,
		DeleteURL:    deleteURL,
		ID:           id,
		Expires:      expires,
		Metadata:     metadata,
		ProviderData: providerData,
	}
}

// GetFileExtension returns the lowercase file extension
func (bp *BaseProvider) GetFileExtension(filePath string) string {
	return strings.ToLower(filepath.Ext(filePath))
}

// IsRetryableError checks if an error is retryable
func (bp *BaseProvider) IsRetryableError(err error) bool {
	return IsRetryable(err)
}

// LogProviderError logs provider errors with context
func (bp *BaseProvider) LogProviderError(operation string, err error, fields map[string]interface{}) {
	// Add provider name to fields
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["provider"] = bp.name

	logging.ErrorContext(operation, err, fields)
}