package buzzheavier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/parnexcodes/woof/internal/logging"
	"github.com/parnexcodes/woof/internal/providers"
)

func init() {
	// Initialize logging for tests
	logging.Init(false, os.Stderr)
}

func TestBuzzHeavierProvider_New(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]interface{}
		expected string
	}{
		{
			name: "default config",
			config: map[string]interface{}{},
			expected: "https://w.buzzheavier.com",
		},
		{
			name: "custom config",
			config: map[string]interface{}{
				"upload_url":        "https://custom.example.com",
				"download_base_url": "https://download.example.com",
				"timeout":           "5m",
			},
			expected: "https://custom.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(tt.config)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			if provider.UploadURL != tt.expected {
				t.Errorf("UploadURL = %v, want %v", provider.UploadURL, tt.expected)
			}
		})
	}
}

func TestBuzzHeavierProvider_Name(t *testing.T) {
	provider, err := New(map[string]interface{}{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	expected := "BuzzHeavier"
	if got := provider.Name(); got != expected {
		t.Errorf("Name() = %v, want %v", got, expected)
	}
}

func TestBuzzHeavierProvider_Upload_Success(t *testing.T) {
	// Mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Method = %v, want %v", r.Method, http.MethodPut)
		}

		expectedPath := "/test.txt"
		if r.URL.Path != expectedPath {
			t.Errorf("Path = %v, want %v", r.URL.Path, expectedPath)
		}

		// Read request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("Error reading request body: %v", err)
		}

		expected := "test content"
		if string(body) != expected {
			t.Errorf("Body = %v, want %v", string(body), expected)
		}

		// Send success response
		response := BuzzHeavierResponse{
			Code: 200,
			Data: struct {
				ID string `json:"id"`
			}{
				ID: "abc123",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer ts.Close()

	provider, err := New(map[string]interface{}{
		"upload_url":        ts.URL,
		"download_base_url": "https://buzzheavier.com",
		"timeout":           "5s",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	file := bytes.NewReader([]byte("test content"))
	filePath := "/path/to/test.txt"

	response, err := provider.Upload(ctx, filePath, file, int64(file.Len()))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	expected := "https://buzzheavier.com/abc123"
	if response.URL != expected {
		t.Errorf("Upload() URL = %v, want %v", response.URL, expected)
	}

	// Verify response structure
	if response.ID != "abc123" {
		t.Errorf("Upload() ID = %v, want %v", response.ID, "abc123")
	}

	if response.DownloadURL != expected {
		t.Errorf("Upload() DownloadURL = %v, want %v", response.DownloadURL, expected)
	}

	// Verify metadata
	if response.Metadata == nil {
		t.Error("Upload() Metadata should not be nil")
	}

	if response.Metadata["provider"] != "BuzzHeavier" {
		t.Errorf("Upload() Metadata provider = %v, want %v", response.Metadata["provider"], "BuzzHeavier")
	}

	if response.Metadata["original_name"] != "test.txt" {
		t.Errorf("Upload() Metadata original_name = %v, want %v", response.Metadata["original_name"], "test.txt")
	}
}

func TestBuzzHeavierProvider_Upload_HttpError(t *testing.T) {
	// Mock server that returns error
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer ts.Close()

	provider, err := New(map[string]interface{}{
		"upload_url": ts.URL,
		"timeout":    "5s",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	file := bytes.NewReader([]byte("test content"))
	filePath := "/path/to/test.txt"

	_, err = provider.Upload(ctx, filePath, file, int64(file.Len()))
	if err == nil {
		t.Error("Upload() should return error for HTTP 500")
	}

	expectedErr := "upload failed with status 500"
	if err.Error()[:len(expectedErr)] != expectedErr {
		t.Errorf("Error = %v, want to contain %v", err.Error(), expectedErr)
	}
}

func TestBuzzHeavierProvider_Upload_InvalidJSON(t *testing.T) {
	// Mock server that returns invalid JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{invalid json}"))
	}))
	defer ts.Close()

	provider, err := New(map[string]interface{}{
		"upload_url": ts.URL,
		"timeout":    "5s",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	file := bytes.NewReader([]byte("test content"))
	filePath := "/path/to/test.txt"

	_, err = provider.Upload(ctx, filePath, file, int64(file.Len()))
	if err == nil {
		t.Error("Upload() should return error for invalid JSON")
	}

	expectedErr := "failed to parse response"
	if err.Error()[:len(expectedErr)] != expectedErr {
		t.Errorf("Error = %v, want to contain %v", err.Error(), expectedErr)
	}
}

func TestBuzzHeavierProvider_Upload_BadResponseCode(t *testing.T) {
	// Mock server that returns error code in JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := BuzzHeavierResponse{
			Code: 500,
			Data: struct {
				ID string `json:"id"`
			}{
				ID: "",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))
	defer ts.Close()

	provider, err := New(map[string]interface{}{
		"upload_url": ts.URL,
		"timeout":    "5s",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	file := bytes.NewReader([]byte("test content"))
	filePath := "/path/to/test.txt"

	_, err = provider.Upload(ctx, filePath, file, int64(file.Len()))
	if err == nil {
		t.Error("Upload() should return error for response code 500")
	}

	expectedErr := "upload failed with code 500 (code: 500)"
	if err.Error() != expectedErr {
		t.Errorf("Error = %v, want %v", err.Error(), expectedErr)
	}
}

func TestBuzzHeavierProvider_ValidateFile(t *testing.T) {
	provider, err := New(map[string]interface{}{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()

	// Test valid file size
	err = provider.ValidateFile(ctx, "/test.txt", int64(1024))
	if err != nil {
		t.Errorf("ValidateFile() for small file should not return error, got %v", err)
	}

	// Test file too large
	largeSize := int64(20 * 1024 * 1024 * 1024) // 20GB, larger than default 10GB
	err = provider.ValidateFile(ctx, "/large.txt", largeSize)
	if err == nil {
		t.Error("ValidateFile() for oversized file should return error")
	}

	// Verify it's a ProviderError with correct type
	var provErr *providers.ProviderError
	if !errors.As(err, &provErr) {
		t.Error("ValidateFile() should return a ProviderError")
	} else {
		if provErr.Type != providers.ErrorTypeFileTooLarge {
			t.Errorf("ProviderError type = %v, want %v", provErr.Type, providers.ErrorTypeFileTooLarge)
		}
	}
}

func TestBuzzHeavierProvider_GetMaxFileSize(t *testing.T) {
	provider, err := New(map[string]interface{}{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	maxSize := provider.GetMaxFileSize()
	expected := int64(10 * 1024 * 1024 * 1024) // 10GB default
	if maxSize != expected {
		t.Errorf("GetMaxFileSize() = %v, want %v", maxSize, expected)
	}

	// Test custom max size
	customSize := int64(5 * 1024 * 1024 * 1024) // 5GB
	provider, err = New(map[string]interface{}{
		"max_file_size": customSize,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	maxSize = provider.GetMaxFileSize()
	if maxSize != customSize {
		t.Errorf("GetMaxFileSize() with custom size = %v, want %v", maxSize, customSize)
	}
}

func TestBuzzHeavierProvider_GetSupportedExtensions(t *testing.T) {
	provider, err := New(map[string]interface{}{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	extensions := provider.GetSupportedExtensions()
	if len(extensions) != 1 || extensions[0] != "*" {
		t.Errorf("GetSupportedExtensions() = %v, want [*]", extensions)
	}
}