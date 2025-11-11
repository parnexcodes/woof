package buzzheavier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/parnexcodes/woof/internal/logging"
	"github.com/parnexcodes/woof/internal/providers"
)

// BuzzHeavierResponse represents the API response format
type BuzzHeavierResponse struct {
	Code int `json:"code"`
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

// BuzzHeavierProvider implements the provider interface for BuzzHeavier
type BuzzHeavierProvider struct {
	UploadURL            string
	DownloadBaseURL      string
	Timeout              time.Duration
	HTTPClient           *http.Client
	// Provider capabilities
	MaxFileSize          int64
	SupportedExtensions  map[string]bool
}

// New creates a new BuzzHeavier provider
func New(config map[string]interface{}) (*BuzzHeavierProvider, error) {
	uploadURL, ok := config["upload_url"].(string)
	if !ok {
		uploadURL = "https://w.buzzheavier.com"
	}

	downloadBaseURL, ok := config["download_base_url"].(string)
	if !ok {
		downloadBaseURL = "https://buzzheavier.com"
	}

	timeoutStr, ok := config["timeout"].(string)
	if !ok {
		timeoutStr = "10m"
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		timeout = 10 * time.Minute // Default timeout
		logging.ErrorContext("provider_config", err, map[string]interface{}{
			"provider": "BuzzHeavier",
			"setting":  "timeout",
			"value":    timeoutStr,
		})
	}

	providerConfig := map[string]interface{}{
		"upload_url":        uploadURL,
		"download_base_url": downloadBaseURL,
		"timeout":           timeout.String(),
	}
	logging.ProviderConfig("BuzzHeavier", providerConfig)

// Provider configuration
	maxSize := int64(10 * 1024 * 1024 * 1024) // 10GB default
	if size, ok := config["max_file_size"].(int64); ok {
		maxSize = size
	}

	// Support all file types by default
	supportedExtensions := make(map[string]bool)
	supportedExtensions["*"] = true

	return &BuzzHeavierProvider{
		UploadURL:            uploadURL,
		DownloadBaseURL:      downloadBaseURL,
		Timeout:              timeout,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		MaxFileSize:          maxSize,
		SupportedExtensions:  supportedExtensions,
	}, nil
}

// Name returns the provider name
func (p *BuzzHeavierProvider) Name() string {
	return "BuzzHeavier"
}

// uploadWithResponse implements the upload method with standardized response
func (p *BuzzHeavierProvider) uploadWithResponse(ctx context.Context, filePath string, file io.Reader, size int64) (*providers.ProviderResponse, error) {
	// Validate the file first
	if err := p.ValidateFile(ctx, filePath, size); err != nil {
		return nil, err
	}

	// Extract filename from path
	filename := filepath.Base(filePath)
	uploadURL := fmt.Sprintf("%s/%s", p.UploadURL, filename)

	// Read entire content to ensure we have the complete data and correct size
	buf, err := io.ReadAll(file)
	if err != nil {
		p.logProviderError("file_read", err, map[string]interface{}{
			"file":  filename,
			"size":  size,
		})
		return nil, providers.NewNetworkError("failed to read file", err)
	}
	actualSize := int64(len(buf))

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(buf))
	if err != nil {
		p.logProviderError("http_request_create", err, map[string]interface{}{
			"method": http.MethodPut,
			"url":    uploadURL,
		})
		return nil, providers.NewNetworkError("failed to create request", err)
	}

	// Set content type and content length
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", actualSize))

	// Log HTTP request details
	logging.HTTPRequest(http.MethodPut, uploadURL, map[string]string{
		"Content-Type":   "application/octet-stream",
		"Content-Length": fmt.Sprintf("%d", actualSize),
	})

	// Make request and measure duration
	start := time.Now()
	resp, err := p.HTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		p.logProviderError("http_request", err, map[string]interface{}{
			"url": uploadURL,
		})
		return nil, providers.NewNetworkError("failed to upload file", err)
	}
	defer resp.Body.Close()

	// Read response body for debugging
	responseBody, _ := io.ReadAll(resp.Body)

	// Log HTTP response
	logging.HTTPResponse(resp.StatusCode, string(responseBody), duration)

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, providers.NewAPIError(
			fmt.Sprintf("%d", resp.StatusCode),
			fmt.Sprintf("upload failed with status %d: %s", resp.StatusCode, string(responseBody)),
			err,
		)
	}

	// Parse JSON response (from already read body)
	var response BuzzHeavierResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		p.logProviderError("json_parse", err, map[string]interface{}{
			"response": string(responseBody),
		})
		return nil, providers.NewAPIError("JSON_PARSE_ERROR", "failed to parse response", err)
	}

	// Check response code
	if response.Code != http.StatusOK && response.Code != http.StatusCreated {
		return nil, providers.NewAPIError(
			fmt.Sprintf("%d", response.Code),
			fmt.Sprintf("upload failed with code %d", response.Code),
			nil,
		)
	}

	if response.Data.ID == "" {
		return nil, providers.NewAPIError("MISSING_ID", "upload response missing file ID", nil)
	}

	// Construct download URL
	downloadURL := fmt.Sprintf("%s/%s", p.DownloadBaseURL, response.Data.ID)

	// Create structured response
	result := &providers.ProviderResponse{
		URL:         downloadURL,
		DownloadURL: downloadURL,
		ID:          response.Data.ID,
		Metadata: map[string]string{
			"provider":      "BuzzHeavier",
			"upload_method": "direct",
			"duration_ms":   fmt.Sprintf("%d", duration.Milliseconds()),
			"original_name": filename,
			"upload_size":   fmt.Sprintf("%d", actualSize),
		},
		ProviderData: &BuzzHeavierResponse{
			Code: response.Code,
			Data: response.Data,
		},
	}

	logging.UploadComplete(filename, downloadURL, duration)

	return result, nil
}

// ValidateFile validates a file before upload
func (p *BuzzHeavierProvider) ValidateFile(ctx context.Context, filePath string, size int64) error {
	// Check file size
	if p.MaxFileSize > 0 && size > p.MaxFileSize {
		logging.ErrorContext("file_too_large", fmt.Errorf("file too large"), map[string]interface{}{
			"provider":     "BuzzHeavier",
			"file_size":    size,
			"max_size":     p.MaxFileSize,
			"file_path":    filePath,
		})
		return providers.NewFileTooLargeError(
			fmt.Sprintf("file size %d bytes exceeds maximum %d bytes", size, p.MaxFileSize),
			nil,
		)
	}

	return nil
}

// GetMaxFileSize returns the maximum file size supported by the provider
func (p *BuzzHeavierProvider) GetMaxFileSize() int64 {
	return p.MaxFileSize
}

// GetSupportedExtensions returns the list of supported file extensions
func (p *BuzzHeavierProvider) GetSupportedExtensions() []string {
	var extensions []string
	for ext := range p.SupportedExtensions {
		extensions = append(extensions, ext)
	}
	return extensions
}

// logProviderError logs provider errors with context
func (p *BuzzHeavierProvider) logProviderError(operation string, err error, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["provider"] = "BuzzHeavier"
	logging.ErrorContext(operation, err, fields)
}

// Upload uploads a file to BuzzHeavier and returns a structured response
func (p *BuzzHeavierProvider) Upload(ctx context.Context, filePath string, file io.Reader, size int64) (*providers.ProviderResponse, error) {
	return p.uploadWithResponse(ctx, filePath, file, size)
}