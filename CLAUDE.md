# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Building
```bash
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
# Install locally
go install .

# Install from remote
go install github.com/parnexcodes/woof@latest
```

## Project Architecture

### High-Level Design
Woof is a high-performance CLI file uploader built with Go and Cobra. The application uses a modular, interface-based design that supports multiple file hosting providers with concurrent upload capabilities.

### Core Components

#### 1. CLI Layer (`cmd/`)
- **main.go**: Entry point that delegates to cmd package
- **cmd/root.go**: Root command with global flags, Cobra initialization, and Viper configuration binding
- **cmd/upload.go**: Upload command implementation
- **cmd/version.go**: Version command

#### 2. Core Upload Engine (`internal/uploader/`)
- **uploader.go**: Core interfaces and types (Provider, Uploader, Scanner, UploadResult)
- **pool.go**: DefaultUploader implementation with semaphore-controlled concurrency using errgroup
- **scanner.go**: File and directory scanning implementation

#### 3. Configuration Management (`internal/config/`)
- Uses Viper for configuration loading from YAML files and environment variables
- Supports configuration in `$HOME/.woof.yaml` or local `.woof.yaml`
- Default values for providers, upload settings, and global options

#### 4. Provider System (`pkg/providers/`)
- **factory.go**: Factory pattern for creating provider instances from configuration
- Individual provider packages implement the `Provider` interface
- Currently supports BuzzHeavier provider

### Key Interfaces

```go
type Provider interface {
    Name() string
    Upload(ctx context.Context, filePath string, file io.Reader, size int64) (string, error)
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
- Implements PUT-based file uploads to BuzzHeavier service
- Constructs download URLs from API response IDs
- Full test coverage with mock HTTP servers
- Configurable timeouts and base URLs

### Configuration Schema
```yaml
concurrency: int          # Max parallel uploads
verbose: bool            # Verbose logging
output: string           # text or json
providers:
  - name: string         # Provider name
    enabled: bool        # Enable/disable provider
    settings: map        # Provider-specific settings
upload:
  retry_attempts: int    # Number of retry attempts
  retry_delay: duration # Delay between retries
  chunk_size: int64      # Upload chunk size
  timeout: duration      # Upload timeout
```

## Development Notes

### Adding New Providers
1. Create a new package under `pkg/providers/`
2. Implement the `Provider` interface
3. Add factory case in `pkg/providers/factory.go`
4. Add provider configuration defaults in `internal/config/config.go`
5. Write comprehensive tests with mock HTTP servers

### Code Organization
- Internal packages (`internal/`) are for application-private code
- Public packages (`pkg/`) are for reusable components like providers
- Clear separation between CLI logic, core functionality, and extensions

### Error Handling
- Structured error wrapping with context using `fmt.Errorf`
- Individual file upload failures don't abort the entire operation
- Provider fallbacks: tries each provider until one succeeds