package buzzheavier

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

		if r.URL.Path != "/test.txt" {
			t.Errorf("Path = %v, want %v", r.URL.Path, "/test.txt")
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

	url, err := provider.Upload(ctx, filePath, file, int64(file.Len()))
	if err != nil {
		t.Fatalf("Upload() error = %v", err)
	}

	expected := "https://buzzheavier.com/abc123"
	if url != expected {
		t.Errorf("Upload() = %v, want %v", url, expected)
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

	expectedErr := "upload failed with code 500"
	if err.Error() != expectedErr {
		t.Errorf("Error = %v, want %v", err.Error(), expectedErr)
	}
}