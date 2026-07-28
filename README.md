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
- **One-line init**: `InitDefault(path)` + top-level `Trace/Debug/.../Scope()` functions with automatic `file:line` capture (`runtime.Caller`)
- **Hex & Escape logging**: Top-level `HexInfo`, `EscapeInfo`, etc. for binary dumps and safe escaping
- **Crash logging**: `InitException` + `defer Recover()` / `SafeGo` captures panics with full stack traces to a dedicated file
- **Lightweight synchronous log**: `SimpleLog` for file/console output that bypasses the async channel (ideal for crash logs)
- **FTP upload**: `UpLoadConfig` + `UploadLogFTP` to archive and push log files to an FTP server (pure standard library)
- **Encryption**: `NewAESEncryptor` (AES-256-GCM) and pluggable `IEncryption` factory for optional payload encryption

## Requirements

- Go 1.21 or later

## Installation

```bash
go get github.com/maxhaosl/CYGoLogger
```

## Quick Start

The fastest way — **one line** auto-mounts the console appender plus a full set
of rotated file appenders (Trace/Debug/Info/Warn/Error/Fatal/Main) under the
given path, mirroring the C++ `CY_LOG_CONFIG` macro. No manual `AddAppender`
calls needed:

```go
package main

import (
    gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"
)

func main() {
    // ONE line: console + all file appenders under ./Log
    gologger.InitDefault("./Log")
    defer gologger.Close()

    // Log messages (caller file:line captured automatically)
    gologger.Info("Application started")
    gologger.Debug("Debug info: version=%s", "1.0.0")
    gologger.Warn("This is a warning: id=%d", 42)
    gologger.Error("Error occurred: code=%d", 500)
    gologger.Fatal("Fatal error: aborting")
}
```

