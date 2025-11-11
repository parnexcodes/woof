# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Guidelines

### Important Rules
1. **Always use `go build ./...`** for checking any compilation errors after making changes
2. **Never use emojis** in code or documentation (including README.md and this file)

### Build Testing
After any code changes, always verify compilation with:
```bash
go build ./...
```

This ensures all packages compile correctly before running tests or committing changes.

## Common Development Commands

### Building
```bash
# Build to /out directory
go build -o out/woof .

# Or build without output directory (not recommended)
go build -o woof .
```

### Running Tests
```bash
# Run all tests
go test ./...

# Run tests for specific package
go test ./pkg/providers/buzzheavier/

# Run tests with verbose output
go test -v ./...
```

### Installation
```bash
# Install locally (builds to $GOPATH/bin)
go install .

# Install from remote
go install github.com/parnexcodes/woof@latest

# Or build manually to /out directory and copy:
go build -o out/woof .
cp out/woof /usr/local/bin/
```

## Project Architecture

### High-Level Design
Woof is a high-performance CLI file uploader built with Go and Cobra. The application uses a CLI-first, interface-based design that supports multiple file hosting providers with concurrent upload capabilities. Features professional logging with sirupsen/logrus for colorful, timestamped, structured output. No configuration is required - all features are accessible via command-line flags. Configuration files are opt-in and must be explicitly specified.

### Core Components

#### 1. CLI Layer (`cmd/`)
- **main.go**: Entry point that delegates to cmd package
- **cmd/root.go**: Root command with global flags, Cobra initialization, and Viper configuration binding
- **cmd/upload.go**: Upload command with flag-based file/folder selection, provider management, and validation
- **cmd/upload_test.go**: Comprehensive CLI tests including validation and flag parsing
- **cmd/version.go**: Version command

### New CLI Features
- **Flag-based upload**: `--file/-f` for files, `--folder/-d` for directories
- **Glob pattern support**: Built-in wildcards for flexible file selection
- **Provider selection**: `--all` uses all providers, `--providers` for specific ones
- **Strict validation**: Path validation with helpful error messages
- **CLI-first design**: Works without any configuration; YAML is opt-in (must specify --config flag)

#### 2. Core Upload Engine (`internal/uploader/`)
- **uploader.go**: Core interfaces and types (Provider, Uploader, Scanner, UploadResult)
  - Provider interface now returns structured `ProviderResponse` with metadata
  - UploadResult includes both URL and full ProviderResponse
- **pool.go**: DefaultUploader implementation with semaphore-controlled concurrency using errgroup
  - Handles new structured provider responses
  - Maintains backward compatibility with URL field
- **scanner.go**: File and directory scanning implementation

#### 2.5. Provider System (`internal/providers/`)
- **types.go**: ProviderResponse structure and typed error system
  - `ProviderResponse`: Structured upload responses with URL, metadata, provider data
  - `ProviderError`: Typed errors with categories (Network, API, Authentication, etc.)
  - Error helpers for consistent error creation and handling
- **base.go**: Base provider implementation with common functionality
  - HTTP operations with standardized error handling
  - File validation and capability checking
  - Response parsing and metadata helper methods
- **wrapper.go**: Consistency wrapper for standardized provider behavior
  - Pre-upload validation with capability checking
  - Automatic retry logic with exponential backoff
  - Response enhancement and standardization
  - Configurable behavior (validation, retries, metadata)

#### 3. Configuration Management (`internal/config/`)
- Uses Viper for optional configuration loading from YAML files and environment variables
- **Opt-in configuration**: Config files are only loaded when `--config` flag is explicitly provided
- **CLI-first**: All functionality available without configuration files
- Default values for providers (with official BuzzHeavier URLs), upload settings, and global options

#### 4. Logging System (`internal/logging/`)
- Uses sirupsen/logrus for professional structured logging
- **Smart formatting**: Colored text for TTY, JSON for non-TTY output
- **Categorized logging**: UPLOAD, NETWORK, FILES, CONFIG, CLI, ERROR categories
- **Rich timestamps**: Millisecond precision timestamps in verbose mode
- **Structured fields**: Key-value pairs for filtering and parsing
- **Level control**: Debug level in verbose mode, Error level otherwise
- **Clean output**: No caller clutter, user-friendly formatting

