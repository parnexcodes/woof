package gofile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/parnexcodes/woof/internal/logging"
	"github.com/parnexcodes/woof/internal/providers"
)

func TestMain(m *testing.M) {
	// Initialize logging for tests
	logging.Init(false, os.Stderr)
	os.Exit(m.Run())
}

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected *GoFileProvider
	}{
		{
			name:   "default config",
			config: map[string]interface{}{},
			expected: &GoFileProvider{
				UploadURL:           "https://upload.gofile.io/uploadFile",
				Timeout:             10 * time.Minute,
				OptionalFolderID:    "",
				MaxFileSize:         0,
				SupportedExtensions: map[string]bool{"*": true},
			},
		},
		{
			name: "custom config",
			config: map[string]interface{}{
				"upload_url": "https://custom.upload.example.com",
				"timeout":    "5m",
				"folder_id":  "folder123",
			},
			expected: &GoFileProvider{
				UploadURL:           "https://custom.upload.example.com",
				Timeout:             5 * time.Minute,
				OptionalFolderID:    "folder123",
				MaxFileSize:         0,
				SupportedExtensions: map[string]bool{"*": true},
			},
		},
		{
			name: "invalid timeout uses default",
			config: map[string]interface{}{
				"timeout": "invalid",
			},
			expected: &GoFileProvider{
				UploadURL:           "https://upload.gofile.io/uploadFile",
				Timeout:             10 * time.Minute,
				OptionalFolderID:    "",
				MaxFileSize:         0,
				SupportedExtensions: map[string]bool{"*": true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(tt.config)
			require.NoError(t, err)

			assert.Equal(t, tt.expected.UploadURL, provider.UploadURL)
			assert.Equal(t, tt.expected.Timeout, provider.Timeout)
			assert.Equal(t, tt.expected.OptionalFolderID, provider.OptionalFolderID)
			assert.Equal(t, tt.expected.MaxFileSize, provider.MaxFileSize)
			assert.Equal(t, tt.expected.SupportedExtensions, provider.SupportedExtensions)
		})
	}
}

func TestName(t *testing.T) {
	provider, err := New(map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, "GoFile", provider.Name())
}

func TestValidateFile(t *testing.T) {
	provider, err := New(map[string]interface{}{})
	require.NoError(t, err)

	// GoFile has no file size limits, so validation should always pass
	err = provider.ValidateFile(context.Background(), "test.txt", 100*1024*1024*1024) // 100GB
	assert.NoError(t, err)

	err = provider.ValidateFile(context.Background(), "test.txt", 0)
	assert.NoError(t, err)
}

func TestGetMaxFileSize(t *testing.T) {
	provider, err := New(map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), provider.GetMaxFileSize()) // 0 means unlimited
}

func TestGetSupportedExtensions(t *testing.T) {
	provider, err := New(map[string]interface{}{})
	require.NoError(t, err)
	extensions := provider.GetSupportedExtensions()
	assert.Contains(t, extensions, "*")
}

