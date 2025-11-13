# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository.

## Development Commands

### Build and Test
```bash
# Always verify compilation after changes (required)
go build ./...

# Build binary
go build -o woof .

# Run all tests
go test ./...

# Run tests for specific package
go test ./pkg/providers/buzzheavier/

# Run tests with verbose output
go test -v ./...

# Run a single test
go test -run TestName ./path/to/package/
```

### Installation
```bash
# Install locally (builds to $GOPATH/bin)
go install .
```

## Architecture Overview

### CLI-First Design
Woof is designed to work entirely via command-line flags without any configuration files. Configuration is **opt-in only** - users must explicitly specify `--config` to load YAML files. All features are accessible through CLI flags.

Key CLI flags:
- `-f, --file`: Files to upload (supports glob patterns)
- `-d, --folder`: Directories to upload  
- `--all`: Use all available providers (no config needed)
- `--providers`: Specific providers to use
- `-c, --concurrency`: Max parallel uploads
- `-v, --verbose`: Enable structured logging with timestamps

### Core Upload Flow
1. **CLI Layer** (`cmd/upload.go`): Parses flags, validates paths, selects providers
2. **Uploader** (`internal/uploader/pool.go`): Manages concurrent uploads with semaphore-controlled goroutines
3. **Providers** (`pkg/providers/`): Individual provider implementations wrapped in consistency layer
4. **Logging** (`internal/logging/`): Category-based structured logging throughout

### Provider System
Providers implement a structured interface that returns rich metadata:

```go
type Provider interface {
    Name() string
    Upload(ctx context.Context, filePath string, file io.Reader, size int64) (*ProviderResponse, error)
    ValidateFile(ctx context.Context, filePath string, size int64) error
    GetMaxFileSize() int64
    GetSupportedExtensions() []string
}
```

**Provider Consistency Wrapper** (`internal/providers/wrapper.go`) automatically:
- Validates files before upload (size, extensions, capabilities)
- Retries failed uploads with exponential backoff
- Enhances responses with metadata (timestamps, provider info)
- Standardizes error handling and categorization

### Concurrency Model
- **Semaphore-controlled**: Uses `golang.org/x/sync/semaphore` to limit concurrent uploads
- **Errgroup management**: Coordinates goroutines and error propagation
- **Progress tracking**: Channel-based progress reporting via `progressReader` wrapper

### Logging System
Professional logging with sirupsen/logrus, initialized in `cmd/upload.go`:

```go
// Available logging categories
logging.UploadStart(filename, size)
logging.UploadComplete(filename, url, duration)
logging.HTTPRequest(method, url, headers)
logging.FileValidation(path, validationType, err)
logging.ConfigLoad(source, values)
```

**Categories**: UPLOAD, NETWORK, FILES, CONFIG, CLI, ERROR

## Adding New Providers

1. Create package under `pkg/providers/{name}/`
2. Implement `Provider` interface with structured `ProviderResponse` returns
3. Add factory case in `pkg/providers/factory.go`
4. Add to `CreateAllProviders()` for `--all` flag support
5. Add provider defaults in `internal/config/config.go` (optional)
6. Use base provider helpers from `internal/providers/base.go` for HTTP operations
7. Categorize errors using `ErrorTypeNetwork`, `ErrorTypeAPI`, etc.
8. Write tests with mock HTTP servers
9. Update `.woof.yaml` example configuration (optional)

**Required methods**:
- `Name()`: Provider identifier
- `Upload()`: Return `*ProviderResponse` with URL, metadata, provider data
- `ValidateFile()`: Check file size, extensions against provider limits
- `GetMaxFileSize()`: Return maximum file size in bytes (0 for unlimited)
- `GetSupportedExtensions()`: Return supported file extensions (["*"] for all types)

**Provider capabilities**:
- Report accurate file size limits (int64, 0 = unlimited)
- Report supported extensions (use ["*"] for all file types)
- Return structured `ProviderResponse` with metadata
- Categorize errors using provider error types
- Support context cancellation for timeout handling

## Code Organization

- `cmd/`: CLI commands (Cobra-based)
- `internal/`: Private application code
  - `uploader/`: Core upload engine and concurrency
  - `providers/`: Provider infrastructure (types, base, wrapper)
  - `config/`: Configuration management (opt-in)
  - `logging/`: Structured logging wrapper
  - `output/`: Result formatting
- `pkg/`: Public/reusable packages
  - `providers/`: Provider implementations and factory
    - `buzzheavier/`: BuzzHeavier provider (PUT-based, size limits)
    - `gofile/`: GoFile provider (multipart, unlimited size)

## Important Rules

1. **Always use `go build ./...`** after making changes to verify compilation
2. **Never use emojis** in code or documentation
3. **CLI-first**: All features must work without configuration files
4. **Structured responses**: Providers must return `*ProviderResponse` with metadata
5. **Error categorization**: Use typed errors from `internal/providers/types.go`
6. **Path validation**: `--file` flags must point to files, `--folder` to directories
