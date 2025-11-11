package providers

import (
	"errors"
	"time"
)

// ProviderResponse represents a structured upload response across all providers
type ProviderResponse struct {
	// Primary download URL
	URL string `json:"url"`

	// Optional but standard fields
	DownloadURL string `json:"download_url,omitempty"`
	DeleteURL   string `json:"delete_url,omitempty"`
	ID          string `json:"id,omitempty"`
	Expires     *time.Time `json:"expires,omitempty"`

	// Provider-specific information
	Metadata     map[string]string `json:"metadata,omitempty"`
	ProviderData interface{}       `json:"provider_data,omitempty"`
}

// ErrorType represents different categories of provider errors
type ErrorType int

const (
	ErrorTypeUnknown ErrorType = iota
	ErrorTypeNetwork           // Network connectivity issues
	ErrorTypeAPI               // API-level errors from provider
	ErrorTypeAuthentication    // Authentication or authorization failures
	ErrorTypeQuota             // Quota exceeded or rate limiting
	ErrorTypeFileTooLarge      // File exceeds provider size limits
	ErrorTypeUnsupported       // File type or format not supported
	ErrorTypeTemporary         // Temporary provider issue (retryable)
)

// ProviderError represents a structured provider error
type ProviderError struct {
	Type      ErrorType `json:"type"`
	Code      string    `json:"code"`      // Provider-specific error code
	Message   string    `json:"message"`   // Human-readable error message
	Retryable bool      `json:"retryable"` // Whether this error is retryable
	Cause     error     `json:"-"`         // Original error for logging
}

// Error implements the error interface
func (pe *ProviderError) Error() string {
	if pe.Code != "" {
		return pe.Message + " (code: " + pe.Code + ")"
	}
	return pe.Message
}

// Unwrap returns the underlying cause
func (pe *ProviderError) Unwrap() error {
	return pe.Cause
}

// Is checks if the error matches the target
func (pe *ProviderError) Is(target error) bool {
	if targetProvider, ok := target.(*ProviderError); ok {
		return pe.Type == targetProvider.Type
	}
	return false
}

// NewProviderError creates a new ProviderError
func NewProviderError(errorType ErrorType, code, message string, retryable bool, cause error) *ProviderError {
	return &ProviderError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Retryable: retryable,
		Cause:     cause,
	}
}

// Predefined error constructors
func NewNetworkError(message string, cause error) *ProviderError {
	return NewProviderError(ErrorTypeNetwork, "", message, true, cause)
}

func NewAPIError(code, message string, cause error) *ProviderError {
	return NewProviderError(ErrorTypeAPI, code, message, false, cause)
}

func NewAuthenticationError(message string, cause error) *ProviderError {
	return NewProviderError(ErrorTypeAuthentication, "", message, false, cause)
}

func NewQuotaError(message string, cause error) *ProviderError {
	return NewProviderError(ErrorTypeQuota, "", message, true, cause)
}

func NewFileTooLargeError(message string, cause error) *ProviderError {
	return NewProviderError(ErrorTypeFileTooLarge, "", message, false, cause)
}

func NewUnsupportedError(message string, cause error) *ProviderError {
	return NewProviderError(ErrorTypeUnsupported, "", message, false, cause)
}

func NewTemporaryError(message string, cause error) *ProviderError {
	return NewProviderError(ErrorTypeTemporary, "", message, true, cause)
}

// IsRetryable checks if the error is retryable
func IsRetryable(err error) bool {
	var provErr *ProviderError
	if errors.As(err, &provErr) {
		return provErr.Retryable
	}
	return false
}

// GetErrorType extracts the ErrorType from an error
func GetErrorType(err error) ErrorType {
	var provErr *ProviderError
	if errors.As(err, &provErr) {
		return provErr.Type
	}
	return ErrorTypeUnknown
}