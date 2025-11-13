package gofile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"github.com/parnexcodes/woof/internal/logging"
	"github.com/parnexcodes/woof/internal/providers"
)

// GoFileResponse represents the API response format
type GoFileResponse struct {
	Status string `json:"status"`
	Data   struct {
		DownloadPage string `json:"downloadPage"`
		ID           string `json:"id"`
		FileName     string `json:"fileName"`
	} `json:"data"`
}

// GoFileProvider implements the provider interface for GoFile
type GoFileProvider struct {
	UploadURL            string
	Timeout              time.Duration
	HTTPClient           *http.Client
	OptionalFolderID     string
	// Provider capabilities - GoFile has no file size limits
	MaxFileSize          int64
	SupportedExtensions  map[string]bool
}

// New creates a new GoFile provider
func New(config map[string]interface{}) (*GoFileProvider, error) {
	uploadURL, ok := config["upload_url"].(string)
	if !ok {
		uploadURL = "https://upload.gofile.io/uploadFile"
	}

	timeoutStr, ok := config["timeout"].(string)
	if !ok {
		timeoutStr = "10m"
	}
	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		timeout = 10 * time.Minute // Default timeout
		logging.ErrorContext("provider_config", err, map[string]interface{}{
			"provider": "GoFile",
			"setting":  "timeout",
			"value":    timeoutStr,
		})
	}

	optionalFolderID, _ := config["folder_id"].(string)

	providerConfig := map[string]interface{}{
		"upload_url": uploadURL,
		"timeout":    timeout.String(),
		"folder_id":  optionalFolderID,
	}
	logging.ProviderConfig("GoFile", providerConfig)

	// GoFile has no file size limits - set to 0 (unlimited)
	maxSize := int64(0)

	// Support all file types by default
	supportedExtensions := make(map[string]bool)
	supportedExtensions["*"] = true

	return &GoFileProvider{
		UploadURL:            uploadURL,
		Timeout:              timeout,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		OptionalFolderID:     optionalFolderID,
		MaxFileSize:          maxSize,
		SupportedExtensions:  supportedExtensions,
	}, nil
}

// Name returns the provider name
func (p *GoFileProvider) Name() string {
	return "GoFile"
}

// uploadWithResponse implements the upload method with standardized response
func (p *GoFileProvider) uploadWithResponse(ctx context.Context, filePath string, file io.Reader, size int64) (*providers.ProviderResponse, error) {
	// Validate the file first
	if err := p.ValidateFile(ctx, filePath, size); err != nil {
		return nil, err
	}

	// Extract filename from path
	filename := filepath.Base(filePath)

	// Read entire content to ensure we have the complete data
	buf, err := io.ReadAll(file)
	if err != nil {
		p.logProviderError("file_read", err, map[string]interface{}{
			"file": filename,
			"size": size,
		})
		return nil, providers.NewNetworkError("failed to read file", err)
	}
	actualSize := int64(len(buf))

	// Create multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add file field
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		p.logProviderError("form_file_create", err, map[string]interface{}{
			"filename": filename,
		})
		return nil, providers.NewNetworkError("failed to create form file", err)
	}

	_, err = part.Write(buf)
	if err != nil {
		p.logProviderError("form_file_write", err, map[string]interface{}{
			"filename": filename,
		})
		return nil, providers.NewNetworkError("failed to write form file", err)
	}

	// Add optional folder ID field
	if p.OptionalFolderID != "" {
		err = writer.WriteField("folderId", p.OptionalFolderID)
		if err != nil {
			p.logProviderError("form_folder_write", err, map[string]interface{}{
				"folder_id": p.OptionalFolderID,
			})
			return nil, providers.NewNetworkError("failed to write folder ID", err)
		}
	}

	// Close the writer to finalize the form
	err = writer.Close()
	if err != nil {
		p.logProviderError("form_close", err, nil)
		return nil, providers.NewNetworkError("failed to close form writer", err)
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.UploadURL, &body)
	if err != nil {
		p.logProviderError("http_request_create", err, map[string]interface{}{
			"method": http.MethodPost,
			"url":    p.UploadURL,
		})
		return nil, providers.NewNetworkError("failed to create request", err)
	}

	// Set content type and content length
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Content-Length", fmt.Sprintf("%d", body.Len()))

	// Log HTTP request details
	logging.HTTPRequest(http.MethodPost, p.UploadURL, map[string]string{
		"Content-Type":   writer.FormDataContentType(),
		"Content-Length": fmt.Sprintf("%d", body.Len()),
		"folder_id":      p.OptionalFolderID,
	})

	// Make request and measure duration
	start := time.Now()
	resp, err := p.HTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		p.logProviderError("http_request", err, map[string]interface{}{
			"url": p.UploadURL,
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
	var response GoFileResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		p.logProviderError("json_parse", err, map[string]interface{}{
			"response": string(responseBody),
		})
		return nil, providers.NewAPIError("JSON_PARSE_ERROR", "failed to parse response", err)
	}

	// Check response status
	if response.Status != "ok" {
		return nil, providers.NewAPIError(
			"UPLOAD_ERROR",
			fmt.Sprintf("upload failed with status: %s", response.Status),
			nil,
		)
	}

	if response.Data.DownloadPage == "" {
		return nil, providers.NewAPIError("MISSING_DOWNLOAD_URL", "upload response missing download URL", nil)
	}

	if response.Data.ID == "" {
		return nil, providers.NewAPIError("MISSING_ID", "upload response missing file ID", nil)
	}

	// Create structured response
	result := &providers.ProviderResponse{
		URL:         response.Data.DownloadPage,
		DownloadURL: response.Data.DownloadPage,
		ID:          response.Data.ID,
		Metadata: map[string]string{
			"provider":      "GoFile",
			"upload_method": "multipart_form",
			"duration_ms":   fmt.Sprintf("%d", duration.Milliseconds()),
			"original_name": filename,
			"upload_size":   fmt.Sprintf("%d", actualSize),
			"gofile_id":     response.Data.ID,
			"gofile_name":   response.Data.FileName,
		},
		ProviderData: &GoFileResponse{
			Status: response.Status,
			Data:   response.Data,
		},
	}

	if p.OptionalFolderID != "" {
		result.Metadata["folder_id"] = p.OptionalFolderID
	}

	logging.UploadComplete(filename, response.Data.DownloadPage, duration)

	return result, nil
}

// ValidateFile validates a file before upload
func (p *GoFileProvider) ValidateFile(ctx context.Context, filePath string, size int64) error {
	// GoFile has no file size limits, so no size validation needed
	return nil
}

// GetMaxFileSize returns the maximum file size supported by the provider
func (p *GoFileProvider) GetMaxFileSize() int64 {
	return p.MaxFileSize // 0 means unlimited
}

// GetSupportedExtensions returns the list of supported file extensions
func (p *GoFileProvider) GetSupportedExtensions() []string {
	var extensions []string
	for ext := range p.SupportedExtensions {
		extensions = append(extensions, ext)
	}
	return extensions
}

// logProviderError logs provider errors with context
func (p *GoFileProvider) logProviderError(operation string, err error, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["provider"] = "GoFile"
	logging.ErrorContext(operation, err, fields)
}

// Upload uploads a file to GoFile and returns a structured response
func (p *GoFileProvider) Upload(ctx context.Context, filePath string, file io.Reader, size int64) (*providers.ProviderResponse, error) {
	return p.uploadWithResponse(ctx, filePath, file, size)
}