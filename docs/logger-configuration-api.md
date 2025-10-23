# Logger Configuration Architecture

## Overview

The `logger` package provides a production-ready logging system built on [zerolog](https://github.com/rs/zerolog) with automatic log rotation, flexible configuration, and environment-based overrides. It supports both console-based logging (for containerized services) and file-based logging (for traditional applications) with intelligent defaults and validation.

**Key Features:**
- Type-safe configuration with validation
- Environment variable overrides for 12-factor app compliance
- Automatic log rotation via [lumberjack](https://github.com/natefinch/lumberjack)
- Separate files for info and error logs
- Zero-allocation structured JSON logging
- Production-ready with fallback mechanisms

---

## Quick Start

**IMPORTANT:** All logger creation methods automatically respect environment variables (LOG_MODE, LOG_LEVEL, LOG_FORMAT, etc.). Environment variables always override code defaults, following the 12-factor app pattern.

### Console Logging (Default)

```go
import "github.com/weka/go-weka-observability/logger"

func main() {
    // Creates default logger: console sink, JSON format, info level
    // Override via env: LOG_LEVEL=0 LOG_FORMAT=raw
    logr := logger.CreateLogger()
    logr.Info("Application started")
}
```

**Output to stderr:**
```json
{"level":"info","time":"2025-09-30T12:00:00Z","message":"Application started"}
```

### File Logging with Functional Options

```go
// Functional options set defaults, but LOG_* env vars can override
logr := logger.CreateLogger(
    logger.WithFileSink("/var/log/myapp", "myapp.log"),
    logger.WithRotation(100, 5, 28), // 100MB, 5 backups, 28 days
    logger.WithInfoLevel(),
)
// Override via env: LOG_MODE=console to switch to stderr
logr.Info("Processing request")
```

**Files created:**
```
/var/log/myapp/
├── myapp.log       # Info, debug, trace logs
└── myapp-error.log # Warn, error, fatal logs
```

### With Context (Recommended for Applications)

```go
func main() {
    ctx := context.Background()

    // Create logger with options (overrideable via LOG_* env vars)
    logr := logger.CreateLogger(
        logger.WithConsoleSink(),
        logger.WithInfoLevel(),
    )

    // Store in context for propagation
    ctx = logger.ContextWithLogr(ctx, logr)

    // Use throughout application
    processRequest(ctx)
}

func processRequest(ctx context.Context) {
    logger := logger.MustLogrFromContext(ctx)
    logger.Info("Processing started", "request_id", "req-123")
}
```

---

## Architecture

### Design Principles

1. **Explicit over Implicit** - No magic defaults, all configuration is visible
2. **Environment-Driven** - Support 12-factor app configuration
3. **Fail-Safe** - Validation with warnings, never crashes
4. **Zero-Allocation** - Built on zerolog for high performance
5. **Production-Ready** - Automatic rotation, compression, retention

### Component Diagram

```
Application
    ↓
Config (Configuration)
    ↓
NewZeroLoggerWithConfig() (Factory)
    ↓
Validation Layer (slog warnings + fallbacks)
    ↓
Multi-Level Writer (info vs error separation)
    ↓
    ├─→ Console Writer (stderr)
    └─→ File Writer (lumberjack rotation)
```

### Log Flow

```
Application Code
    ↓
log.Info().Msg("message")
    ↓
zerolog.Logger (level filtering)
    ↓
Multi-Level Writer (route by level)
    ↓
    ├─→ Info Writer → myapp.log
    └─→ Error Writer → myapp-error.log
```

---

## Core Concepts

### OutputMode

Determines where logs are written:

```go
type OutputMode string

const (
    ConsoleMode OutputMode = "console"  // Logs to stderr
    FileMode    OutputMode = "file"     // Logs to rotating files
)
```

**When to use:**
- **ConsoleMode**: Docker containers, Kubernetes pods, systemd services, local development
- **FileMode**: Traditional daemons, CLI tools, legacy applications

### Config

Complete configuration for logger behavior:

```go
type Config struct {
    Sink   SinkConfig    // Destination configuration
    Format FormatConfig  // Presentation configuration
}

type SinkConfig struct {
    Mode       OutputMode  // Where logs go
    Dir        string      // Directory for files (FileMode)
    FileName   string      // Base filename (FileMode)
    MaxSizeMB  int         // MB before rotation
    MaxFiles   int         // Number of backups
    MaxAgeDays int         // Days to retain
}

type FormatConfig struct {
    Level        zerolog.Level  // Minimum log level
    Format       LogFormat      // Output format
    TimeOnly     bool           // Time-only timestamps
    CallerDirLvl int            // Caller directory depth
}
```

**Default values** (from `DefaultConfig()`):
- **Sink defaults:**
  - `Mode`: `ConsoleMode` (cloud-native default)
  - `Dir`: `/var/log`
  - `FileName`: `""` (empty - must set for FileMode)
  - `MaxSizeMB`: `100` MB
  - `MaxFiles`: `5` backups
  - `MaxAgeDays`: `28` days
- **Format defaults:**
  - `Level`: `zerolog.InfoLevel`
  - `Format`: `LogFormatJSON`
  - `TimeOnly`: `false`
  - `CallerDirLvl`: `-1` (disabled)

### Multi-Level Writer

Logs are automatically separated by severity:

**Info File** (`myapp.log`):
- Trace
- Debug
- Info

**Error File** (`myapp-error.log`):
- Warn
- Error
- Fatal
- Panic

**Rationale**: Faster incident response - error logs are isolated from debug noise.

---

## Configuration Patterns

### Pattern 1: Use Defaults (Simplest)

**Use Case:** Simple applications, quick start, local development

```go
// Creates logger with defaults: console, JSON, info level
// All overrideable via LOG_* environment variables
logr := logger.CreateLogger()
logr.Info("Application started")
```

### Pattern 2: Functional Options (Recommended)

**Use Case:** Set application defaults while allowing environment overrides

```go
// Set explicit defaults, but allow env vars to override
logr := logger.CreateLogger(
    logger.WithConsoleSink(),      // Override: LOG_MODE=file
    logger.WithInfoLevel(),         // Override: LOG_LEVEL=0
    logger.WithJSONFormat(),        // Override: LOG_FORMAT=raw
)
```

### Pattern 3: Explicit Configuration

**Use Case:** Full control, complex configuration, programmatic setup

```go
config := logger.Config{
    Sink: logger.SinkConfig{
        Mode:       logger.FileMode,
        Dir:        "/var/log/myapp",
        FileName:   "service.log",
        MaxSizeMB:  50,
        MaxFiles:   10,
        MaxAgeDays: 7,
    },
    Format: logger.FormatConfig{
        Level:  zerolog.DebugLevel,
        Format: logger.LogFormatJSON,
    },
}

// Environment variables can still override this config
logr := logger.CreateLoggerFrom(config)
```

### Pattern 4: Context-Based (Recommended for Applications)

**Use Case:** Microservices, request handling, span propagation

```go
func main() {
    ctx := context.Background()

    // Create logger (overrideable via LOG_* env vars)
    logr := logger.CreateLogger(
        logger.WithConsoleSink(),
        logger.WithInfoLevel(),
    )

    // Store in context
    ctx = logger.ContextWithLogr(ctx, logr)

    // Pass context through call chain
    http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
        handleRequest(r.Context())
    })
}

func handleRequest(ctx context.Context) {
    // Retrieve logger from context
    logger := logger.MustLogrFromContext(ctx)
    logger.Info("Handling request")
}
```

**Deployment Examples:**

```bash
# Production: use code defaults
./myapp

# Staging: enable debug logging
LOG_LEVEL=0 ./myapp

# Development: colored output
LOG_FORMAT=raw LOG_LEVEL=0 ./myapp

# Development: different directory
LOG_DIR=/tmp/dev-logs ./myapp
```

**Key Feature:** Only environment variables that are **actually set** override defaults. All other fields preserve your custom values.

### Pattern 4: Environment-Only (12-Factor App)

**Use Case:** Cloud-native applications, containers

```go
config := logger.NewDefaultConfigWithEnvOverride()
log := logger.NewZeroLoggerWithConfig(config)
```

**Kubernetes ConfigMap:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-logging
data:
  LOG_MODE: "file"
  LOG_DIR: "/var/log/app"
  LOG_FILE_NAME: "production.log"
  LOG_MAX_SIZE_MB: "100"
  LOG_MAX_FILES: "10"
  LOG_MAX_AGE_DAYS: "30"
```

---

## Environment Variables

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `LOG_MODE` | Output mode | `console` | `file` |
| `LOG_DIR` | Log directory | `/var/log` | `/app/logs` |
| `LOG_FILE_NAME` | Log filename | `""` | `service.log` |
| `LOG_MAX_SIZE_MB` | Max file size (MB) | `100` | `50` |
| `LOG_MAX_FILES` | Max backups | `5` | `10` |
| `LOG_MAX_AGE_DAYS` | Retention (days) | `28` | `7` |
| `LOG_LEVEL` | Log level (0-5) | `1` (info) | `0` (trace) |
| `LOG_FORMAT` | Format (`raw`/`json`/`plain`) | `json` | `raw` |
| `LOG_TIME_ONLY` | Use time-only format | `false` | `true` |

**Log Levels:**
- `-1` = Trace (everything)
- `0` = Debug
- `1` = Info (default)
- `2` = Warn
- `3` = Error
- `4` = Fatal

---

## API Reference

### Configuration Functions

#### `DefaultConfig() Config`

Returns default configuration for console logging.

```go
config := logger.DefaultConfig()
// OutputMode: ConsoleMode
// LogFileName: "" (empty)
```

#### `NewConfigFromEnv(defaultConfig Config) Config`

Merges custom defaults with environment overrides. **Only set environment variables override defaults.**

```go
custom := logger.Config{
    Sink: logger.SinkConfig{
        Mode:       logger.FileMode,
        Dir:        "/my/logs",
        FileName:   "app.log",
        MaxSizeMB:  200,
    },
}

os.Setenv("LOG_MAX_SIZE_MB", "50")

config := logger.NewConfigFromEnv(custom)
// Result:
// - MaxLogSize: 50 (from env)
// - LogDir: "/my/logs" (from custom)
// - LogFileName: "app.log" (from custom)
```

#### `NewDefaultConfigWithEnvOverride() Config`

Convenience function for: `NewConfigFromEnv(DefaultConfig())`

```go
config := logger.NewDefaultConfigWithEnvOverride()
```

### Logger Creation

#### `NewZeroLogger() *zerolog.Logger`

Creates logger with environment configuration.

```go
log := logger.NewZeroLogger()
log.Info().Msg("Ready")
```

#### `NewZeroLoggerWithConfig(config Config) *zerolog.Logger`

Creates logger with explicit configuration.

```go
config := logger.Config{
    Sink: logger.SinkConfig{
        Mode:       logger.FileMode,
        Dir:        "/var/log",
        FileName:   "app.log",
        MaxSizeMB:  100,
        MaxFiles:   5,
        MaxAgeDays: 28,
    },
}

log := logger.NewZeroLoggerWithConfig(config)
```

#### `NewNamedLogger(serviceName string) *Logger`

Creates logger with service name field.

```go
log := logger.NewNamedLogger("http-server")
log.Info().Msg("Started")
// {"service":"http-server","level":"info",...}
```

---

## Validation & Safety

### Automatic Fallbacks

The logger **never crashes** due to misconfiguration. Instead, it emits warnings via `slog` and uses safe fallbacks.

#### Missing LogFileName (FileMode)

**Configuration:**
```go
config := logger.DefaultConfig()
config.OutputMode = logger.FileMode
// Forgot to set LogFileName!
```

**Behavior:**
```
2025/09/30 12:00:00 WARN FileMode requires LogFileName, using fallback
    fallback=app.log
    suggestion="set LogFileName explicitly in your config"
```

**Result:** Logs written to `/var/log/app.log`

#### Missing LogDir (FileMode)

**Configuration:**
```go
config := logger.Config{
    Sink: logger.SinkConfig{
        Mode:     logger.FileMode,
        FileName: "test.log",
        // Dir is empty
    },
}
```

**Behavior:**
```
2025/09/30 12:00:00 WARN FileMode requires LogDir, using fallback
    fallback=/tmp
    suggestion="set LogDir explicitly in your config"
```

**Result:** Logs written to `/tmp/test.log`

#### Correct Configuration (No Warnings)

```go
config := logger.Config{
    Sink: logger.SinkConfig{
        Mode:       logger.FileMode,
        Dir:        "/var/log/myapp",
        FileName:   "myapp.log",
        MaxSizeMB:  100,
        MaxFiles:   5,
        MaxAgeDays: 28,
    },
}

log := logger.NewZeroLoggerWithConfig(config)
// No warnings - configuration is complete
```

---

## Log Rotation

### How It Works

Powered by [lumberjack](https://github.com/natefinch/lumberjack):

1. **Size-Based Rotation**: When `myapp.log` reaches `MaxLogSize` MB, it's renamed to `myapp-2025-09-30T12-00-00.000.log`
2. **Compression**: Old logs are gzipped: `myapp-2025-09-30T12-00-00.000.gz`
3. **Backup Management**: Keep only `MaxLogFiles` backups
4. **Age-Based Cleanup**: Delete logs older than `MaxAge` days

### File Naming Convention

**Active logs:**
```
myapp.log        # Current info logs
myapp-error.log  # Current error logs
```

**Rotated logs:**
```
myapp-2025-09-30T12-00-00.000.gz       # Yesterday's info
myapp-error-2025-09-30T12-00-00.000.gz # Yesterday's errors
myapp-2025-09-29T12-00-00.000.gz       # 2 days ago
```

**Example timeline:**
```
/var/log/myapp/
├── myapp.log (80MB)              # Active, still growing
├── myapp-error.log (5MB)         # Active
├── myapp-2025-09-30.gz (100MB)   # Rotated today
├── myapp-2025-09-29.gz (100MB)   # Yesterday
├── myapp-2025-09-28.gz (100MB)   # 2 days ago
├── myapp-2025-09-27.gz (100MB)   # 3 days ago
└── myapp-2025-09-26.gz (100MB)   # 4 days ago (oldest backup)
```

### Configuration Examples

**High-frequency application:**
```go
config.Sink.MaxSizeMB = 50   // Rotate more often
config.Sink.MaxFiles = 20    // Keep more history
config.Sink.MaxAgeDays = 7   // Delete after 1 week
```

**Low-frequency application:**
```go
config.Sink.MaxSizeMB = 500  // Larger files
config.Sink.MaxFiles = 3     // Fewer backups
config.Sink.MaxAgeDays = 90  // Keep for 3 months
```

---

## Use Cases

### Use Case 1: Microservice in Kubernetes

**Requirements:**
- Logs to stderr (collected by Kubernetes)
- JSON format for parsing
- Environment-driven configuration

**Implementation:**
```go
log := logger.NewZeroLogger()
log.Info().
    Str("request_id", requestID).
    Str("endpoint", "/api/users").
    Int("status_code", 200).
    Msg("Request completed")
```

**Kubernetes logs:**
```bash
kubectl logs pod-name
{"level":"info","request_id":"abc123","endpoint":"/api/users",...}
```

### Use Case 2: Traditional Daemon

**Requirements:**
- Logs to files in /var/log
- Automatic rotation
- Separate error logs for monitoring

**Implementation:**
```go
config := logger.Config{
    Sink: logger.SinkConfig{
        Mode:       logger.FileMode,
        Dir:        "/var/log/mydaemon",
        FileName:   "daemon.log",
        MaxSizeMB:  100,
        MaxFiles:   10,
        MaxAgeDays: 30,
    },
}

log := logger.NewZeroLoggerWithConfig(config)
```

**Monitoring:**
```bash
tail -f /var/log/mydaemon/daemon-error.log  # Watch errors only
```

### Use Case 3: CLI Tool

**Requirements:**
- Logs to files (keep console clean)
- Small log files
- Short retention

**Implementation:**
```go
config := logger.Config{
    Sink: logger.SinkConfig{
        Mode:       logger.FileMode,
        Dir:        os.TempDir(),
        FileName:   "cli-tool.log",
        MaxSizeMB:  10,  // 10MB
        MaxFiles:   2,
        MaxAgeDays: 1,   // 1 day
    },
}

log := logger.NewZeroLoggerWithConfig(config)
```

### Use Case 4: Multi-Environment Application

**Requirements:**
- Different config per environment
- Override via environment variables
- Sane defaults for development

**Implementation:**
```go
appDefaults := logger.Config{
    Sink: logger.SinkConfig{
        Mode:       logger.FileMode,
        Dir:        "/app/logs",
        FileName:   "app.log",
        MaxSizeMB:  100,
        MaxFiles:   5,
        MaxAgeDays: 14,
    },
}

config := logger.NewConfigFromEnv(appDefaults)
log := logger.NewZeroLoggerWithConfig(config)
```

**Environments:**

```bash
# Development
LOG_DIR=/tmp/dev-logs LOG_LEVEL=0 ./app

# Staging
LOG_MAX_SIZE_MB=50 LOG_MAX_AGE_DAYS=7 ./app

# Production
# Uses appDefaults as-is
./app
```

### Use Case 5: Per-Module Loggers

**Requirements:**
- Separate log files per module
- Shared configuration (size, rotation)

**Implementation:**
```go
baseConfig := logger.Config{
    Sink: logger.SinkConfig{
        Mode:       logger.FileMode,
        Dir:        "/var/log/myapp",
        MaxSizeMB:  100,
        MaxFiles:   5,
        MaxAgeDays: 28,
    },
}

// HTTP module
httpConfig := baseConfig
httpConfig.Sink.FileName = "http.log"
httpLog := logger.NewZeroLoggerWithConfig(httpConfig)

// Database module
dbConfig := baseConfig
dbConfig.Sink.FileName = "database.log"
dbLog := logger.NewZeroLoggerWithConfig(dbConfig)

// Queue module
queueConfig := baseConfig
queueConfig.LogFileName = "queue.log"
queueLog := logger.NewZeroLoggerWithConfig(queueConfig)
```

**Result:**
```
/var/log/myapp/
├── http.log
├── http-error.log
├── database.log
├── database-error.log
├── queue.log
└── queue-error.log
```

---

## Performance

### Zero-Allocation Logging

Built on [zerolog](https://github.com/rs/zerolog), which uses zero-allocation JSON encoding:

```go
log.Info().
    Str("key", "value").   // No allocation
    Int("count", 42).      // No allocation
    Msg("event")           // No allocation
```

**Performance:**
- Zero allocations per log call (when properly used)
- Significantly faster than reflection-based loggers (logrus, stdlib)
- For detailed benchmarks, see [Go Logging Benchmarks](https://betterstack-community.github.io/go-logging-benchmarks/)

### Buffered Writes

Lumberjack buffers writes for efficiency:
- Reduces syscalls
- Improves throughput
- Minimal latency impact

### Level-Based Routing

Separate writers avoid filtering overhead:
- Info logs → info writer (no level check)
- Error logs → error writer (no level check)

---

## Testing

### Testing with Temporary Directories

```go
func TestLogging(t *testing.T) {
    config := logger.Config{
        Sink: logger.SinkConfig{
            Mode:       logger.FileMode,
            Dir:        t.TempDir(), // Auto-cleanup
            FileName:   "test.log",
            MaxSizeMB:  10,
            MaxFiles:   2,
            MaxAgeDays: 1,
        },
    }

    log := logger.NewZeroLoggerWithConfig(config)
    log.Info().Msg("test message")

    // Verify log file
    logPath := filepath.Join(config.Sink.Dir, "test.log")
    content, err := os.ReadFile(logPath)
    require.NoError(t, err)
    assert.Contains(t, string(content), "test message")
}
```

### Testing with In-Memory Buffer

```go
func TestLogContent(t *testing.T) {
    var buf bytes.Buffer

    log := zerolog.New(&buf).With().Timestamp().Logger()
    log.Info().Str("key", "value").Msg("test")

    assert.Contains(t, buf.String(), `"key":"value"`)
    assert.Contains(t, buf.String(), `"message":"test"`)
}
```

---

## Best Practices

### 1. Always Set FileName for FileMode

```go
// ✅ Good
config := logger.Config{
    Sink: logger.SinkConfig{
        Mode:     logger.FileMode,
        FileName: "myapp.log", // Explicit
        Dir:      "/var/log",
    },
}

// ❌ Bad - relies on fallback warning
config := logger.Config{
    Sink: logger.SinkConfig{
        Mode: logger.FileMode,
        Dir:  "/var/log",
        // Missing FileName
    },
}
```

### 2. Use Environment Variables for Deployment Configuration

```go
// ✅ Good - configurable per environment
appDefaults := logger.Config{...}
config := logger.NewConfigFromEnv(appDefaults)

// ❌ Bad - hardcoded
config := logger.Config{
    LogDir: "/var/log", // Fixed, can't override
}
```

### 3. Use ConsoleMode for Containers

```go
// ✅ Good for Docker/Kubernetes
log := logger.NewZeroLogger() // Default ConsoleMode

// ❌ Bad for containers
config.OutputMode = logger.FileMode // Files in container
```

### 4. Separate Logs by Module/Service

```go
// ✅ Good - easy to debug
httpLog := logger.NewZeroLoggerWithConfig(httpConfig)
dbLog := logger.NewZeroLoggerWithConfig(dbConfig)

// ❌ Bad - mixed concerns
log := logger.NewZeroLogger() // Everything in one file
```

### 5. Configure Retention Based on Disk Space

```go
// ✅ Good - calculate based on disk
// Disk: 100GB, allow 10GB for logs
// Each file: 100MB, keep 100 files
config.Sink.MaxSizeMB = 100
config.Sink.MaxFiles = 100

// ❌ Bad - unlimited growth
config.Sink.MaxFiles = 999
config.Sink.MaxAgeDays = 999
```

---

## Troubleshooting

### Logs Not Appearing

**Symptoms:** No log files created

**Check:**
1. Is `LogFileName` set for FileMode?
2. Does `LogDir` exist and is it writable?
3. Is log level filtering messages? (Set `LOG_LEVEL=-1` to see everything)
4. Check stderr for validation warnings

### Permission Errors

**Error:**
```
zerolog: could not write event: can't open new logfile: open /var/log/app.log: permission denied
```

**Solutions:**
```go
// Option 1: Use writable directory
config.Sink.Dir = "/tmp"

// Option 2: Run with permissions
sudo ./app

// Option 3: Create directory first
mkdir -p /var/log/myapp
chmod 755 /var/log/myapp
```

### Disk Space Exhaustion

**Symptoms:** Disk full, logs growing indefinitely

**Solutions:**
```go
// Reduce log size
config.Sink.MaxSizeMB = 50  // 50MB instead of 100MB

// Reduce retention
config.Sink.MaxFiles = 5    // 5 backups instead of 10
config.Sink.MaxAgeDays = 7  // 7 days instead of 28
```

### Missing Error Logs

**Symptoms:** Info logs work, error logs missing

**Check:**
1. Log level - are you logging below threshold?
2. File permissions on `{filename}-error.log`
3. Disk space - error file might be full

---

## Summary

The logger configuration architecture provides:

✅ **Flexible Configuration** - Console or file-based logging
✅ **Environment Overrides** - 12-factor app compliance
✅ **Production-Ready** - Automatic rotation, compression, retention
✅ **Type-Safe** - `OutputMode` enum prevents errors
✅ **Fail-Safe** - Validation with warnings, never crashes
✅ **High Performance** - Zero-allocation structured logging
✅ **Multi-Level** - Separate info and error logs

For detailed examples, see the test suite in `logger/logger_test.go`.