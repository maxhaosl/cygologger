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

// Package Core provides the foundational type definitions for the logging library.
package Core

// ELogType represents the type of log message.
type ELogType int

const (
	LogTypeNone    ELogType = 0
	LogTypeTrace   ELogType = 1
	LogTypeDebug   ELogType = 2
	LogTypeInfo    ELogType = 3
	LogTypeWarn    ELogType = 4
	LogTypeError   ELogType = 5
	LogTypeFatal   ELogType = 6
	LogTypeMain    ELogType = 7
	LogTypeRemote  ELogType = 8
	LogTypeSys     ELogType = 9
	LogTypeConsole ELogType = 10
	LogTypeMax     ELogType = 11
)

// ELogLevel represents the output destination level (bitmask).
type ELogLevel int

const (
	LogLevelConsole ELogLevel = 1
	LogLevelTrace   ELogLevel = 2
	LogLevelDebug   ELogLevel = 4
	LogLevelInfo    ELogLevel = 8
	LogLevelWarn    ELogLevel = 16
	LogLevelError   ELogLevel = 32
	LogLevelFatal   ELogLevel = 64
	LogLevelRemote  ELogLevel = 128
	LogLevelSys     ELogLevel = 256
)

// ELogLevelFilter controls which log levels are written (bitmask).
type ELogLevelFilter int

const (
	LogFilterAll             = ELogLevelFilter(LogLevelConsole | LogLevelTrace | LogLevelDebug | LogLevelInfo | LogLevelWarn | LogLevelError | LogLevelFatal)
	LogFilterWarnsAndErrors = ELogLevelFilter(LogLevelInfo | LogLevelWarn | LogLevelError | LogLevelFatal)
	LogFilterErrors         = ELogLevelFilter(LogLevelError | LogLevelFatal)
	LogFilterNone           ELogLevelFilter = 0
)

// ELogFileMode controls how log files are named.
type ELogFileMode int

const (
	LogFileModeAppend ELogFileMode = 0
	LogFileModeTime   ELogFileMode = 1
)

// ELogLayoutType selects the built-in layout template.
type ELogLayoutType int

const (
	LogLayoutTypeCustom   ELogLayoutType = 0
	LogLayoutTypeBuildin1 ELogLayoutType = 1
	LogLayoutTypeBuildin2 ELogLayoutType = 2
	LogLayoutTypeBuildin3 ELogLayoutType = 3
)

// STStatistics holds logging statistics counters.
type STStatistics struct {
	NTotalLine          uint64
	NTotalByte          uint64
	FTotalCurrentFps   float64
	FTotalAverageFps   float64
	NTotalPublicQueue   uint32
	NTotalPrivateQueue  uint32

	NConsoleLine uint64
	NConsoleByte uint64
	FConsoleCurrentFps float64
	FConsoleAverageFps float64

	NTraceLine uint64
	NTraceByte uint64
	FTraceCurrentFps float64
	FTraceAverageFps float64

	NDebugLine uint64
	NDebugByte uint64
	FDebugCurrentFps float64
	FDebugAverageFps float64

	NInfoLine uint64
	NInfoByte uint64
	FInfoCurrentFps float64
	FInfoAverageFps float64

	NWarnLine uint64
	NWarnByte uint64
	FWarnCurrentFps float64
	FWarnAverageFps float64

	NErrorLine uint64
	NErrorByte uint64
	FErrorCurrentFps float64
	FErrorAverageFps float64

	NFatalLine uint64
	NFatalByte uint64
	FFatalCurrentFps float64
	FFatalAverageFps float64

	NMainLine uint64
	NMainByte uint64
	FMainCurrentFps float64
	FMainAverageFps float64

	NRemoteLine uint64
	NRemoteByte uint64
	FRemoteCurrentFps float64
	FRemoteAverageFps float64

	NSysLine uint64
	NSysByte uint64
	FSysCurrentFps float64
	FSysAverageFps float64
}

// Default configuration constants.
const (
	DefaultLogShowConsoleWindow = false
	DefaultLogWriteRemote      = false
	DefaultLogWriteSys         = false
	DefaultLogFileMode         = LogFileModeTime
	DefaultLogLayoutType       = LogLayoutTypeBuildin1
)

const (
	DefaultLogLimitEnable          = true
	DefaultLogLimitClearUnLogFile = true
	DefaultLogTimeClearLog        = 60
	DefaultLogTimeExpiredFile      = 24
	DefaultLogCheckFileSizeTime    = 60 * 5
	DefaultLogCheckFileCountTime   = 60
	DefaultLogCheckFileSize        = 1024 * 1024 * 5
	DefaultLogCountPerType        = 20
	DefaultLogCheckFileTypeSize   = 1024 * 1024 * 500
	DefaultLogCheckAllFileSize     = 1024 * 1024 * 1024
)

const DefaultLogLevelFilter = ELogLevelFilter(LogLevelConsole | LogLevelTrace | LogLevelDebug | LogLevelInfo | LogLevelWarn | LogLevelError | LogLevelFatal)
