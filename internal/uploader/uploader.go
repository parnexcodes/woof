package uploader

import (
	"context"
	"io"
	"time"
)

// UploadResult represents the result of a file upload
type UploadResult struct {
	FileName    string        `json:"filename"`
	FilePath    string        `json:"filepath"`
	Size        int64         `json:"size"`
	URL         string        `json:"url"`
	Provider    string        `json:"provider"`
	Duration    time.Duration `json:"duration"`
	Error       error         `json:"error,omitempty"`
	UploadTime  time.Time     `json:"upload_time"`
	ProgressInfo interface{} `json:"-"`
}

// ProgressInfo represents upload progress information
type ProgressInfo struct {
	FileName      string  `json:"filename"`
	BytesUploaded int64   `json:"bytes_uploaded"`
	TotalBytes    int64   `json:"total_bytes"`
	Percentage    float64 `json:"percentage"`
	Speed         float64 `json:"speed"` // bytes per second
}

// Provider interface for different file hosting services
type Provider interface {
	Name() string
	Upload(ctx context.Context, filePath string, file io.Reader, size int64) (string, error)
}

// FileInfo represents information about a file to be uploaded
type FileInfo struct {
	Path     string
	Name     string
	Size     int64
	Modified time.Time
	IsDir    bool
}

// Scanner interface for scanning files and directories
type Scanner interface {
	Scan(ctx context.Context, paths []string) (<-chan FileInfo, <-chan error)
}

// UploadConfig holds configuration for upload operations
type UploadConfig struct {
	Concurrency   int
	Providers     []Provider
	OutputFormat  string
	Verbose       bool
	RetryAttempts int
	RetryDelay    time.Duration
}

// Uploader interface for upload operations
type Uploader interface {
	Upload(ctx context.Context, paths []string, config UploadConfig) (<-chan UploadResult, <-chan ProgressInfo, error)
	GetProgress() <-chan ProgressInfo
}