/*
 * CYGoLogger License
 * -----------
 *
 * CYGoLogger is licensed under the terms of the MIT license reproduced below.
 * This means that CYGoLogger is free software and can be used for both academic
 * and commercial purposes at absolutely no cost.
 *
 * ===============================================================================
 *
 * Copyright (C) 2023-2024 ShiLiang.Hao <newhaosl@163.com>, foobra<vipgs99@gmail.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

package ICYLogger

import (
	"fmt"
	"runtime"
	"strings"

	Appender "github.com/maxhaosl/CYGoLogger/ICYLogger/Appender"
	Common "github.com/maxhaosl/CYGoLogger/ICYLogger/Common"
	Core "github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
	Encryption "github.com/maxhaosl/CYGoLogger/ICYLogger/Encryption"
	Filter "github.com/maxhaosl/CYGoLogger/ICYLogger/Filter"
	Layout "github.com/maxhaosl/CYGoLogger/ICYLogger/Layout"
	Logger "github.com/maxhaosl/CYGoLogger/ICYLogger/Logger"
	Schedule "github.com/maxhaosl/CYGoLogger/ICYLogger/Schedule"
	UpLoad "github.com/maxhaosl/CYGoLogger/ICYLogger/UpLoad"
)

// =============================================================================
// Internal helpers
// =============================================================================

// apiCallerInfo captures the caller's file, function name, and line number
// from the perspective of a top-level API function. skip=1 yields the caller
// of the exported function (e.g. Debug, Info), which is the user's source location.
func apiCallerInfo(skip int) (file, funcName string, line int) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "???", "???", 0
	}
	funcName = runtime.FuncForPC(pc).Name()
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		file = file[idx+1:]
	}
	if idx := strings.LastIndex(file, "\\"); idx >= 0 {
		file = file[idx+1:]
	}
	if idx := strings.LastIndex(funcName, "/"); idx >= 0 {
		funcName = funcName[idx+1:]
	}
	return
}

// =============================================================================
// Go-idiomatic convenience logging functions with automatic caller capture
//
// Usage:
//   import gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"
//
//   gologger.InitDefault("./logs")
//   gologger.Debug("value = %d", val)
//   gologger.Info("processing item %s", name)
//   gologger.Warn("deprecated API called")
//   gologger.Error("unexpected error: %v", err)
//
// These functions use runtime.Caller to automatically capture the caller's
// file name, line number, and function name — no manual __FILE__/__LINE__ needed.
// =============================================================================

// Trace writes a trace-level log message (log type Trace, most verbose).
func Trace(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmt(int(Core.LogLevelTrace), Core.LogTypeTrace, -1, file, funcName, line, format, args...)
}

// Debug writes a debug-level log message.
func Debug(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmt(int(Core.LogLevelDebug), Core.LogTypeDebug, -1, file, funcName, line, format, args...)
}

// Info writes an info-level log message.
func Info(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, file, funcName, line, format, args...)
}

// Warn writes a warning-level log message.
func Warn(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmt(int(Core.LogLevelWarn), Core.LogTypeWarn, -1, file, funcName, line, format, args...)
}

// Error writes an error-level log message.
func Error(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmt(int(Core.LogLevelError), Core.LogTypeError, -1, file, funcName, line, format, args...)
}

// Fatal writes a fatal-level log message (highest severity).
func Fatal(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmt(int(Core.LogLevelFatal), Core.LogTypeFatal, -1, file, funcName, line, format, args...)
}

// Main writes a log message to the main application log (LogTypeMain).
func Main(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeMain, -1, file, funcName, line, format, args...)
}

// Remote writes a log message to the remote log (LogTypeRemote, TCP).
func Remote(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmt(int(Core.LogLevelRemote), Core.LogTypeRemote, -1, file, funcName, line, format, args...)
}

// Sys writes a log message to the system event log (LogTypeSys, syslog/EventLog).
func Sys(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmt(int(Core.LogLevelSys), Core.LogTypeSys, -1, file, funcName, line, format, args...)
}

// Console writes a log message to the console appender only.
func Console(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmt(int(Core.LogLevelConsole), Core.LogTypeConsole, -1, file, funcName, line, format, args...)
}

// =============================================================================
// Channel-aware convenience logging functions
//
// These mirror the basic Trace/Debug/Info/... helpers but carry an explicit
// channel string that is rendered by the layout (the [channel] / Channel:
// field), mirroring the C++ WriteLog(szChannel, ...) overloads.
//
// Usage:
//   gologger.InfoCh("ModuleA", "value = %d", val)
//   gologger.ErrorCh("ModuleB", "unexpected error: %v", err)
// =============================================================================

// TraceCh writes a trace-level log to the given channel.
func TraceCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtCh(int(Core.LogLevelTrace), Core.LogTypeTrace, -1, channel, file, funcName, line, format, args...)
}

// DebugCh writes a debug-level log to the given channel.
func DebugCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtCh(int(Core.LogLevelDebug), Core.LogTypeDebug, -1, channel, file, funcName, line, format, args...)
}

// InfoCh writes an info-level log to the given channel.
func InfoCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtCh(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, channel, file, funcName, line, format, args...)
}

// WarnCh writes a warn-level log to the given channel.
func WarnCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtCh(int(Core.LogLevelWarn), Core.LogTypeWarn, -1, channel, file, funcName, line, format, args...)
}

// ErrorCh writes an error-level log to the given channel.
func ErrorCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtCh(int(Core.LogLevelError), Core.LogTypeError, -1, channel, file, funcName, line, format, args...)
}

// FatalCh writes a fatal-level log to the given channel.
func FatalCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtCh(int(Core.LogLevelFatal), Core.LogTypeFatal, -1, channel, file, funcName, line, format, args...)
}

// MainCh writes a main-type log to the given channel.
func MainCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtCh(int(Core.LogLevelInfo), Core.LogTypeMain, -1, channel, file, funcName, line, format, args...)
}

// RemoteCh writes a remote log to the given channel.
func RemoteCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtCh(int(Core.LogLevelRemote), Core.LogTypeRemote, -1, channel, file, funcName, line, format, args...)
}

// SysCh writes a system log to the given channel.
func SysCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtCh(int(Core.LogLevelSys), Core.LogTypeSys, -1, channel, file, funcName, line, format, args...)
}

// =============================================================================
// Scope — enter/exit scope logging with defer
//
// Usage:
//   func myFunc() {
//       defer gologger.Scope()()
//       // ... function body ...
//   }
//   // Output:
//   //   [TRACE] ENTER: myFunc (myfile.go:42)
//   //   [TRACE] EXIT:  myFunc (myfile.go:42)
// =============================================================================

// Scope returns a deferred function that logs function entry now and exit later.
// It uses runtime.Caller to capture the caller's location automatically.
func Scope() func() {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmt(
		int(Core.LogLevelTrace), Core.LogTypeTrace, -1,
		file, funcName, line,
		fmt.Sprintf("ENTER: %s", funcName),
	)
	return func() {
		Logger.GetInstance().WriteLogFmt(
			int(Core.LogLevelTrace), Core.LogTypeTrace, -1,
			file, funcName, line,
			fmt.Sprintf("EXIT:  %s", funcName),
		)
	}
}

// =============================================================================
// One-click initialization
//
// Usage:
//   gologger.InitDefault("./logs")          // simplest: enable console
//   gologger.InitDefaultWithOpts("./logs",
//       gologger.WithConsole(false),
//       gologger.WithWriteRemote(true),
//   )
// =============================================================================

// InitDefault initializes the logger with sensible defaults and auto-mounts a
// full set of file appenders (Trace/Debug/Info/Warn/Error/Fatal/Main) plus the
// console appender — exactly one call is enough to start writing, mirroring the
// C++ CY_LOG_CONFIG macro.
// - logPath: directory for log files (created if missing)
// - Console output is enabled by default
// - Uses built-in layout 1 with time-based rolling file names
func InitDefault(logPath string) bool {
	return InitDefaultWithOpts(logPath, WithConsole(true))
}

// Option is a functional option for configuring InitDefaultWithOpts.
type Option func(*Core.CYLoggerConfig)

// WithConsole enables or disables console output. The C++ default for
// LOG_SHOW_CONSOLE_WINDOW is false; InitDefault turns it on for convenience,
// while InitDefaultWithOpts honours whatever the caller passes (no forced
// default), so WithConsole(false) actually disables the console appender.
func WithConsole(b bool) Option {
	return func(c *Core.CYLoggerConfig) { c.SetShowConsole(b) }
}

// WithWriteRemote enables or disables remote TCP log output (default: disabled).
func WithWriteRemote(b bool) Option {
	return func(c *Core.CYLoggerConfig) { c.SetWriteRemote(b) }
}

// WithWriteSys enables or disables system event log output (default: disabled).
func WithWriteSys(b bool) Option {
	return func(c *Core.CYLoggerConfig) { c.SetWriteSys(b) }
}

// WithFileMode sets the log file naming mode (default: LogFileModeTime).
func WithFileMode(m Core.ELogFileMode) Option {
	return func(c *Core.CYLoggerConfig) { c.SetFileMode(m) }
}

// WithLayoutType sets the default layout template (default: LogLayoutTypeBuildin1).
func WithLayoutType(t Core.ELogLayoutType) Option {
	return func(c *Core.CYLoggerConfig) { c.SetLayoutType(t) }
}

// WithLogLevel sets the log level filter (default: all levels enabled).
func WithLogLevel(f Core.ELogLevelFilter) Option {
	return func(c *Core.CYLoggerConfig) { c.SetLogLevelFilter(f) }
}

// WithMode sets the logger mode, controlling which log-type file appenders are
// mounted and the effective level filter. It implements the retrieval project's
// Debug/Release switch:
//   gologger.WithMode(gologger.ModeRelease) // only Warn/Error files; Trace/Info never created or recorded
//   gologger.WithMode(gologger.ModeDebug)   // Trace/Info/Warn/Error files (4 types)
//   gologger.WithMode(gologger.ModeAll)     // all built-in file types (backward compatible)
func WithMode(m Core.EMode) Option {
	return func(c *Core.CYLoggerConfig) { c.SetMode(m) }
}

// WithRemoteAddr sets the remote log server address (host:port).
// Default is "127.0.0.1:7000". Only used when WithWriteRemote(true) is also set.
func WithRemoteAddr(addr string) Option {
	return func(c *Core.CYLoggerConfig) { c.SetRemoteAddr(addr) }
}

// WithRemoteProto selects the remote transport protocol. Default is TCP; pass
// Core.RemoteProtoUDP to align wire-compatibility with the C++ UDP remote appender
// (900-byte datagram packets). Only used when WithWriteRemote(true) is also set.
func WithRemoteProto(p Core.ERemoteProto) Option {
	return func(c *Core.CYLoggerConfig) { c.SetRemoteProto(p) }
}

// WithClearPeriodSec sets the cleanup/rotation check period in seconds.
// Default is 60. Older log files and over-quota files are pruned on this cadence.
func WithClearPeriodSec(n int) Option {
	return func(c *Core.CYLoggerConfig) { c.SetClearPeriodSec(n) }
}

// WithErrorLogPath sets the dedicated error log file path. A bare file name is
// resolved against the configured log directory.
func WithErrorLogPath(p string) Option {
	return func(c *Core.CYLoggerConfig) { c.SetErrorLogPath(p) }
}

// WithRestriction configures the file rotation policy. It mirrors the ten
// parameters of the C++ CYLoggerImpl::SetRestriction; when omitted, sane
// defaults are applied automatically during Init.
func WithRestriction(bEnable, bClear bool,
	nTimeClear, nTimeExpired, nSizeTime, nCountTime, nSize, nCount, nTypeSize, nAllSize int) Option {
	return func(c *Core.CYLoggerConfig) {
		c.SetRestriction(bEnable, bClear, nTimeClear, nTimeExpired, nSizeTime, nCountTime, nSize, nCount, nTypeSize, nAllSize)
	}
}

// DefaultConfig returns a freshly initialised *CYLoggerConfig populated with the
// same default values as the C++ CY_LOG_CONFIG macro (see Core/defaults.go and
// Inc/ICYLoggerDefine.hpp). Use it to inspect or tweak defaults before Init:
//
//	cfg := gologger.DefaultConfig()
//	cfg.SetShowConsole(true)
//	gologger.GetCYLoggerConfigInstance() // ... or pass through options
func DefaultConfig() *Core.CYLoggerConfig { return Core.DefaultConfig() }

// InitDefaultWithOpts initializes the logger with the given path and functional options.
//
// Example:
//
//	gologger.InitDefaultWithOpts("./logs",
//	    gologger.WithWriteRemote(true),
//	    gologger.WithFileMode(Core.LogFileModeAppend),
//	)
func InitDefaultWithOpts(logPath string, opts ...Option) bool {
	config := Core.GetCYLoggerConfigInstance()
	config.SetLogPath(logPath)
	// NOTE: do NOT force a console default here — that would override an explicit
	// WithConsole(false) and break LOG_SHOW_CONSOLE_WINDOW=false. InitDefault (the
	// no-options entry point) already passes WithConsole(true), and the config
	// singleton's own default for bShowConsole is false, matching the C++ default.
	for _, opt := range opts {
		opt(config)
	}
	instance := Logger.GetInstance()
	instance.Init()
	return instance.IsInit()
}

// =============================================================================
// Direct logging — bypass level filter, with auto caller info
//
// These functions write directly to the target log type entity, bypassing
// the level filter check. Useful for forced logging that should always appear
// regardless of the current log level configuration.
//
// Usage:
//   gologger.DirectDebug("always shown debug message: %s", detail)
//   gologger.DirectError("critical: %v", err)
// =============================================================================

// DirectDebug writes a debug log directly, bypassing level filtering.
func DirectDebug(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirect(Core.LogTypeDebug, -1, file, funcName, line, format, args...)
}

// DirectInfo writes an info log directly, bypassing level filtering.
func DirectInfo(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirect(Core.LogTypeInfo, -1, file, funcName, line, format, args...)
}

// DirectWarn writes a warn log directly, bypassing level filtering.
func DirectWarn(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirect(Core.LogTypeWarn, -1, file, funcName, line, format, args...)
}

// DirectError writes an error log directly, bypassing level filtering.
func DirectError(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirect(Core.LogTypeError, -1, file, funcName, line, format, args...)
}

// DirectFatal writes a fatal log directly, bypassing level filtering.
func DirectFatal(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirect(Core.LogTypeFatal, -1, file, funcName, line, format, args...)
}

// DirectTrace writes a trace log directly, bypassing level filtering.
func DirectTrace(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirect(Core.LogTypeTrace, -1, file, funcName, line, format, args...)
}

// DirectMain writes a main log directly, bypassing level filtering.
func DirectMain(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirect(Core.LogTypeMain, -1, file, funcName, line, format, args...)
}

// DirectRemote writes a remote log directly, bypassing level filtering.
func DirectRemote(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirect(Core.LogTypeRemote, -1, file, funcName, line, format, args...)
}

// DirectSys writes a system log directly, bypassing level filtering.
func DirectSys(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirect(Core.LogTypeSys, -1, file, funcName, line, format, args...)
}

// =============================================================================
// Direct channel-aware logging — bypass level filter, with auto caller info
// =============================================================================

// DirectTraceCh writes a trace log to the channel directly, bypassing level filtering.
func DirectTraceCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirectCh(Core.LogTypeTrace, -1, channel, file, funcName, line, format, args...)
}

// DirectDebugCh writes a debug log to the channel directly, bypassing level filtering.
func DirectDebugCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirectCh(Core.LogTypeDebug, -1, channel, file, funcName, line, format, args...)
}

// DirectInfoCh writes an info log to the channel directly, bypassing level filtering.
func DirectInfoCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirectCh(Core.LogTypeInfo, -1, channel, file, funcName, line, format, args...)
}

// DirectWarnCh writes a warn log to the channel directly, bypassing level filtering.
func DirectWarnCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirectCh(Core.LogTypeWarn, -1, channel, file, funcName, line, format, args...)
}

// DirectErrorCh writes an error log to the channel directly, bypassing level filtering.
func DirectErrorCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirectCh(Core.LogTypeError, -1, channel, file, funcName, line, format, args...)
}

// DirectFatalCh writes a fatal log to the channel directly, bypassing level filtering.
func DirectFatalCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirectCh(Core.LogTypeFatal, -1, channel, file, funcName, line, format, args...)
}

// DirectMainCh writes a main log to the channel directly, bypassing level filtering.
func DirectMainCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirectCh(Core.LogTypeMain, -1, channel, file, funcName, line, format, args...)
}

// DirectRemoteCh writes a remote log to the channel directly, bypassing level filtering.
func DirectRemoteCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirectCh(Core.LogTypeRemote, -1, channel, file, funcName, line, format, args...)
}

// DirectSysCh writes a system log to the channel directly, bypassing level filtering.
func DirectSysCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteLogFmtDirectCh(Core.LogTypeSys, -1, channel, file, funcName, line, format, args...)
}

// =============================================================================
// Log file utilities
// =============================================================================

// ZipLog compresses the log file at src into the zip archive named dst.
// It returns true on success. This is a thin convenience wrapper over the
// background Schedule's DoZipLog (real archive/zip compression).
func ZipLog(src, dst string) bool {
	return Schedule.GetCYLoggerScheduleInstance().DoZipLog(src, dst)
}

// =============================================================================
// Flush / Close helpers
// =============================================================================

// Flush flushes all pending log messages across every appender.
func Flush() {
	Logger.FlushLogger()
}

// FlushType flushes pending messages for a single log type.
func FlushType(eType Core.ELogType) {
	Logger.FlushLoggerType(eType)
}

// Close flushes and shuts down the logger, releasing all resources.
// Typically deferred right after InitDefault:
//
//	gologger.InitDefault("./logs")
//	defer gologger.Close()
func Close() {
	Logger.FlushLogger()
	Logger.UnInitLogger()
	Common.GetCYExceptionLogFileInstance().CloseLog()
}

// =============================================================================
// Runtime configuration shortcuts (one-line API)
//
// These functions let you reconfigure the logger after InitDefault without a
// full re-Init:
//   gologger.SetLogLevel(gologger.LogFilterWarnsAndErrors)
//   gologger.SetLayout(gologger.LogLayoutTypeBuildin2)
//   gologger.SetWriteRemote(true)   // auto-mounts the remote appender
// =============================================================================

// SetWriteRemote toggles remote (TCP/UDP) log output at runtime. When enabled
// it auto-mounts the remote appender (mirroring C++ SetWriteRemote(true), which
// attaches the remote appender on demand); when disabled it unmounts it. No
// full re-Init is required. Safe to call any time after InitDefault.
func SetWriteRemote(b bool) {
	cfg := Core.GetCYLoggerConfigInstance()
	cfg.SetWriteRemote(b)
	appFactory := Appender.GetCYLoggerAppenderFactoryInstance()
	if b {
		// The entity factory pre-seeds an empty entity for every log type, so
		// the reliable "is it actually mounted" signal is a registered appender.
		if len(appFactory.GetAppenders(Core.LogTypeRemote)) == 0 {
			Logger.GetInstance().AddAppender(Core.LogTypeRemote, "", cfg.GetRemoteAddr(), cfg.GetFileMode())
		}
	} else {
		if len(appFactory.GetAppenders(Core.LogTypeRemote)) > 0 {
			Logger.GetInstance().ReleaseLoggerEntity(Core.LogTypeRemote)
		}
	}
}

// SetWriteSys toggles system event log (syslog/EventLog) output at runtime.
// When enabled it auto-mounts the system appender (mirroring C++
// SetWriteSys(true)); when disabled it unmounts it. No full re-Init required.
func SetWriteSys(b bool) {
	cfg := Core.GetCYLoggerConfigInstance()
	cfg.SetWriteSys(b)
	appFactory := Appender.GetCYLoggerAppenderFactoryInstance()
	if b {
		if len(appFactory.GetAppenders(Core.LogTypeSys)) == 0 {
			Logger.GetInstance().AddAppender(Core.LogTypeSys, "", "", cfg.GetFileMode())
		}
	} else {
		if len(appFactory.GetAppenders(Core.LogTypeSys)) > 0 {
			Logger.GetInstance().ReleaseLoggerEntity(Core.LogTypeSys)
		}
	}
}

// SetLogLevel sets the global log level filter — a one-line shortcut over the
// per-instance control, mirroring C++ SetLogLevel. It takes effect immediately
// on every subsequent write (the control re-checks the filter per message).
//   gologger.SetLogLevel(gologger.LogFilterWarnsAndErrors)
func SetLogLevel(filter Core.ELogLevelFilter) {
	Logger.GetInstance().SetLogLevel(filter)
}

// SetFilter installs a custom pattern filter as the global filter, mirroring
// C++ CYLoggerControl::SetFilter. Pass nil to fall back to the default filter.
func SetFilter(pFilter *Filter.ICYLoggerPatternFilter) {
	Logger.GetInstance().SetFilter(pFilter)
}

// SetLayout switches the global template layout to the built-in/custom layout
// of the given type, mirroring C++ CYLoggerControl::SetLayout. The resolved
// layout object is shared with the layout manager, so appenders created
// afterwards pick it up automatically.
//   gologger.SetLayout(gologger.LogLayoutTypeBuildin2)
func SetLayout(eType Core.ELogLayoutType) {
	layout := Layout.GetCYLoggerTemplateLayoutManagerInstance().GetLayout(eType)
	Logger.GetInstance().SetLayout(eType, layout)
}

// =============================================================================
// Hex logging — dump binary data as a formatted hex + ASCII table
//
// Usage:
//   gologger.HexInfo(payload)   // hex dump at info level
// =============================================================================

// HexTrace writes a hex dump of data at trace level.
func HexTrace(data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLog(int(Core.LogLevelTrace), Core.LogTypeTrace, -1, file, funcName, line, data)
}

// HexDebug writes a hex dump of data at debug level.
func HexDebug(data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLog(int(Core.LogLevelDebug), Core.LogTypeDebug, -1, file, funcName, line, data)
}

// HexInfo writes a hex dump of data at info level.
func HexInfo(data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLog(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, file, funcName, line, data)
}

// HexWarn writes a hex dump of data at warn level.
func HexWarn(data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLog(int(Core.LogLevelWarn), Core.LogTypeWarn, -1, file, funcName, line, data)
}

// HexError writes a hex dump of data at error level.
func HexError(data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLog(int(Core.LogLevelError), Core.LogTypeError, -1, file, funcName, line, data)
}

// HexFatal writes a hex dump of data at fatal level.
func HexFatal(data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLog(int(Core.LogLevelFatal), Core.LogTypeFatal, -1, file, funcName, line, data)
}

// HexMain writes a hex dump of data to the main application log.
func HexMain(data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLog(int(Core.LogLevelInfo), Core.LogTypeMain, -1, file, funcName, line, data)
}

// HexRemote writes a hex dump of data to the remote log.
func HexRemote(data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLog(int(Core.LogLevelRemote), Core.LogTypeRemote, -1, file, funcName, line, data)
}

// HexSys writes a hex dump of data to the system event log.
func HexSys(data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLog(int(Core.LogLevelSys), Core.LogTypeSys, -1, file, funcName, line, data)
}

// =============================================================================
// Hex channel-aware logging
// =============================================================================

// HexTraceCh writes a hex dump of data to the channel at trace level.
func HexTraceCh(channel string, data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLogCh(int(Core.LogLevelTrace), Core.LogTypeTrace, -1, channel, file, funcName, line, data)
}

// HexDebugCh writes a hex dump of data to the channel at debug level.
func HexDebugCh(channel string, data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLogCh(int(Core.LogLevelDebug), Core.LogTypeDebug, -1, channel, file, funcName, line, data)
}

// HexInfoCh writes a hex dump of data to the channel at info level.
func HexInfoCh(channel string, data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLogCh(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, channel, file, funcName, line, data)
}

// HexWarnCh writes a hex dump of data to the channel at warn level.
func HexWarnCh(channel string, data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLogCh(int(Core.LogLevelWarn), Core.LogTypeWarn, -1, channel, file, funcName, line, data)
}

// HexErrorCh writes a hex dump of data to the channel at error level.
func HexErrorCh(channel string, data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLogCh(int(Core.LogLevelError), Core.LogTypeError, -1, channel, file, funcName, line, data)
}

// HexFatalCh writes a hex dump of data to the channel at fatal level.
func HexFatalCh(channel string, data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLogCh(int(Core.LogLevelFatal), Core.LogTypeFatal, -1, channel, file, funcName, line, data)
}

// HexMainCh writes a hex dump of data to the channel on the main log.
func HexMainCh(channel string, data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLogCh(int(Core.LogLevelInfo), Core.LogTypeMain, -1, channel, file, funcName, line, data)
}

// HexRemoteCh writes a hex dump of data to the channel on the remote log.
func HexRemoteCh(channel string, data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLogCh(int(Core.LogLevelRemote), Core.LogTypeRemote, -1, channel, file, funcName, line, data)
}

// HexSysCh writes a hex dump of data to the channel on the system log.
func HexSysCh(channel string, data []byte) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteHexLogCh(int(Core.LogLevelSys), Core.LogTypeSys, -1, channel, file, funcName, line, data)
}

// =============================================================================
// Escape logging — special-character-escaped messages
//
// Usage:
//   gologger.EscapeInfo("value with ]brackets, and commas")
// =============================================================================

// EscapeTrace writes an escape-formatted trace log.
func EscapeTrace(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmt(int(Core.LogLevelTrace), Core.LogTypeTrace, -1, file, funcName, line, format, args...)
}

// EscapeDebug writes an escape-formatted debug log.
func EscapeDebug(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmt(int(Core.LogLevelDebug), Core.LogTypeDebug, -1, file, funcName, line, format, args...)
}

// EscapeInfo writes an escape-formatted info log.
func EscapeInfo(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, file, funcName, line, format, args...)
}

// EscapeWarn writes an escape-formatted warn log.
func EscapeWarn(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmt(int(Core.LogLevelWarn), Core.LogTypeWarn, -1, file, funcName, line, format, args...)
}

// EscapeError writes an escape-formatted error log.
func EscapeError(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmt(int(Core.LogLevelError), Core.LogTypeError, -1, file, funcName, line, format, args...)
}

// EscapeFatal writes an escape-formatted fatal log.
func EscapeFatal(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmt(int(Core.LogLevelFatal), Core.LogTypeFatal, -1, file, funcName, line, format, args...)
}

// EscapeMain writes an escape-formatted log message to the main application log.
func EscapeMain(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmt(int(Core.LogLevelInfo), Core.LogTypeMain, -1, file, funcName, line, format, args...)
}

// EscapeRemote writes an escape-formatted log message to the remote log.
func EscapeRemote(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmt(int(Core.LogLevelRemote), Core.LogTypeRemote, -1, file, funcName, line, format, args...)
}

// EscapeSys writes an escape-formatted log message to the system event log.
func EscapeSys(format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmt(int(Core.LogLevelSys), Core.LogTypeSys, -1, file, funcName, line, format, args...)
}

// =============================================================================
// Escape channel-aware logging
// =============================================================================

// EscapeTraceCh writes an escape-formatted trace log to the channel.
func EscapeTraceCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmtCh(int(Core.LogLevelTrace), Core.LogTypeTrace, -1, channel, file, funcName, line, format, args...)
}

// EscapeDebugCh writes an escape-formatted debug log to the channel.
func EscapeDebugCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmtCh(int(Core.LogLevelDebug), Core.LogTypeDebug, -1, channel, file, funcName, line, format, args...)
}

// EscapeInfoCh writes an escape-formatted info log to the channel.
func EscapeInfoCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmtCh(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, channel, file, funcName, line, format, args...)
}

// EscapeWarnCh writes an escape-formatted warn log to the channel.
func EscapeWarnCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmtCh(int(Core.LogLevelWarn), Core.LogTypeWarn, -1, channel, file, funcName, line, format, args...)
}

// EscapeErrorCh writes an escape-formatted error log to the channel.
func EscapeErrorCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmtCh(int(Core.LogLevelError), Core.LogTypeError, -1, channel, file, funcName, line, format, args...)
}

// EscapeFatalCh writes an escape-formatted fatal log to the channel.
func EscapeFatalCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmtCh(int(Core.LogLevelFatal), Core.LogTypeFatal, -1, channel, file, funcName, line, format, args...)
}

// EscapeMainCh writes an escape-formatted log message to the channel on the main log.
func EscapeMainCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmtCh(int(Core.LogLevelInfo), Core.LogTypeMain, -1, channel, file, funcName, line, format, args...)
}

// EscapeRemoteCh writes an escape-formatted log message to the channel on the remote log.
func EscapeRemoteCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmtCh(int(Core.LogLevelRemote), Core.LogTypeRemote, -1, channel, file, funcName, line, format, args...)
}

// EscapeSysCh writes an escape-formatted log message to the channel on the system log.
func EscapeSysCh(channel, format string, args ...any) {
	file, funcName, line := apiCallerInfo(1)
	Logger.GetInstance().WriteEscapeLogFmtCh(int(Core.LogLevelSys), Core.LogTypeSys, -1, channel, file, funcName, line, format, args...)
}

// =============================================================================
// Crash / exception handling (panic-recover based)
//
// Usage:
//   gologger.InitException("./logs")   // enable crash logging (optional)
//
//   func risky() {
//       defer gologger.Recover()       // capture & log any panic here
//       // ... code that may panic ...
//   }
//
//   gologger.SafeGo(func() { ... })    // run a goroutine with auto crash logging
// =============================================================================

// InitException initializes the crash/exception log file at <logPath>/Exception.log.
// Panics captured via Recover / RecoverGoroutine / SafeGo are written there with
// a full stack trace. Returns true on success.
func InitException(logPath string) bool {
	return Common.GetCYExceptionLogFileInstance().InitLog(logPath)
}

// SetPanicHandler registers a callback invoked whenever a panic is captured.
func SetPanicHandler(h func(recovered any, stack string)) {
	Common.GetCYExceptionLogFileInstance().SetPanicHandler(Common.PanicHandler(h))
}

// SetPanicRethrow controls whether Recover re-panics after logging (default false).
func SetPanicRethrow(b bool) {
	Common.GetCYExceptionLogFileInstance().SetRethrow(b)
}

// Recover is a defer helper that captures a panic in the current goroutine and
// writes it (with stack trace) to the exception log. It MUST be used as:
//
//	defer gologger.Recover()
//
// (recover() is called directly inside this deferred function, as Go requires.)
func Recover() {
	if r := recover(); r != nil {
		Common.ReportPanic(r)
	}
}

// SafeGo runs fn in a new goroutine whose panics are automatically captured and
// logged to the exception log, preventing silent crashes of background workers.
func SafeGo(fn func()) {
	Common.SafeGo(fn)
}

// =============================================================================
// Log-file upload (FTP) — archive-then-upload helper
//
// Usage:
//   cfg := &gologger.UpLoadConfig{Host: "127.0.0.1", Port: 21,
//       User: "u", Password: "p", RemoteDir: "/logs", Passive: true}
//   gologger.UploadLogFTP(cfg, "./logs/Info.log", "")
// =============================================================================

// UpLoadConfig is the connection configuration for log uploads.
type UpLoadConfig = UpLoad.CYUpLoadConfig

// UploadLogFTP uploads a single local log file to an FTP server. If remotePath
// is empty, the base name of localPath under cfg.RemoteDir is used. This is a
// one-shot helper (connect, upload, disconnect).
func UploadLogFTP(cfg *UpLoadConfig, localPath, remotePath string) error {
	return UpLoad.GetCYUpLoadFactoryInstance().UploadFile(UpLoad.UpLoadTypeFTP, cfg, localPath, remotePath)
}

// ZipAndUploadFTP compresses src into a temporary zip and uploads it via FTP.
// remotePath, if empty, defaults to the zip base name under cfg.RemoteDir.
func ZipAndUploadFTP(cfg *UpLoadConfig, src, zipPath, remotePath string) error {
	if !ZipLog(src, zipPath) {
		return fmt.Errorf("zip failed for %s", src)
	}
	return UploadLogFTP(cfg, zipPath, remotePath)
}

// =============================================================================
// Encryption helpers — build a log-content encryptor
//
// Usage:
//   enc, _ := gologger.NewAESEncryptor([]byte("my-secret-key"))
//   ciph, _ := enc.Encrypt([]byte("sensitive log line"))
// =============================================================================

// Encryptor is the pluggable encryption interface.
type Encryptor = Encryption.IEncryption

// NewAESEncryptor creates and initializes an AES-256-GCM encryptor from key material.
func NewAESEncryptor(key []byte) (Encryptor, error) {
	enc, err := Encryption.GetCYEncryptionFactoryInstance().CreateEncryption(Encryption.EncryptionTypeAESGCM)
	if err != nil {
		return nil, err
	}
	if err := enc.Init(key); err != nil {
		return nil, err
	}
	return enc, nil
}
