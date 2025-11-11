package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/parnexcodes/woof/internal/uploader"
)

// formatBytes formats bytes into human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// JSONHandler implements Handler for JSON output
type JSONHandler struct {
	encoder   *json.Encoder
	first     bool
	progress  bool
	output    io.Writer
}

// NewJSONHandler creates a new JSON handler
func NewJSONHandler(w io.Writer) *JSONHandler {
	return &JSONHandler{
		encoder:  json.NewEncoder(w),
		first:    true,
		progress: false,
		output:   w,
	}
}

// HandleResult handles an upload result in JSON format
func (j *JSONHandler) HandleResult(result uploader.UploadResult) error {
	if j.first {
		fmt.Fprintf(j.output, "[")
		j.first = false
	} else {
		fmt.Fprintf(j.output, ",")
	}

	result.ProgressInfo = nil // Remove progress info from result output
	return j.encoder.Encode(result)
}

// HandleProgress handles progress information in JSON format
func (j *JSONHandler) HandleProgress(progress uploader.ProgressInfo) error {
	// Progress updates are streamed as separate JSON objects
	if !j.progress {
		// Start progress stream marker
		fmt.Fprintf(j.output, "\n{\"type\":\"progress_stream\",\"items\":[")
		j.progress = true
		j.first = true
	}

	if j.first {
		j.first = false
	} else {
		fmt.Fprintf(j.output, ",")
	}

	item := map[string]interface{}{
		"type":     "progress",
		"filename": progress.FileName,
		"bytes":    progress.BytesUploaded,
		"total":    progress.TotalBytes,
		"percent":  progress.Percentage,
		"speed":    progress.Speed,
	}

	return j.encoder.Encode(item)
}

// Close closes the JSON handler
func (j *JSONHandler) Close() error {
	if !j.first {
		fmt.Fprintf(j.output, "]")
	}
	return nil
}

// TextHandler implements Handler for human-readable text output
type TextHandler struct {
	output io.Writer
}

// NewTextHandler creates a new text handler
func NewTextHandler(w io.Writer) *TextHandler {
	return &TextHandler{
		output: w,
	}
}

// HandleResult handles an upload result in text format
func (t *TextHandler) HandleResult(result uploader.UploadResult) error {
	if result.Error != nil {
		fmt.Fprintf(t.output, "ERROR %s: %v\n", result.FileName, result.Error)
		return nil
	}

	fmt.Fprintf(t.output,
		"SUCCESS %s (%s) -> %s [%s via %s]\n",
		result.FileName,
		formatBytes(result.Size),
		result.URL,
		result.Duration.Round(time.Millisecond),
		result.Provider,
	)
	return nil
}

// HandleProgress handles progress information in text format
func (t *TextHandler) HandleProgress(progress uploader.ProgressInfo) error {
	// Simple progress bar for text output
	barWidth := 40

	// Handle edge cases for percentage
	percentage := progress.Percentage
	if percentage < 0 {
		percentage = 0
	} else if percentage > 100 {
		percentage = 100
	}

	filled := int(percentage / 100.0 * float64(barWidth))
	if filled < 0 {
		filled = 0
	} else if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("=", filled) + strings.Repeat(" ", barWidth-filled)

	fmt.Fprintf(t.output, "\r[%s] %s %.1f%% (%s/%s)",
		bar,
		progress.FileName,
		percentage,
		formatBytes(progress.BytesUploaded),
		formatBytes(progress.TotalBytes),
	)

	if progress.BytesUploaded >= progress.TotalBytes {
		fmt.Fprintf(t.output, "\n")
	}
	return nil
}

// Close closes the text handler
func (t *TextHandler) Close() error {
	return nil
}