> Prefer the classic C-style API? `InitLogger` + `LOG_*` macros + manual
> `AddAppender` still work exactly as before (see [Log Macros](#log-macros)).

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

## Channel-Aware Logging

Every logging function also has a channel-aware variant that carries an explicit
`channel` string (the first argument), mirroring the C++ `ICYLogger::WriteLog(szChannel, ...)`
overloads. The channel is rendered by the layout (e.g. `[Channel:Name]` / the
`[channel]` bracket) and overrides the appender channel for that single message.

```go
// Idiomatic API (auto caller file/line/func capture)
gologger.InfoCh("ModuleA", "value = %d", val)
gologger.ErrorCh("ModuleB", "unexpected error: %v", err)
gologger.DebugCh("Net", "recv %d bytes", n)
gologger.HexInfoCh("Proto", payload)         // hex dump on a channel
gologger.EscapeInfoCh("Raw", "a ]bracketed, value")  // escaped on a channel

// Direct (bypass level filter) and legacy LOG_* forms also have _CH variants
gologger.DirectWarnCh("HotPath", "always shown: %s", msg)
gologger.LOG_TRACE_CH("Boot", "boot step %d", i)
gologger.LOG_DIRECT_INFO_CH("Boot", "forced info")
```

| Variant family | Channel-aware functions |
| --- | --- |
| Idiomatic | `TraceCh` `DebugCh` `InfoCh` `WarnCh` `ErrorCh` `FatalCh` `MainCh` `RemoteCh` `SysCh` |
| Direct (no filter) | `DirectTraceCh` … `DirectSysCh` |
| Hex | `HexTraceCh` … `HexSysCh` |
| Escape | `EscapeTraceCh` … `EscapeSysCh` |
| Legacy `LOG_*` | `LOG_TRACE_CH` … `LOG_REMOTE_CH`, `LOG_DIRECT_TRACE_CH` … `LOG_DIRECT_MAIN_CH` |

If a message has no channel of its own, the appender's channel (set via
`AddAppender`/`InitDefault`) is rendered instead, preserving the previous behaviour.

## One-Line Initialization & Top-Level API

For the most common cases you only need **one import and one line**:

```go
import gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"

func main() {
    gologger.InitDefault("./Log")   // zero-config: console + rotated file, level FILTER_ALL
    defer gologger.Close()          // flushes and releases everything

    gologger.Info("Application started: v%s", "1.0.0")
    gologger.Debug("Debug info: count=%d", 10)
    gologger.Warn("This is a warning")
    gologger.Error("Error occurred: code=%d", 500)

    // Hex dump (one call, auto file:line)
    gologger.HexInfo([]byte{0x48, 0x65, 0x6c, 0x6c, 0x6f})

    // Escape special characters safely
    gologger.EscapeInfo("Message with [brackets] and ]more[")
}
```

Top-level convenience functions (all capture the caller `file:line` automatically):

| Function family | Levels / Types |
|-----------------|----------------|
| `Trace/Debug/Info/Warn/Error/Fatal` | formatted (`fmt.Sprintf`) logging per level |
| `Main/Remote/Sys` | formatted logging to Main / Remote / Sys types |
| `HexTrace/HexDebug/.../HexFatal/HexMain/HexRemote/HexSys` | hex-dump a `[]byte` payload per level/type |
| `EscapeTrace/EscapeDebug/.../EscapeFatal/EscapeMain/EscapeRemote/EscapeSys` | escape special chars then log per level/type |
| `Flush()` / `FlushType(t)` | flush the async queue (all / by type) |
| `Close()` | flush + `UnInitLogger`, call in `defer` |

### Runtime Configuration Shortcuts

Configure the logger at runtime with one-line calls (no `GetInstance()` boilerplate):

```go
// Toggle Remote / Sys appender on the fly
gologger.SetWriteRemote(true)   // mounts TCP/UDP remote appender
gologger.SetWriteSys(true)      // mounts syslog appender

// Switch log level at runtime
gologger.SetLogLevel(gologger.LogFilterErrors)

// Apply a custom PatternFilter
filter := gologger.NewPatternFilter(",;|", "CSV", "PATTERN_CSV", gologger.TupleFieldType_CSV)
gologger.SetFilter(filter)

// Change layout format
gologger.SetLayout(gologger.LogLayoutTypeBuildin2)
```

> The classic `LOG_*` macros and `GetInstance().WriteLog*` methods remain fully supported (backward compatible).

## Crash (Exception) Logging

Capture panics with full stack traces into a dedicated, synchronous exception log file — independent of the async main channel, so it still works when the program is about to crash.

```go
// Enable crash logging (writes ./Log/Exception.log)
gologger.InitException("./Log")

// Option A: defer the helper at the top of any function
func risky() {
    defer gologger.Recover()
    mayPanic()
}

// Option B: run a goroutine safely (panics are caught automatically)
gologger.SafeGo(func() { backgroundWork() })

// Option C: custom panic handler
gologger.SetPanicHandler(func(recv any, stack string) {
    fmt.Fprintln(os.Stderr, "PANIC:", recv)
})

// Optional: re-throw after logging (default: swallow)
gologger.SetPanicRethrow(true)
```

Under the hood `Recover()` calls `recover()` directly inside the deferred function, and background goroutines spawned via `CYNamedThread`/`SafeGo` route their panics to the same exception log.

## Lightweight Synchronous Log (SimpleLog)

`SimpleLog` writes **synchronously** (no async queue), perfect for crash logs or tiny helper tools:

```go
lg := gologger.NewCYSimpleLogFile()
lg.InitLog("./Log/simple.log", gologger.SimpleLogTypeFile)
lg.WriteString("immediate, no buffering\n")
lg.WriteLog("formatted %d", 42)
lg.CloseLog()
lg.DeleteAllFile() // remove all files created by this logger
```

`SimpleLogTypeNone/File/Console/All` controls the destination; `CYSimpleLogConsole` adds ANSI colors.

## FTP Upload

Archive or push log files to an FTP server using a pure standard-library client (no third-party dependency):

```go
cfg := gologger.UpLoadConfig{
    Host: "ftp.example.com", Port: 21,
    User: "user", Password: "pass",
    RemoteDir: "/logs", TimeoutSec: 30, Passive: true,
}
if err := gologger.UploadLogFTP("./Log/app_20240101.log", cfg); err != nil {
    gologger.Error("upload failed: %v", err)
}
```

The `IUpLoad` interface and `GetCYUpLoadFactoryInstance()` factory allow adding other backends (S3, etc.) later.

## Encryption

Pluggable `IEncryption` with an AES-256-GCM sample. Useful for encrypting payloads before writing/archiving:

```go
enc := gologger.NewAESEncryptor("my-secret-passphrase")
cipher, _ := enc.Encrypt([]byte("plain text"))
plain, _ := enc.Decrypt(cipher) // -> "plain text"
```

`GetCYEncryptionFactoryInstance()` provides the factory; `EncryptionTypeNone` is a pass-through.

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
| `LogLayoutTypeBuildin4` | `[YYYY-MM-DD HH:MM:SS.mmm][TYPE&#124;P:pid&#124;T:tid][func(line)] Msg` |
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
├── inc.go                  # Single import entry point (re-exports everything)
├── api.go                  # Top-level InitDefault/Info/Hex*/Escape*/Recover/Close
├── Core/types.go           # Enums and constants
├── Common/
│   ├── common.go           # Message types, FPS counter, utilities, goroutine wrapper
│   ├── simplelog.go        # CYSimpleLogFile / CYSimpleLogConsole (sync log)
│   └── exception.go        # CYExceptionLogFile, Recover/SafeGo, panic capture
├── Filter/filter.go        # Pattern filter chain
├── Layout/
│   ├── interfaces.go       # ICYLogger, ICYLoggerTemplateLayout interfaces
│   └── layout.go           # Built-in and custom layouts
├── Config/config.go        # Global configuration singleton
├── Statistics/statistics.go # Atomic line/byte/FPS counters
├── Entity/entity.go        # Appender container and factory
├── Appender/appender.go    # 6 appender types (console/file/main/buffer/remote/system)
├── UpLoad/
│   ├── upload.go           # IUpLoad interface + factory
│   └── ftp.go              # CYFTPUpLoad (pure stdlib FTP client)
├── Encryption/
│   └── encryption.go       # IEncryption interface, factory, AES-GCM sample
├── Logger/logger.go       # CYLoggerControl and CYLoggerImpl
└── Schedule/
    └── CYLoggerSchedule.go # Background expired-file cleanup + zip
```

## One-Click Verification

CYGoLogger ships with a single command that proves **every** feature works, is
stable under `-race`, and performs well — the same gate used to keep the library
commercial-grade:

```bash
bash Build/verify.sh
```

It chains the following stages and exits non-zero the moment any stage fails:

| Stage | Command | What it proves |
|-------|---------|----------------|
| `build` | `go build ./...` | the whole module compiles |
| `vet` | `go vet ./...` | static correctness |
| `test-race` | `go test -race ./...` | functional + **stability** (concurrent writes, repeated Init/Close, graceful Flush) is race-clean |
| `bench` | `go test -bench=. -benchmem -benchtime 100x -run '^$'` | **efficiency** — throughput & `allocs/op` for write/layout/hex/escape/queue paths |
| `examples` | `go run .` in every `examples/*` | end-to-end behaviour, incl. `config_verify`'s 6 isolated `-opt` sub-processes (console / remote / sys / file-mode / layout / defaults) |

The final summary prints a `PASS: N  FAIL: M` matrix. macOS has no `timeout`
command, so the script never wraps stages in a timeout; the benchmark step uses
a short `-benchtime 100x` so a CI runner cannot hang.

### Feature matrix

The `examples/feature_verify` program is a human-readable feature matrix: it
exercises each capability through the public API and prints one `PASS`/`FAIL`
line per feature, e.g.

```
[K] Log levels & types (Trace/Debug/Info/Warn/Error/Fatal/Main)   ... PASS
[L] Channel-aware logging                                         ... PASS
[M] Direct logging bypasses level filter                          ... PASS
[N] Escape-formatted logging                                      ... PASS
[O] Hex dump logging                                              ... PASS
[P] Scope enter/exit logging                                      ... PASS
[Q] Concurrent async-safe writes                                  ... PASS
[R] Template layouts Buildin1..4                                  ... PASS
[S] Compression (ZipLog)                                          ... PASS
[T] AES-256-GCM encryption                                        ... PASS
[U] Statistics counters                                           ... PASS
[V] Panic / exception capture                                     ... PASS
[W] Entity inspection                                             ... PASS
[X] FTP upload (in-process server, end-to-end)                     ... PASS
=== RESULT: 90 passed, 0 failed ===
```

### What is covered

- **Levels & types**: Trace/Debug/Info/Warn/Error/Fatal/Main/Remote/Sys + `*Ch` channel variants.
- **Destinations**: console, file, Main, remote (TCP/UDP), system/syslog.
- **Rotation & limits**: per-file size + time rolling, expired-file cleanup, per-type count, per-type size, global size caps, non-log-file purge.
- **Async architecture**: double-buffered swap loop, `Flush`/`Close` graceful drain, no goroutine/appender leaks across repeated Init/Close.
- **Formatting**: 4 built-in layouts + custom, hex dumps, escape formatting.
- **Robustness**: level/channel filters, panic capture (`SafeGo`/`Recover`), `ForceNewFile`, AES-256-GCM encryption, FTP upload, zip compression, live statistics, `Entity` inspection.

## License

CYGoLogger is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Authors

- ShiLiang.Hao &lt;newhaosl@163.com&gt;
- foobra &lt;vipgs99@gmail.com&gt;

## References

- [CYLogger (C++)](https://github.com/maxhaosl/CYLogger) - The original C++ logging library
