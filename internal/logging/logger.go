package logging

import (
	"io"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type Logger struct {
	*logrus.Logger
	verbose bool
}

var defaultLogger *Logger

// Categories for consistent logging
const (
	CategoryNetwork  = "NETWORK"
	CategoryProvider = "PROVIDER"
	CategoryFiles    = "FILES"
	CategoryConfig   = "CONFIG"
	CategoryUpload   = "UPLOAD"
	CategoryCLI      = "CLI"
	CategoryError    = "ERROR"
)

// Init initializes the logging system with verbose flag and output destination
func Init(verbose bool, output io.Writer) {
	logger := logrus.New()

	// Configure output
	if output != nil {
		logger.SetOutput(output)
	} else {
		logger.SetOutput(os.Stderr)
	}

	// Configure formatter based on output type and verbose setting
	if isTTY(logger.Out) && verbose {
		// Colored text formatter for interactive verbose mode
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05.000",
			ForceColors:     true,
			DisableColors:   false,
		})
	} else if isTTY(logger.Out) {
		// Colored but minimal for non- verbose
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   false,
			ForceColors:     true,
			DisableColors:   false,
		})
	} else {
		// JSON formatter for non-interactive output
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	}

	// Set log level
	if verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		// Only show errors and above in non-verbose mode
		logger.SetLevel(logrus.ErrorLevel)
	}

	// Disable caller reporting to keep output cleaner
	logger.SetReportCaller(false)

	defaultLogger = &Logger{
		Logger:  logger,
		verbose: verbose,
	}
}

// isTTY checks if the output is a terminal
func isTTY(output io.Writer) bool {
	// Simple check - could be enhanced with more sophisticated detection
	file, ok := output.(*os.File)
	return ok && (file.Fd() == 1 || file.Fd() == 2)
}

// IsVerbose returns whether verbose logging is enabled
func IsVerbose() bool {
	if defaultLogger == nil {
		return false
	}
	return defaultLogger.verbose
}

// Helper function to log with category field
func (l *Logger) logWithCategory(level logrus.Level, category string, message string, fields logrus.Fields) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["category"] = category

	l.WithFields(fields).Log(level, message)
}

// Network Operations Logging Functions
func HTTPRequest(method, url string, headers map[string]string) {
	if !IsVerbose() {
		return
	}
	fields := logrus.Fields{
		"method": method,
		"url":    url,
	}
	if headers != nil && len(headers) > 0 {
		fields["headers"] = headers
	}
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryNetwork, "HTTP request", fields)
}

func HTTPResponse(statusCode int, body string, duration time.Duration) {
	if !IsVerbose() {
		return
	}
	fields := logrus.Fields{
		"status_code": statusCode,
		"duration_ms": duration.Milliseconds(),
	}
	if body != "" {
		// Limit body length for readability
		if len(body) > 200 {
			body = body[:200] + "..."
		}
		fields["body"] = body
	}
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryNetwork, "HTTP response", fields)
}

func ProviderConfig(providerName string, config map[string]interface{}) {
	if !IsVerbose() || len(config) == 0 {
		return
	}
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryProvider, "Provider configuration", logrus.Fields{
		"provider": providerName,
		"config":   config,
	})
}

// File Operations Logging Functions
func FileScan(paths []string) {
	if !IsVerbose() {
		return
	}
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryFiles, "Scanning files", logrus.Fields{
		"path_count": len(paths),
		"paths":      paths,
	})
}

func FileFound(path string, size int64, isDir bool) {
	if !IsVerbose() {
		return
	}
	fileType := "file"
	if isDir {
		fileType = "dir"
	}
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryFiles, "File found", logrus.Fields{
		"path":     path,
		"size":     size,
		"type":     fileType,
	})
}

