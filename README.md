# Woof

High-performance parallel file uploader CLI in Go with Cobra.

> **⚠️ Development Status**: This project is currently in active development. Expect major changes and potential breaking updates before the v1.0 release. Features are being added rapidly and the CLI interface may evolve significantly during this phase.

## Features

- High-performance concurrent uploads using Go goroutines
- CLI-first design - no configuration required
- Support for files and directories with explicit flag separation
- Glob pattern support for flexible file selection
- Multiple file hosting providers with provider selection
- Structured provider responses with metadata and rich error information
- Automatic file validation, retry logic, and capability checking
- Provider consistency wrapper with standardized behavior
- Real-time progress tracking
- Multiple output formats (text, JSON)
- Enhanced logging with colorful timestamps and structured output
- Professional logging with sirupsen/logrus
- Optional YAML configuration for advanced users
- Retry mechanism with backoff
- Strict validation with helpful error messages

## Installation

```bash
go install github.com/parnexcodes/woof@latest
```

## Usage

### Basic Upload

```bash
# Upload a single file using the --file flag
woof upload -f file.txt

# Upload multiple files
woof upload -f file1.txt -f file2.txt -f file3.txt

# Upload using glob patterns
woof upload -f "*.txt" -f "./logs/*.log"

# Upload a directory using the --folder flag
woof upload -d ./uploads/

# Use all available providers
woof upload --all -f "*.pdf"

# Use specific provider
woof upload --providers buzzheavier -f document.txt

# Mixed files and folders
woof upload --all -f "*.go" -d ./backups -f README.md

# Upload with custom concurrency
woof upload --all -c 10 -f ./large_files/*

# Upload with JSON output
woof upload --all -o json -f ./files/*

# Get help
woof upload --help
```

### Enhanced Logging

Woof uses professional logging with sirupsen/logrus for beautiful, informative output:

```bash
# Verbose mode shows detailed logging with colors and timestamps
woof upload --all -v -f document.txt

# Regular mode shows clean output without debug information
woof upload --all -f *.pdf

# The logging includes:
# - Colored log levels (DEBUG, INFO, WARN, ERROR)
# - Millisecond precision timestamps
# - Structured fields for filtering
# - Category-based organization (UPLOAD, NETWORK, FILES, CONFIG, CLI)
```

### Configuration (Optional)

Woof works out-of-the-box without any configuration and does **not** auto-load config files. Configuration is opt-in and must be explicitly specified with the `--config` flag.

For advanced users, you can create a `.woof.yaml` file and load it explicitly:

```yaml
# Global settings
concurrency: 5
verbose: false
output: "text"

# Provider configuration
providers:
  - name: "buzzheavier"
    enabled: true
    settings:
      upload_url: "https://w.buzzheavier.com"  # Optional - defaults to official URL
      download_base_url: "https://buzzheavier.com"  # Optional - defaults to official URL
      timeout: "10m"

# Upload settings
upload:
  retry_attempts: 3
  retry_delay: "2s"
  chunk_size: 1048576  # 1MB
  timeout: "30m"
```

**Note:** Configuration is opt-in! Most users don't need any config file. You can use all features directly from CLI:
- `--all` to use all available providers
- `--providers` for specific providers
- `--file`/`--folder` for uploads with glob support

To use a config file, you must specify it explicitly:
```bash
woof upload --config .woof.yaml --all -f "*.pdf"
```

### Available Providers

- **BuzzHeavier**: File hosting service with PUT-based uploads
  - Works out-of-the-box with default URLs (no config needed)
  - Use with `--providers buzzheavier` flag or `--all` to include all providers

### Upload Command

```bash
woof upload [flags]
```

**Flags:**
- `--all`: Use all available providers regardless of configuration
- `-f, --file strings`: Files to upload (can be used multiple times, supports glob patterns)
- `-d, --folder strings`: Folders to upload (can be used multiple times)
- `--providers strings`: Specific providers to use
- `-c, --concurrency int`: Maximum number of parallel uploads (default: 5)
- `-o, --output string`: Output format (text, json) (default: text)
- `--retry-attempts int`: Number of retry attempts per file (default: 3)
- `--retry-delay duration`: Delay between retry attempts (default: 2s)
- `--progress`: Show upload progress (default: true)
- `-v, --verbose`: Verbose output

**Global Flags:**
- `--config string`: Config file (required to use YAML configuration)

## Project Structure

```
woof/
├── cmd/                 # CLI commands
│   ├── root.go         # Root command with global flags
│   ├── upload.go       # Upload command
│   └── version.go      # Version command
├── internal/           # Internal packages
│   ├── uploader/       # Core upload logic with provider interfaces
│   ├── providers/      # Provider system (types, base provider, consistency wrapper)
│   ├── config/         # Configuration management
│   ├── logging/        # Professional logging system with logrus
│   └── output/         # Output handlers
├── pkg/               # Public packages
│   └── providers/     # File hosting provider implementations (BuzzHeavier, factory)
├── main.go            # Application entry point
└── go.mod             # Go module definition
```

## Commands

### Upload

Upload files and directories to hosting providers with new flag-based interface:

```bash
# Basic usage - requires explicit file/folder flags
woof upload -f file.txt --all
woof upload -f "*.pdf" -d ./documents
woof upload --providers buzzheavier -d ./backups
```

### Version

Display version information:
```bash
woof version
```

## Development

### Building

```bash
go build -o woof .
```

### Running Tests

```bash
go test ./...
```

## Architecture

The project follows Go best practices with:

- **CLI-First Design**: No configuration required - all features accessible via flags
- **Modular Design**: Clear separation of concerns between uploader, configuration, logging, and output handling
- **Professional Logging**: Structured logging with sirupsen/logrus featuring colors, timestamps, and categorical organization
- **Concurrent Processing**: Semaphore-controlled goroutine pools for parallel uploads
- **Interface-Based Design**: Easy extensibility for new file hosting providers
- **Provider Consistency System**: Structured responses, automatic validation, retry logic, and standardized error handling
- **Rich Provider Responses**: Upload responses include metadata, URLs, deletion links, expiration info, and provider-specific data
- **Typed Error System**: Categorized errors with retryability information and structured context
- **Strict Validation**: Path validation, file size limits, and provider capability checking
- **Glob Pattern Support**: Built-in wildcard pattern matching for files
- **Error Handling**: Structured error handling with proper error propagation and log context
- **Progress Tracking**: Real-time progress reporting via channels

## License

MIT License