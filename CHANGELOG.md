# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2024-01-01

### Added

- **Core type system**: Full set of log type enums (`ELogType`, `ELogLevel`, `ELogLevelFilter`, `ELogFileMode`, `ELogLayoutType`)
- **Log macros**: `LOG_TRACE`, `LOG_DEBUG`, `LOG_INFO`, `LOG_WARN`, `LOG_ERROR`, `LOG_FATAL`, `LOG_MAIN`, `LOG_SYS`, `LOG_REMOTE`
- **Async appenders**:
  - `CYLoggerConsoleAppender`: Console output with ANSI color support
  - `CYLoggerFileAppender`: File output with time-based and size-based rotation
  - `CYLoggerMainAppender`: Multi-level file appender (alias of FileAppender for `LogTypeMain`)
  - `CYLoggerBufferAppender`: Buffered appender with per-type queues
  - `CYLoggerRemoteAppender`: TCP remote logging with reconnection support
  - `CYLoggerSystemAppender`: Unix syslog output
- **Appender factory**: `CYLoggerAppenderFactory` with registration and management
- **Pattern layout**: Three built-in formats (`Layout1`, `Layout2`, `Layout3`) and custom layout support
- **Escape filter**: `ICYLoggerPatternFilter` chain for escaping special characters in messages
- **File restriction**: `CYFileRestriction` for controlling max file size, count, and total size
- **Automatic file cleanup**: `CYLoggerSchedule` with expired file deletion (60s interval)
- **Statistics**: `CYStatistics` with atomic per-type line/byte counters and FPS tracking
- **Double-buffered queues**: `CYLoggerBaseAppender` with `PublicQueue` and `PrivateQueue` channel swap
- **Configuration**: `CYLoggerConfig` singleton for global log path, console, layout, and level settings
- **Global singleton**: `GetInstance()`, `InitLogger()`, `UnInitLogger()`, `FreeInstance()`
- **Flush synchronization**: `sync.Cond` based flush coordination between writer and caller goroutines
- **Message pooling**: `CYBaseMessage` with `AcquireBaseMessage()` / `ReleaseBaseMessage()`
- **Hex logging**: `WriteHexLog` for binary data output (16 bytes/line with ASCII column)
- **Escape logging**: `WriteEscapeLogFmt` for messages containing special characters
- **Signal handling**: Console test example demonstrates graceful shutdown on `SIGINT`/`SIGTERM`
- **Cross-platform utilities**: `CYPublicFunction` with file ops, env vars, OS detection, goroutine ID
- **FPS counter**: `CYFPSCounter` with configurable update interval and current/average FPS
- **Examples**: `simple_example`, `simple_test`, `console_test`, `simple_logger_test`

### Changed

- Module path migrated from `github.com/cylogger/gologger` to `github.com/maxhaosl/CYGoLogger`

### Fixed

- N/A (initial release)

### Known Limitations

- `CYLoggerSystemAppender.doWrite` is not wired in `Run()` — syslog output is a stub
- `CYLoggerRemoteAppender` uses TCP instead of the C++ UDP protocol
- `CYLoggerMainAppender` does not inherit the 6-level per-type buffer queue like its C++ counterpart
- File cleanup only supports age-based expiration (not count, type-size, or total-size limits)
- `CY_SCOPE()` RAII scope logging macro has no Go equivalent
