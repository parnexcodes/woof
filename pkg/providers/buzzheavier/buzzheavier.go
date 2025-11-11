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
)

// BuzzHeavierResponse represents the API response format
type BuzzHeavierResponse struct {
	Code int `json:"code"`
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

// BuzzHeavierProvider implements the uploader.Provider interface for BuzzHeavier
type BuzzHeavierProvider struct {
	UploadURL        string
	DownloadBaseURL  string
	Timeout          time.Duration
	HTTPClient       *http.Client
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

	return &BuzzHeavierProvider{
		UploadURL:       uploadURL,
		DownloadBaseURL: downloadBaseURL,
		Timeout:         timeout,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}, nil
}

// Name returns the provider name
func (p *BuzzHeavierProvider) Name() string {
	return "BuzzHeavier"
}

// Upload uploads a file to BuzzHeavier
func (p *BuzzHeavierProvider) Upload(ctx context.Context, filePath string, file io.Reader, size int64) (string, error) {
	// Extract filename from path
	filename := filepath.Base(filePath)
	uploadURL := fmt.Sprintf("%s/%s", p.UploadURL, filename)

	// Read entire content to ensure we have the complete data and correct size
	buf, err := io.ReadAll(file)
	if err != nil {
		logging.ErrorContext("file_read", err, map[string]interface{}{
			"file":  filename,
			"size":  size,
		})
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	actualSize := int64(len(buf))

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(buf))
	if err != nil {
		logging.ErrorContext("http_request_create", err, map[string]interface{}{
			"method": http.MethodPut,
			"url":    uploadURL,
		})
		return "", fmt.Errorf("failed to create request: %w", err)
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
		logging.ErrorContext("http_request", err, map[string]interface{}{
			"url": uploadURL,
		})
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for debugging
	responseBody, _ := io.ReadAll(resp.Body)

	// Log HTTP response
	logging.HTTPResponse(resp.StatusCode, string(responseBody), duration)

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(responseBody))
	}

	// Parse JSON response (from already read body)
	var response BuzzHeavierResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		logging.ErrorContext("json_parse", err, map[string]interface{}{
			"response": string(responseBody),
		})
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Check response code
	if response.Code != http.StatusOK && response.Code != http.StatusCreated {
		return "", fmt.Errorf("upload failed with code %d", response.Code)
	}

	if response.Data.ID == "" {
		return "", fmt.Errorf("upload response missing file ID")
	}

	// Construct download URL
	downloadURL := fmt.Sprintf("%s/%s", p.DownloadBaseURL, response.Data.ID)

	return downloadURL, nil
}