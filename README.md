# Woof

High-performance parallel file uploader CLI in Go with Cobra.

## Features

- High-performance concurrent uploads using Go goroutines
- Support for files and directories
- Multiple file hosting providers
- Real-time progress tracking
- Multiple output formats (text, JSON)
- Configurable via YAML files or environment variables
- Retry mechanism with backoff
- Piping support for integration with other tools

## Installation

```bash
go install github.com/parnexcodes/woof@latest
```

## Usage

### Basic Upload

```bash
# Upload a single file
woof upload file.txt

# Upload multiple files
woof upload file1.txt file2.txt file3.txt

# Upload directory
woof upload ./uploads/

# Upload with custom concurrency
woof upload -c 10 ./large_files/

# Upload with JSON output
woof upload -o json ./files/
```

### Configuration

Create a `.woof.yaml` file in your current directory or home directory:

```yaml
concurrency: 5
verbose: false
output: "text"

providers:
  - name: "buzzheavier"
    enabled: true
    settings:
      upload_url: "https://w.buzzheavier.com"
      download_base_url: "https://buzzheavier.com"
      timeout: "10m"

upload:
  retry_attempts: 3
  retry_delay: "2s"
  chunk_size: 1048576  # 1MB
  timeout: "30m"
```

### Available Providers

- **BuzzHeavier**: File hosting service with PUT-based uploads
  - Enabled by setting `enabled: true` in configuration
  - Use with `-p buzzheavier` flag or enable all providers

## Project Structure

```
woof/
├── cmd/                 # CLI commands
│   ├── root.go         # Root command with global flags
│   ├── upload.go       # Upload command
│   └── version.go      # Version command
├── internal/           # Internal packages
│   ├── uploader/       # Core upload logic
│   ├── config/         # Configuration management
│   └── output/         # Output handlers
├── pkg/               # Public packages
│   └── providers/     # File hosting providers
├── main.go            # Application entry point
└── go.mod             # Go module definition
```

## Commands

### Upload

Upload files and directories to hosting providers:
```bash
woof upload [files/directories...] [flags]
```

Options:
- `-c, --concurrency int`: Maximum number of parallel uploads (default: 5)
- `-o, --output string`: Output format (text, json) (default: text)
- `-p, --providers strings`: Specific providers to use
- `--retry-attempts int`: Number of retry attempts per file (default: 3)
- `--retry-delay duration`: Delay between retry attempts (default: 2s)
- `--progress`: Show upload progress (default: true)

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

- **Modular Design**: Clear separation of concerns between uploader, configuration, and output handling
- **Concurrent Processing**: Semaphore-controlled goroutine pools for parallel uploads
- **Interface-Based Design**: Easy extensibility for new file hosting providers
- **Error Handling**: Structured error handling with proper error propagation
- **Progress Tracking**: Real-time progress reporting via channels

## License

MIT License