func FileValidation(path string, validationType string, err error) {
	if !IsVerbose() {
		return
	}
	level := logrus.DebugLevel
	message := "File validation passed"
	if err != nil {
		level = logrus.WarnLevel
		message = "File validation failed"
	}
	defaultLogger.logWithCategory(level, CategoryFiles, message, logrus.Fields{
		"path":            path,
		"validation_type": validationType,
		"error":           err,
	})
}

// Configuration Logging Functions
func ConfigLoad(source string, values interface{}) {
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryConfig, "Loading configuration", logrus.Fields{
		"source":  source,
		"values":  values,
	})
}

func ProviderSelection(mode string, providers []string) {
	if !IsVerbose() {
		return
	}
	fields := logrus.Fields{
		"mode": mode,
	}
	if len(providers) > 0 {
		fields["providers"] = providers
	}
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryConfig, "Provider selection", fields)
}

// Upload Process Logging Functions
func UploadStart(filename string, size int64) {
	defaultLogger.logWithCategory(logrus.InfoLevel, CategoryUpload, "Starting upload", logrus.Fields{
		"filename": filename,
		"size":     size,
	})
}

func UploadProgress(filename string, bytesRead int64, total int64) {
	if !IsVerbose() {
		return
	}
	percentage := float64(bytesRead) / float64(total) * 100
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryUpload, "Upload progress", logrus.Fields{
		"filename":  filename,
		"bytes_read": bytesRead,
		"total":      total,
		"percentage": percentage,
	})
}

func UploadComplete(filename string, url string, duration time.Duration) {
	defaultLogger.logWithCategory(logrus.InfoLevel, CategoryUpload, "Upload completed", logrus.Fields{
		"filename":  filename,
		"url":       url,
		"duration_ms": duration.Milliseconds(),
	})
}

func UploadError(filename string, provider string, err error) {
	defaultLogger.logWithCategory(logrus.ErrorLevel, CategoryUpload, "Upload failed", logrus.Fields{
		"filename": filename,
		"provider": provider,
		"error":    err,
	})
}

// Concurrency Logging Functions
func ConcurrencySettings(workers int, semaphoreSize int) {
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryUpload, "Concurrency settings", logrus.Fields{
		"workers":         workers,
		"semaphore_size": semaphoreSize,
	})
}

func SemaphoreState(acquired int, available int) {
	if !IsVerbose() {
		return
	}
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryUpload, "Semaphore state", logrus.Fields{
		"acquired":  acquired,
		"available": available,
	})
}

// CLI and Flag Processing Logging Functions
func FlagProcessing(flag string, value interface{}) {
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryCLI, "Flag processing", logrus.Fields{
		"flag":  flag,
		"value": value,
	})
}

func CommandExecution(command string, args []string) {
	if !IsVerbose() {
		return
	}
	defaultLogger.logWithCategory(logrus.DebugLevel, CategoryCLI, "Command execution", logrus.Fields{
		"command": command,
		"args":    args,
	})
}

// Error Context Logging Functions
func ErrorContext(context string, err error, details map[string]interface{}) {
	if !IsVerbose() || err == nil {
		return
	}
	fields := logrus.Fields{
		"context": context,
		"error":   err,
	}
	if len(details) > 0 {
		for k, v := range details {
			fields[k] = v
		}
	}
	defaultLogger.logWithCategory(logrus.ErrorLevel, CategoryError, "Error occurred", fields)
}

// General logging methods for direct access
func Info(message string, fields logrus.Fields) {
	defaultLogger.logWithCategory(logrus.InfoLevel, "GENERAL", message, fields)
}

func Debug(message string, fields logrus.Fields) {
	defaultLogger.logWithCategory(logrus.DebugLevel, "GENERAL", message, fields)
}

func Error(message string, fields logrus.Fields) {
	defaultLogger.logWithCategory(logrus.ErrorLevel, "GENERAL", message, fields)
}

func Warn(message string, fields logrus.Fields) {
	defaultLogger.logWithCategory(logrus.WarnLevel, "GENERAL", message, fields)
}