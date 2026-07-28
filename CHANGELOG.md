# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.4] - 2026-07-28

### Changed

- **`LOG_LEVEL_FILTER` now also gates file creation**: a log level disabled by the effective `ELogLevelFilter` no longer generates its dedicated `.log` file at `Init()`. Previously the filter only suppressed *writes* (via `CYLoggerControl.passesFilter` in `Write`), but the per-type file appenders were still mounted — so filtered-out types produced empty files. Now `CYLoggerControl.typeEnabledByFilter` is consulted while mounting appenders in `Init()`, so a suppressed type produces neither output nor a file, fully mirroring the C++ `LOG_LEVEL_FILTER` "suppressed level is fully turned off" semantics.
  - `Main` is the aggregate of every enabled type and is therefore still mounted whenever any logging can occur (gated only by `EMode`, not by individual level bits).
  - `Sys`/`Remote` file appenders are likewise gated by the filter (in addition to the `LOG_WRITE_SYS`/`LOG_WRITE_REMOTE` switches).
  - In `ModeRelease`/`ModeDebug` the mode's own filter is authoritative, so behaviour there is unchanged.

### Added

- `ICYLogger/Logger/filter_test.go`: `TestLogLevelFilterSuppressesFileCreation` (only `Error`/`Fatal`/`Main` files for `LogFilterErrors`), `TestLogLevelFilterAllKeepsAllFiles` (all 6 + Main for `LogFilterAll`, regression guard), `TestLogLevelFilterWarnsAndErrors` (intermediate preset) — all run under the full `go test -race ./ICYLogger/...` suite.
- **`examples/level_filter_test`**: standalone example (`go run .`, exits non-zero on failure) that verifies `LOG_LEVEL_FILTER` end-to-end across three filters — `LogFilterErrors`, `LogFilterWarnsAndErrors`, `LogFilterAll`:
  - a filtered-out level produces **no** `*.log` file (file-existence assertion);
  - an enabled level produces its file with the **correct layout type marker** (`|T|`/`|D|`/`|I|`/`|W|`/`|E|`/`|F|`) and its own message, and **no cross-type leak** (no other level's message appears in the file);
  - the `Main` aggregate contains exactly the enabled levels' messages and none of the filtered ones.
  - Picked up automatically by `Build/verify.sh` (runs `go run .` for every example dir), so it gates the one-click verification.

### Known limitation

- The filter is applied at `Init()` mount time. A level re-enabled at **runtime** via `SetLogLevel` (after `Init` with it disabled) will pass `passesFilter` but its file appender is not lazily mounted, so those messages are dropped until the next `Init`. On-demand lazy mounting is intentionally out of scope here; the typical usage sets the filter once before `Init`.

## [0.3.3] - 2026-07-28

### Added

- **`examples/stress_test` correctness suite** (`correctness.go`, tests 9–12 — the suite now has 12 tests total, all runnable via `-test=all`):
  - `alllevels` — writes all six levels (Trace/Debug/Info/Warn/Error/Fatal) concurrently and verifies, per level: the statistics counter, the on-disk line count, that **every** line carries the correct layout type code (`|T|`, `|D|`, …) and level marker (no cross-type mixing), that the Main log aggregates exactly 6×N lines, and that no line ever regresses to `T:0` (GetGID regression guard)
  - `layout` — asserts the **exact full-line output format** of built-in layouts 1–4 via anchored regexes, both without and with a channel (`[Channel:X]` for layouts 1–3, bare `[X]` for layout 4)
  - `channel` — 4 channels × 25 K lines + 25 K plain lines written concurrently; verifies exact per-channel counts, that plain lines carry no `Channel:` field, and the Main double-write total
  - `filemode` — verifies `LogFileModeAppend` keeps a single fixed `Info.log` accumulating across `Init/Close` cycles, and `LogFileModeTime` produces one `Info_YYYYMMDD_HHMMSS.log` per start with the exact expected name pattern
- `Build/verify.sh` now runs the full 12-test stress suite with short durations as a dedicated gate stage (`example:stress_test(12 tests)`)

### Fixed

- **`examples/stress_test` baseline**: `initLog` now also re-asserts `WithLayoutType(Buildin1)` — the config singleton retains the last layout across `Close()`, so in `-test=all` the `layout` test (which switches to Buildin4) previously leaked its layout into the subsequent `channel` test and broke the `[Channel:...]` assertions

### Verified

- Full suite 12/12 PASS: benchmark P50 9 µs @1 worker (~100 K lines/sec), zero loss from 1 to 2048 goroutines; `Build/verify.sh` PASS 20 / FAIL 0

## [0.3.2] - 2026-07-28

### Fixed

- **`Common/common.go` (`GetGID`)**: the goroutine-ID parser split the stack header with `strings.Fields` and then searched for `"["` *inside* the word `"goroutine"`, which never matches — so `GetGID()` silently returned **0** for every goroutine and the `T:` (thread-id) field in every log line was always `T:0`. Now parses the digits directly from the raw `runtime.Stack` header (`"goroutine <id> ["`): correct IDs, zero intermediate allocations. Regression test added (`TestGetGID`)

### Changed / Performance

- **`Common/common.go` (`GetCurrentProcessId`)**: the PID is now captured once at startup instead of issuing a real `os.Getpid()` syscall per log line (visible in CPU profiles)
- Single-goroutine throughput ≈103 K lines/sec (was ≈98 K); remaining per-line costs are the real disk `write(2)` (~38 % CPU, unavoidable) and `runtime.Stack` for the goroutine ID (~27 %, inherent — Go has no goroutine-local storage)
- Full gate re-verified: `Build/verify.sh` PASS 20 / FAIL 0; `examples/stress_test` 8/8 zero-loss up to 2048 goroutines; library race-clean

## [0.3.1] - 2026-07-28

### Fixed

- **`Logger/logger.go` (console appender)**: `Init()` previously created a console appender on the *first* `Init` regardless of `WithConsole(false)` (the condition was `bShowConsole || c.consoleApp == nil`), so `WithConsole(false)` / `LOG_SHOW_CONSOLE_WINDOW=false` was silently ignored and every log line flooded `stdout` — this also broke the stress test (hundreds of thousands of lines flooded the terminal and looked like a hang). Console appender is now created **only** when `bShowConsole` is true and destroyed when false
- **`Logger/logger.go` (cleanup scheduler)**: `SetLogDir`/`SetExpiredHours`/`SetClearPeriodSec` were guarded by `if c.schedule == nil`, so after a `Close()` + re-`Init()` in the same process the scheduler kept cleaning the **previous** log directory with the **previous** period and never cleaned the new one. Now re-asserted on every `Init`
- **`Common/common.go` (`CYTimeStamps`)**: removed a global `sync.Mutex` that wrapped the pure `fmt.Sprintf` timestamp formatter, which serialised every log line's timestamp formatting across all goroutines

### Changed / Performance

- **`Appender/appender.go` (File & Main appenders)**: eliminated a per-line `os.Stat` syscall used for size-rotation checks — a new in-memory `nCurrentSize` byte counter (seeded on open, incremented per write, including buffered bytes) now drives rotation and `GetSize()`
- **`Appender/appender.go`**: replaced the per-line `write(2)` syscall with a **64 KB `bufio` write buffer** plus a 1-second periodic flush (inside the existing rotation goroutine). Rotation, `UnInit`, `Copy`, and `Flush` all flush the buffer first (`closeFileLocked`/`Flush` unified), bounding data loss on a hard crash to at most one flush interval (~1 s)
- **`Appender/appender.go` (`ClearContents`)**: now truly truncates the file (was opened `O_APPEND` and therefore never actually cleared)
- **Throughput**: single-goroutine file write improved ~3.4× (≈28,961 → ≈98,713 lines/sec); verified race-clean, with 200 K-line integrity, size rotation, count-limit cleanup, and extreme concurrent (up to 2048 goroutine) stress tests all passing zero-loss

### Known Limitations

- A single log file requires a single ordered writer, so under heavy concurrency the aggregate rate approximates the single-writer rate (~100 K lines/sec, including the Main double-write). To scale further, disable the Main double-write or shard logs across channels/files

## [0.3.0] - 2026-07-28

### Added

- **One-click verification gate** `Build/verify.sh`: chains `build → vet → test -race → bench (short) → all examples` (including `config_verify`'s 6 isolated `-opt` sub-processes) and prints a final `PASS/FAIL` summary; exits non-zero on any failure (no `timeout` dependency, macOS-safe)
- **Unit tests** (stdlib `testing` only, zero third-party deps):
  - `Filter/filter_test.go` — pattern-filter add/update/remove, defensive field copy, chain links, escape/delimiter semantics, chain & manager singletons
  - `Encryption/encryption_test.go` — AES-256-GCM round-trip, nonce uniqueness, tamper/wrong-key detection, error paths, None pass-through, factory dispatch
  - `Entity/entity_test.go` — appender add/remove, enable-aware write routing, aggregation (GetLogName/GetSize/Flush/ForceNewFile fan-out), UnInit detach, entity factory (with in-memory `fakeAppender`)
  - `UpLoad/upload_test.go` — FTP upload end-to-end against an in-process stdlib FTP server (PASV parsing, path & byte-exact payload assertions)
  - `Logger/logger_test.go` — level mapping, level/channel filters, hex formatting, caller capture, end-to-end write aggregation, pre-Init safety, statistics
  - `Core/config_options_test.go`, `Core/defaults_test.go`, `Appender/rotation_verify_test.go`, `Schedule/cleanup_verify_test.go`
- **Stability tests** `Logger/stability_test.go`: 16-goroutine concurrent mixed-type writes with live `SetLogLevel`/`Flush`, repeated Init/Close cycles with goroutine-leak check, idempotent Close, 2s sustained-load soak (`-short` aware); whole library is race-clean under `go test -race ./ICYLogger/...`
- **Benchmarks** (`-benchmem`, allocs/op quantified): `Logger` (fmt/parallel/no-format/escape/hex/filtered), `Layout` (Buildin1–4, escaped, timestamps), `Common` (message pool, clone, FPS counter), `Appender` (file write serial/parallel)
- **`examples/feature_verify` full feature matrix**: expanded to 90 checks across [A]–[X] (levels/types, channels, direct-bypass, escape, hex, scope, concurrency, layouts, rotation, cleanup, limits, zip, AES-GCM, statistics, panic capture, entity inspection, FTP upload end-to-end); prints per-feature PASS/FAIL and exits non-zero on failure
- **`examples/config_verify`** and **`examples/limit_verify`**: isolated per-process option verification (console/remote/sys/file-mode/layout/defaults) and restriction-policy verification, with proper exit codes
- **README**: new "One-Click Verification" section with stage table and feature matrix

### Fixed

- **`Logger/logger.go`**: `Init()` did not reset `bExit` after a prior `UnInit()`, so every write after the first `Close()` + re-`Init()` was silently dropped for the rest of the process
- **`Appender/appender.go` (base appender)**: data race — producer-side `Write()`/`GetQueueSize()` read the `PublicQueue` pointer without the lock while the consumer's `swapAndProcess` swapped queue pointers under `a.mu`; queue pointers are now snapshotted under the lock (same fix applied to the `Flush`-fallback `processMessages`)
- **`CYLoggerFileAppender` / `CYLoggerMainAppender`**: data race — synchronous `doWrite` read/wrote `a.file` / `a.szCurrentFile` without holding `a.mu`, racing against `rotateFileLocked` and the time-rotation ticker; the whole read-rotate-write sequence is now guarded by `a.mu`
- **`api.go` `InitDefaultWithOpts`**: no longer forces `SetShowConsole(true)` before applying options, so an explicit `WithConsole(false)` is honoured (matches C++ `LOG_SHOW_CONSOLE_WINDOW=false` default; `InitDefault` still enables console for convenience)

## [0.2.0] - 2025-07-24

### Added

- **Top-level runtime shortcuts**: `SetWriteRemote(bool)`, `SetWriteSys(bool)`, `SetLogLevel(ELogLevelFilter)`, `SetFilter(*PatternFilter)`, `SetLayout(ELogLayoutType)` — configure logger settings without calling `GetInstance()` boilerplate
- **Hex/Escape variants for Main/Remote/Sys**: `HexMain`, `HexRemote`, `HexSys`, `EscapeMain`, `EscapeRemote`, `EscapeSys` — hex-dump and escape-safe logging for all log types
- **Remote/Sys auto-mount on init**: `InitDefault` / `InitDefaultWithOpts` automatically mount Remote and Sys appenders when `WithWriteRemote(true)` / `WithWriteSys(true)` are set, mirroring C++ `CY_LOG_APPENDER` full-set behaviour
- **Statistics.Reset()**: `FreeInstance()` now calls `Statistics.Reset()` to zero all counters on teardown, aligning with C++ `CYLoggerImpl::FreeInstance → Statistics()->Reset()`
- **AppenderFactory NONE→Console branch**: `CreateAppender` now treats `LogTypeNone` as a console appender, matching C++ `CreateFileAppender` semantics
- **Windows `GetExePath()`**: Cross-platform executable directory detection with `//go:build windows` build tag, fallback to `os.Getwd` on non-Windows
- **Common alignment**: `CYPublicFunction` toolset (PrintLog, PrintTraceLog, TrimString, GetLocalUTCOffsetHours, Verify), `NowNano` rdtsc equivalent, `Message` three-type aliases

### Changed

- **FPS window**: detection window changed from 1s to 5s (`LOG_FPS_CHECK_DURATION`), matching C++ default

### Fixed

- Removed stale empty test files (`zz_close_diag_test.go`, `zz_verify_test.go`) that caused compilation failures

### Known Limitations

- `CYLoggerSystemAppender.doWrite` is not wired in `Run()` — syslog output is a stub
- `CYLoggerRemoteAppender` uses TCP instead of the C++ UDP protocol
- `CYLoggerMainAppender` does not inherit the 6-level per-type buffer queue like its C++ counterpart
- File cleanup only supports age-based expiration (not count, type-size, or total-size limits)
- `CY_SCOPE()` RAII scope logging macro has no Go equivalent

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
