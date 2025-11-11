package uploader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultScanner implements the Scanner interface
type DefaultScanner struct{}

// Scan scans the given paths and returns channels for file info and errors
func (s *DefaultScanner) Scan(ctx context.Context, paths []string) (<-chan FileInfo, <-chan error) {
	fileCh := make(chan FileInfo, 100)
	errCh := make(chan error, 10)

	go func() {
		defer close(fileCh)
		defer close(errCh)

		for _, path := range paths {
			select {
			case <-ctx.Done():
				return
			default:
			}

			err := s.walkPath(ctx, path, fileCh)
			if err != nil {
				select {
				case errCh <- fmt.Errorf("failed to scan path %s: %w", path, err):
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return fileCh, errCh
}

func (s *DefaultScanner) walkPath(ctx context.Context, root string, fileCh chan<- FileInfo) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info == nil {
			return nil
		}

		fileInfo := FileInfo{
			Path:     path,
			Name:     info.Name(),
			Size:     info.Size(),
			Modified: info.ModTime(),
			IsDir:    info.IsDir(),
		}

		select {
		case fileCh <- fileInfo:
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	})
}