#### 5. Provider System (`pkg/providers/`)
- **factory.go**: Factory pattern for creating provider instances from configuration or defaults
  - `CreateAllProviders()`: New method for CLI-first provider access without config
  - Automatic consistency wrapper application with configurable settings
  - Support for creating providers with/without consistency wrapper
- **buzzheavier/**: BuzzHeavier provider implementation
  - Implements the new structured Provider interface
  - Returns rich ProviderResponse with metadata (upload duration, file size, etc.)
  - Proper error categorization and structured error handling
  - Built-in official URL defaults (no config required)
- Individual provider packages implement the `Provider` interface

### Key Interfaces

```go
type Provider interface {
    Name() string
    Upload(ctx context.Context, filePath string, file io.Reader, size int64) (*ProviderResponse, error)
    ValidateFile(ctx context.Context, filePath string, size int64) error
    GetMaxFileSize() int64
    GetSupportedExtensions() []string
}

type ProviderResponse struct {
    URL          string            `json:"url"`
    DownloadURL  string            `json:"download_url,omitempty"`
    DeleteURL    string            `json:"delete_url,omitempty"`
    ID           string            `json:"id,omitempty"`
    Expires      *time.Time        `json:"expires,omitempty"`
    Metadata     map[string]string `json:"metadata,omitempty"`
    ProviderData interface{}       `json:"provider_data,omitempty"`
}

type Uploader interface {
    Upload(ctx context.Context, paths []string, config UploadConfig) (<-chan UploadResult, <-chan ProgressInfo, error)
    GetProgress() <-chan ProgressInfo
}
```

### Concurrency Model
- Uses `golang.org/x/sync/semaphore` for upload concurrency control
- `errgroup` for goroutine management and error propagation
- Channel-based progress reporting with buffering
- Progress is tracked via a custom `progressReader` that wraps file readers

### Provider Implementation (BuzzHeavier)
- Implements structured Provider interface with rich metadata responses
- PUT-based file uploads to BuzzHeavier service with built-in official URLs
- Constructs download URLs from API response IDs
- Returns ProviderResponse with upload metadata (duration, size timestamps, etc.)
- Structured error handling with proper categorization and retry information
- Full test coverage with mock HTTP servers
- Configurable timeouts, max file sizes, and base URLs (optional - defaults to official endpoints)

### Provider Consistency System
- **Structured Responses**: All providers return consistent `ProviderResponse` objects
- **Automatic Validation**: File size, extension, and provider capability checking
- **Retry Logic**: Exponential backoff for retryable errors with configurable attempts
- **Error Categorization**: Typed errors with retryability and context information
- **Metadata Enhancement**: Automatic addition of upload timestamps, file info, provider details
- **Consistency Wrapper**: Optional wrapper that ensures standardized behavior across providers

### Configuration Schema (Optional)
```yaml
# NOTE: Configuration is opt-in - must specify --config flag to load
# Use --all flag for CLI-first operation without any config file
concurrency: int          # Max parallel uploads
verbose: bool            # Verbose logging
output: string           # text or json
providers:
  - name: string         # Provider name
    enabled: bool        # Enable/disable provider (ignored with --all flag)
    settings: map        # Provider-specific settings (optional - defaults used)
upload:
  retry_attempts: int    # Number of retry attempts
  retry_delay: duration # Delay between retries
  chunk_size: int64      # Upload chunk size
  timeout: duration      # Upload timeout
```

### CLI Command Structure
```bash
woof upload [flags]

# Required flags (must specify files and/or folders):
-f, --file strings     # Files to upload (supports glob patterns)
-d, --folder strings   # Folders to upload

# Provider selection:
--all                   # Use all available providers (no config needed)
--providers strings     # Specific providers to require

# Global options:
-c, --concurrency int   # Max parallel uploads
-o, --output string     # Output format (text, json)
-v, --verbose           # Verbose logging (shows colored timestamps and structured output)
--config string         # Config file (required to use YAML configuration)
```

### Logging Features
The `--verbose/-v` flag enables professional logging with:
- **Colored output**: Level-based coloring (Debug=white, Info=cyan, Error=red)
- **Timestamps**: Millisecond precision timestamps in `YYYY-MM-DD HH:MM:SS.mmm` format
- **Structured fields**: Key-value pairs like `category=UPLOAD filename=document.txt duration_ms=1234`
- **Category organization**: UPLOAD, NETWORK, FILES, CONFIG, CLI, ERROR
- **Smart formatting**: Text format for terminals, JSON for piping
- **Clean mode**: Non-verbose shows only essential output without debug information

## Development Notes

### Adding New Providers
1. Create a new package under `pkg/providers/`
2. Implement the `Provider` interface:
   - `Name()`: Return provider name
   - `Upload()`: Return structured `*ProviderResponse, error`
   - `ValidateFile()`: Pre-upload validation
   - `GetMaxFileSize()`: Return supported max file size
   - `GetSupportedExtensions()`: Return supported file extensions
3. Use the internal base provider utilities for common HTTP operations and error handling
4. Add factory case in `pkg/providers/factory.go`
5. Add provider to `CreateAllProviders()` method for `--all` flag support
6. Add provider configuration defaults in `internal/config/config.go` (optional)
7. Write comprehensive tests with mock HTTP servers
8. Test CLI integration without configuration using the new flag system

**Provider Implementation Guidelines:**
- Use structured responses with metadata (upload duration, timestamps, etc.)
- Categorize errors using the provider error types (`ErrorTypeNetwork`, `ErrorTypeAPI`, etc.)
- Implement proper file validation and capability reporting
- Include provider-specific data in `ProviderResponse.ProviderData`
- Use the base provider helper methods for HTTP operations and logging

### Code Organization
- Internal packages (`internal/`) are for application-private code
- Public packages (`pkg/`) are for reusable components like providers
- Clear separation between CLI logic, core functionality, logging, and extensions
- **CLI-First Architecture**: Flag-driven command interface with opt-in configuration via --config flag

**Package Structure:**
- `internal/logging/`: Professional logging wrapper around sirupsen/logrus
- `internal/uploader/`: Core upload engine with concurrency control
- `internal/providers/`: Provider system infrastructure (types, base, wrapper)
- `internal/config/`: Configuration management and validation
- `internal/output/`: Result formatting and progress display
- `cmd/`: CLI commands and user interface
- `pkg/providers/`: File hosting provider implementations
  - `buzzheavier/`: BuzzHeavier provider implementation
  - `factory.go`: Provider factory with consistency wrapper support

### Error Handling & Validation
- Structured error wrapping with context using `fmt.Errorf`
- Individual file upload failures don't abort the entire operation
- **Path Validation**: Strict checking that `--file` flags point to files and `--folder` flags point to directories
- **Helpful Error Messages**: Clear guidance on missing arguments, invalid paths, and configuration options
- Provider fallbacks: tries each provider until one succeeds
- Progress bar improvements: Handles edge cases for percentage calculations

### Using the Logging System
The logging system is initialized in `cmd/upload.go` and provides category-based logging:

```go
// Logging is automatically initialized with verbose flag
logging.Init(viper.GetBool("verbose"), os.Stderr)

// Use category-specific logging functions
logging.UploadStart(filename, size)
logging.ConfigLoad(source, values)
logging.HTTPRequest(method, url, headers)
logging.FileValidation(path, validationType, error)
```

**Available Categories:**
- `UPLOAD`: Upload operations, progress, completion
- `NETWORK`: HTTP requests, responses, API calls
- `FILES`: File scanning, validation, discovery
- `CONFIG`: Configuration loading, provider selection
- `CLI`: Flag processing, command execution
- `ERROR`: Error context with structured details

### Testing Strategy
- **Unit Tests**: Core functionality tests for helpers like `expandGlobPatterns()` and `validatePaths()`
- **CLI Integration Tests**: Test flag parsing, validation, and error scenarios
- **Mock Provider Tests**: Isolate provider logic from network dependencies
- **End-to-End**: Full command execution with real flag combinations including logging output