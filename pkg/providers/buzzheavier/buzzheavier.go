package buzzheavier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"
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
	}

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

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, file)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set content type
	req.Header.Set("Content-Type", "application/octet-stream")

	// Make request
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var response BuzzHeavierResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
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