func TestUpload_Success(t *testing.T) {
	// Mock successful GoFile API response
	mockResponse := GoFileResponse{
		Status: "ok",
		Data: struct {
			DownloadPage string `json:"downloadPage"`
			ID           string `json:"id"`
			FileName     string `json:"fileName"`
		}{
			DownloadPage: "https://gofile.io/d/abc123",
			ID:           "abc123",
			FileName:     "test.txt",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/uploadFile", r.URL.Path)
		assert.Contains(t, r.Header.Get("Content-Type"), "multipart/form-data")

		// Parse multipart form to verify file upload
		err := r.ParseMultipartForm(10 << 20) // 10MB max
		require.NoError(t, err)

		file, header, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()

		assert.Equal(t, "test.txt", header.Filename)

		// Read file content to verify
		content, err := io.ReadAll(file)
		require.NoError(t, err)
		assert.Equal(t, "test file content", string(content))

		// Check optional folder ID if present
		folderID := r.FormValue("folderId")
		if folderID != "" {
			assert.Equal(t, "testfolder", folderID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"upload_url": server.URL + "/uploadFile",
		"folder_id":  "testfolder",
	})
	require.NoError(t, err)

	file := bytes.NewBufferString("test file content")
	response, err := provider.Upload(context.Background(), "test.txt", file, int64(file.Len()))
	require.NoError(t, err)

	assert.Equal(t, "https://gofile.io/d/abc123", response.URL)
	assert.Equal(t, "https://gofile.io/d/abc123", response.DownloadURL)
	assert.Equal(t, "abc123", response.ID)
	assert.Equal(t, "GoFile", response.Metadata["provider"])
	assert.Equal(t, "multipart_form", response.Metadata["upload_method"])
	assert.Equal(t, "test.txt", response.Metadata["original_name"])
	assert.Equal(t, "abc123", response.Metadata["gofile_id"])
	assert.Equal(t, "testfolder", response.Metadata["folder_id"])

	// Verify provider data
	providerData, ok := response.ProviderData.(*GoFileResponse)
	require.True(t, ok)
	assert.Equal(t, "ok", providerData.Status)
	assert.Equal(t, "abc123", providerData.Data.ID)
	assert.Equal(t, "test.txt", providerData.Data.FileName)
}

func TestUpload_WithoutFolderID(t *testing.T) {
	mockResponse := GoFileResponse{
		Status: "ok",
		Data: struct {
			DownloadPage string `json:"downloadPage"`
			ID           string `json:"id"`
			FileName     string `json:"fileName"`
		}{
			DownloadPage: "https://gofile.io/d/xyz789",
			ID:           "xyz789",
			FileName:     "photo.jpg",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse multipart form
		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)

		file, header, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()

		assert.Equal(t, "photo.jpg", header.Filename)

		// Verify no folder ID is present
		folderID := r.FormValue("folderId")
		assert.Equal(t, "", folderID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"upload_url": server.URL + "/uploadFile",
	})
	require.NoError(t, err)

	file := bytes.NewBufferString("image content")
	response, err := provider.Upload(context.Background(), "photo.jpg", file, int64(file.Len()))
	require.NoError(t, err)

	assert.Equal(t, "https://gofile.io/d/xyz789", response.URL)
	assert.Equal(t, "xyz789", response.ID)
	assert.Equal(t, "photo.jpg", response.Metadata["original_name"])
	assert.NotContains(t, response.Metadata, "folder_id")
}

func TestUpload_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Internal Server Error")
	}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"upload_url": server.URL + "/uploadFile",
	})
	require.NoError(t, err)

	file := bytes.NewBufferString("test content")
	response, err := provider.Upload(context.Background(), "test.txt", file, int64(file.Len()))
	assert.Nil(t, response)
	assert.Error(t, err)

	// Check if it's an API error
	var apiErr *providers.ProviderError
	assert.True(t, errors.As(err, &apiErr))
	assert.Equal(t, providers.ErrorTypeAPI, apiErr.Type)
}

func TestUpload_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "{invalid json")
	}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"upload_url": server.URL + "/uploadFile",
	})
	require.NoError(t, err)

	file := bytes.NewBufferString("test content")
	response, err := provider.Upload(context.Background(), "test.txt", file, int64(file.Len()))
	assert.Nil(t, response)
	assert.Error(t, err)

	// Check if it's an API error
	var apiErr *providers.ProviderError
	assert.True(t, errors.As(err, &apiErr))
	assert.Equal(t, providers.ErrorTypeAPI, apiErr.Type)
	assert.Equal(t, "JSON_PARSE_ERROR", apiErr.Code)
}

func TestUpload_APIErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "error",
			"data":   map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"upload_url": server.URL + "/uploadFile",
	})
	require.NoError(t, err)

	file := bytes.NewBufferString("test content")
	response, err := provider.Upload(context.Background(), "test.txt", file, int64(file.Len()))
	assert.Nil(t, response)
	assert.Error(t, err)

	// Check if it's an API error
	var apiErr *providers.ProviderError
	assert.True(t, errors.As(err, &apiErr))
	assert.Equal(t, providers.ErrorTypeAPI, apiErr.Type)
	assert.Equal(t, "UPLOAD_ERROR", apiErr.Code)
}

