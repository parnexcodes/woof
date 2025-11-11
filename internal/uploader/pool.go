package uploader

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// DefaultUploader implements the Uploader interface
type DefaultUploader struct {
	scanner    Scanner
	progressCh chan ProgressInfo
	mu         sync.Mutex
}

// NewDefaultUploader creates a new DefaultUploader instance
func NewDefaultUploader() *DefaultUploader {
	return &DefaultUploader{
		scanner:    &DefaultScanner{},
		progressCh: make(chan ProgressInfo, 100),
	}
}

// Upload uploads files to multiple providers with concurrency control
func (u *DefaultUploader) Upload(ctx context.Context, paths []string, config UploadConfig) (<-chan UploadResult, <-chan ProgressInfo, error) {
	// Create result channel
	resultCh := make(chan UploadResult, config.Concurrency*2)

	// Create semaphore for concurrency control
	sem := semaphore.NewWeighted(int64(config.Concurrency))

	// Create error group
	g, ctx := errgroup.WithContext(ctx)

	// Scan for files
	fileCh, errCh := u.scanner.Scan(ctx, paths)

	// Start worker goroutines
	g.Go(func() error {
		defer close(resultCh)
		defer close(u.progressCh)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()

			case fileInfo, ok := <-fileCh:
				if !ok {
					return nil // No more files to process
				}

				if fileInfo.IsDir {
					continue // Skip directories
				}

				// Acquire semaphore slot
				if err := sem.Acquire(ctx, 1); err != nil {
					return err
				}

				// Launch upload goroutine
				g.Go(func() error {
					defer sem.Release(1)
					return u.uploadFile(ctx, fileInfo, config, resultCh)
				})

			case err := <-errCh:
				if err != nil {
					return fmt.Errorf("scan error: %w", err)
				}
			}
		}
	})

	// Wait for all uploads to complete
	go func() {
		if err := g.Wait(); err != nil && err != context.Canceled {
			resultCh <- UploadResult{
				Error: fmt.Errorf("upload failed: %w", err),
			}
		}
	}()

	return resultCh, u.progressCh, nil
}

func (u *DefaultUploader) uploadFile(ctx context.Context, fileInfo FileInfo, config UploadConfig, resultCh chan<- UploadResult) error {
	// Open file
	file, err := os.Open(fileInfo.Path)
	if err != nil {
		resultCh <- UploadResult{
			FileName: fileInfo.Name,
			FilePath: fileInfo.Path,
			Error:    fmt.Errorf("failed to open file: %w", err),
		}
		return nil // Don't fail the entire operation for one file
	}
	defer file.Close()

	// Try each provider until one succeeds
	var lastErr error
	for _, provider := range config.Providers {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		start := time.Now()

		// Create progress tracking reader
		progressReader := &progressReader{
			reader:    file,
			totalSize: fileInfo.Size,
			onProgress: func(bytesRead int64) {
				progress := ProgressInfo{
					FileName:      fileInfo.Name,
					BytesUploaded: bytesRead,
					TotalBytes:    fileInfo.Size,
					Percentage:    float64(bytesRead) / float64(fileInfo.Size) * 100,
				}

				select {
				case u.progressCh <- progress:
				default:
					// Progress channel full, skip this update
				}
			},
		}

		// Reset file offset for each provider
		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			lastErr = err
			continue
		}

		// Upload to provider
		url, err := provider.Upload(ctx, fileInfo.Path, progressReader, fileInfo.Size)
		duration := time.Since(start)

		if err != nil {
			lastErr = err
			if config.Verbose {
				fmt.Printf("Upload to %s failed for %s: %v\n", provider.Name(), fileInfo.Name, err)
			}
			continue
		}

		// Success!
		result := UploadResult{
			FileName:   fileInfo.Name,
			FilePath:   fileInfo.Path,
			Size:       fileInfo.Size,
			URL:        url,
			Provider:   provider.Name(),
			Duration:   duration,
			UploadTime: time.Now(),
		}

		select {
		case resultCh <- result:
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	}

	// All providers failed
	resultCh <- UploadResult{
		FileName: fileInfo.Name,
		FilePath: fileInfo.Path,
		Error:    fmt.Errorf("all providers failed, last error: %w", lastErr),
	}

	return nil
}

// GetProgress returns the progress channel
func (u *DefaultUploader) GetProgress() <-chan ProgressInfo {
	return u.progressCh
}

// progressReader wraps an io.Reader to track read progress
type progressReader struct {
	reader     io.Reader
	totalSize  int64
	bytesRead  int64
	onProgress func(int64)
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	pr.bytesRead += int64(n)
	pr.onProgress(pr.bytesRead)
	return n, err
}