# CYGoLogger

A high-performance, async, multi-level logging library for Go, inspired by the [CYLogger](https://github.com/maxhaosl/CYLogger) C++ library. Supports console, file (with rotation), remote TCP, and Unix syslog outputs.

## Features

- **Multiple log types**: TRACE, DEBUG, INFO, WARN, ERROR, FATAL, MAIN, REMOTE, SYS
- **Multiple outputs**: Console (ANSI color), File (with rotation), Remote TCP, Unix syslog
- **Async logging**: Non-blocking writes via buffered channels, with automatic flush on exit
- **Log level filtering**: Bitmask-based level control at runtime
- **Pattern layout**: Three built-in formats, plus full custom layout support
- **Escape filtering**: Automatic escaping of special characters in log messages
- **File rotation**: Time-based and size-based rotation with configurable limits
- **Expired file cleanup**: Automatic deletion of old log files
- **Statistics**: Per-type line/byte counters and FPS tracking
- **Thread-safe**: Safe for concurrent use from multiple goroutines
- **Cross-platform**: Linux, macOS, Windows

## Requirements

- Go 1.21 or later

## Installation

```bash
go get github.com/maxhaosl/CYGoLogger
```

## Quick Start

```go
package main

import (
    "fmt"
    gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"
)

func main() {
    // Initialize: log path, show console
    ok := gologger.InitLogger("./Log", true)
    if !ok {
        fmt.Println("Failed to initialize logger")
        return
    }
    defer gologger.UnInitLogger()

    // Add file appenders
    gologger.GetInstance().AddAppender(
        gologger.LogTypeInfo, "app", "Info.log", gologger.LogFileModeAppend)
    gologger.GetInstance().AddAppender(
        gologger.LogTypeError, "app", "Error.log", gologger.LogFileModeAppend)

    // Log messages
    gologger.LOG_INFO("Application started")
    gologger.LOG_DEBUG("Debug info: version=%s", "1.0.0")
    gologger.LOG_WARN("This is a warning: id=%d", 42)
    gologger.LOG_ERROR("Error occurred: code=%d", 500)
    gologger.LOG_FATAL("Fatal error: aborting")

    // Flush and cleanup
    gologger.FlushLogger()
}
```

## Log Types

| Type | Value | Description |
|------|-------|-------------|
| `LogTypeNone` | 0 | No output |
| `LogTypeTrace` | 1 | Trace level |
| `LogTypeDebug` | 2 | Debug level |
| `LogTypeInfo` | 3 | Info level |
| `LogTypeWarn` | 4 | Warning level |
| `LogTypeError` | 5 | Error level |
| `LogTypeFatal` | 6 | Fatal level |
| `LogTypeMain` | 7 | Catch-all (all levels) |
| `LogTypeRemote` | 8 | Remote log (TCP) |
| `LogTypeSys` | 9 | System log (syslog) |
| `LogTypeConsole` | 10 | Console output |

## Log Level Filters

```go
// All levels
gologger.GetInstance().SetLogLevel(gologger.LogFilterAll)

// Info and above (INFO, WARN, ERROR, FATAL)
gologger.GetInstance().SetLogLevel(gologger.LogFilterWarnsAndErrors)

// Only ERROR and FATAL
gologger.GetInstance().SetLogLevel(gologger.LogFilterErrors)

// Disable all logging
gologger.GetInstance().SetLogLevel(gologger.LogFilterNone)
```

## Log Macros

```go
gologger.LOG_TRACE("Trace message: %s", "details")
gologger.LOG_DEBUG("Debug message: count=%d", 10)
gologger.LOG_INFO("Info message: version=%s", "1.0")
gologger.LOG_WARN("Warn message: value=%f", 3.14)
gologger.LOG_ERROR("Error message: code=%d", 404)
gologger.LOG_FATAL("Fatal message: reason=%s", "oom")
gologger.LOG_MAIN("Main message")
gologger.LOG_SYS("Sys message")
gologger.LOG_REMOTE("Remote message")
```

## Structured Logging

```go
// With file, function, and line info
gologger.GetInstance().WriteLogFmt(
    int(gologger.LogLevelInfo),
    gologger.LogTypeInfo,
    100, // server code
    "main.go", "main", 42,
    "Structured message: key=%s, value=%d", "foo", 123)

// Escape special characters
gologger.GetInstance().WriteEscapeLogFmt(
    int(gologger.LogLevelInfo),
    gologger.LogTypeInfo,
    -1,
    "main.go", "main", 50,
    "Message with [brackets] and ]more[")

// Hex dump
hexData := []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f}
gologger.GetInstance().WriteHexLog(
    int(gologger.LogLevelDebug),
    gologger.LogTypeDebug,
    -1,
    "main.go", "main", 60,
    hexData)

// Plain message without formatting
gologger.GetInstance().WriteLog(
    int(gologger.LogLevelInfo),
    gologger.LogTypeInfo,
    -1,
    "Direct plain text message")
```

## Layout Types

| Layout | Format |
|--------|--------|
| `LogLayoutTypeBuildin1` | `[HH:MM:SS.mmm][TYPE][PID][TID] Msg` |
| `LogLayoutTypeBuildin2` | `[YYYY-MM-DD HH:MM:SS.mmm][TYPE][PID][TID][file:line][func] Msg` |
| `LogLayoutTypeBuildin3` | `[HH:MM:SS][TYPE][channel] Msg` |
| `LogLayoutTypeCustom` | User-provided layout |

Switch layout at runtime:

```go
gologger.GetInstance().SetLayout(gologger.LogLayoutTypeBuildin2, nil)

// Custom layout
customLayout := gologger.NewCYLoggerTemplateLayoutCustom(
    gologger.GetCYLoggerTemplateLayoutManagerInstance().GetLayout(gologger.LogLayoutTypeBuildin3))
gologger.GetInstance().SetLayout(gologger.LogLayoutTypeCustom, customLayout)
```

## File Rotation

```go
// Time-based naming (default): app_20240101_120000.log
gologger.GetInstance().AddAppender(
    gologger.LogTypeInfo, "app", "Info.log", gologger.LogFileModeTime)

// Append to single file
gologger.GetInstance().AddAppender(
    gologger.LogTypeInfo, "app", "Info.log", gologger.LogFileModeAppend)
```

## Statistics

```go
var stats gologger.STStatistics
gologger.GetInstance().GetStats(&stats)

fmt.Printf("Total Lines: %d\n", stats.NTotalLine)
fmt.Printf("Total Bytes: %d\n", stats.NTotalByte)
fmt.Printf("Info Lines:  %d\n", stats.NInfoLine)
```

## Concurrent Logging

CYGoLogger is safe for concurrent use. Multiple goroutines can call logging functions simultaneously.

```go
for i := 0; i < 5; i++ {
    go func(id int) {
        for j := 0; j < 100; j++ {
            gologger.LOG_INFO("Goroutine %d: message %d", id, j)
        }
    }(i)
}
```

## Package Structure

```
ICYLogger/
├── inc.go                  # Single import entry point
├── Core/types.go           # Enums and constants
├── Common/common.go        # Message types, FPS counter, utilities
├── Filter/filter.go        # Pattern filter chain
├── Layout/
│   ├── interfaces.go       # ICYLogger, ICYLoggerTemplateLayout interfaces
│   └── layout.go           # Built-in and custom layouts
├── Config/config.go        # Global configuration singleton
├── Statistics/statistics.go # Atomic line/byte/FPS counters
├── Entity/entity.go        # Appender container and factory
├── Appender/appender.go    # 6 appender types
├── Logger/logger.go       # CYLoggerControl and CYLoggerImpl
└── Schedule/
    └── CYLoggerSchedule.go # Background expired-file cleanup
```

## License

CYGoLogger is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Authors

- ShiLiang.Hao &lt;newhaosl@163.com&gt;
- foobra &lt;vipgs99@gmail.com&gt;

## References

- [CYLogger (C++)](https://github.com/maxhaosl/CYLogger) - The original C++ logging library