func TestUpload_MissingDownloadURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"id":       "test123",
				"fileName": "test.txt",
				// Missing downloadPage
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"upload_url": server.URL + "/uploadFile",
	})
	require.NoError(t, err)

	file := bytes.NewBufferString("test content")
	response, err := provider.Upload(context.Background(), "test.txt", file, int64(file.Len()))
	assert.Nil(t, response)
	assert.Error(t, err)

	// Check if it's an API error
	var apiErr *providers.ProviderError
	assert.True(t, errors.As(err, &apiErr))
	assert.Equal(t, providers.ErrorTypeAPI, apiErr.Type)
	assert.Equal(t, "MISSING_DOWNLOAD_URL", apiErr.Code)
}

func TestUpload_MissingID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "ok",
			"data": map[string]interface{}{
				"downloadPage": "https://gofile.io/d/test",
				"fileName":     "test.txt",
				// Missing ID
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"upload_url": server.URL + "/uploadFile",
	})
	require.NoError(t, err)

	file := bytes.NewBufferString("test content")
	response, err := provider.Upload(context.Background(), "test.txt", file, int64(file.Len()))
	assert.Nil(t, response)
	assert.Error(t, err)

	// Check if it's an API error
	var apiErr *providers.ProviderError
	assert.True(t, errors.As(err, &apiErr))
	assert.Equal(t, providers.ErrorTypeAPI, apiErr.Type)
	assert.Equal(t, "MISSING_ID", apiErr.Code)
}

func TestUpload_FileReadError(t *testing.T) {
	provider, err := New(map[string]interface{}{})
	require.NoError(t, err)

	// Create a reader that will fail on read
	errorReader := &errorReader{error: fmt.Errorf("read error")}

	response, err := provider.Upload(context.Background(), "test.txt", errorReader, 100)
	assert.Nil(t, response)
	assert.Error(t, err)

	// Check if it's a network error
	var netErr *providers.ProviderError
	assert.True(t, errors.As(err, &netErr))
	assert.Equal(t, providers.ErrorTypeNetwork, netErr.Type)
}

// errorReader is a test helper that always returns an error on Read
type errorReader struct {
	error error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.error
}

func TestUpload_ContextCancellation(t *testing.T) {
	provider, err := New(map[string]interface{}{
		"timeout": "100ms", // Very short timeout
	})
	require.NoError(t, err)

	// Create a context that will be cancelled
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Use a slow server that won't respond in time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Sleep longer than context timeout
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer server.Close()

	provider.UploadURL = server.URL + "/uploadFile"

	file := bytes.NewBufferString(strings.Repeat("x", 1024))
	response, err := provider.Upload(ctx, "test.txt", file, int64(file.Len()))
	assert.Nil(t, response)
	assert.Error(t, err)

	// Check that it's a network error due to context cancellation
	var netErr *providers.ProviderError
	assert.True(t, errors.As(err, &netErr))
	assert.Equal(t, providers.ErrorTypeNetwork, netErr.Type)
	assert.Contains(t, netErr.Cause.Error(), "context deadline exceeded")
}

func TestUpload_ZeroSizeFile(t *testing.T) {
	mockResponse := GoFileResponse{
		Status: "ok",
		Data: struct {
			DownloadPage string `json:"downloadPage"`
			ID           string `json:"id"`
			FileName     string `json:"fileName"`
		}{
			DownloadPage: "https://gofile.io/d/empty",
			ID:           "empty",
			FileName:     "empty.txt",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse multipart form
		err := r.ParseMultipartForm(10 << 20)
		require.NoError(t, err)

		file, header, err := r.FormFile("file")
		require.NoError(t, err)
		defer file.Close()

		assert.Equal(t, "empty.txt", header.Filename)

		// Read file content - should be empty
		content, err := io.ReadAll(file)
		require.NoError(t, err)
		assert.Equal(t, []byte{}, content)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	provider, err := New(map[string]interface{}{
		"upload_url": server.URL + "/uploadFile",
	})
	require.NoError(t, err)

	file := bytes.NewBuffer([]byte{}) // Empty file
	response, err := provider.Upload(context.Background(), "empty.txt", file, 0)
	require.NoError(t, err)

	assert.Equal(t, "https://gofile.io/d/empty", response.URL)
	assert.Equal(t, "empty", response.ID)
	assert.Equal(t, "0", response.Metadata["upload_size"])
}
