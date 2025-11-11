package output

import (
	"fmt"
	"os"
	"strings"

	"github.com/parnexcodes/woof/internal/uploader"
)

// Handler interface for different output formats
type Handler interface {
	HandleResult(result uploader.UploadResult) error
	HandleProgress(progress uploader.ProgressInfo) error
	Close() error
}

// NewHandler creates a new output handler for the specified format
func NewHandler(format string) (Handler, error) {
	switch strings.ToLower(format) {
	case "json":
		return NewJSONHandler(os.Stdout), nil
	case "text":
		return NewTextHandler(os.Stdout), nil
	default:
		return nil, fmt.Errorf("unsupported output format: %s", format)
	}
}