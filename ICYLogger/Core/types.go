/*
 * cygologger License
 * -----------
 *
 * cygologger is licensed under the terms of the MIT license reproduced below.
 * This means that cygologger is free software and can be used for both academic
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
	LogFilterAll                            = ELogLevelFilter(LogLevelConsole | LogLevelTrace | LogLevelDebug | LogLevelInfo | LogLevelWarn | LogLevelError | LogLevelFatal)
	LogFilterWarnsAndErrors                 = ELogLevelFilter(LogLevelInfo | LogLevelWarn | LogLevelError | LogLevelFatal)
	LogFilterErrors                         = ELogLevelFilter(LogLevelError | LogLevelFatal)
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
	LogLayoutTypeBuildin4 ELogLayoutType = 4 // [Time][TYPE|P:pid|T:tid][func(line)] Msg
	LogLayoutTypeBuildin5 ELogLayoutType = 5 // [Time][TYPE|P:pid|T:tid][CH:channel][func(line)] Msg
	// LogLayoutTypeBuildin6 is the retrieval-preferred layout: like Buildin4 but
	// the channel is rendered as a bare [name] bracket placed BEFORE [func(line)]
	// (Buildin4 puts [func(line)] before the channel). Output:
	//   [Time][TYPE|P:pid|T:tid][channel][func(line)] Msg
	LogLayoutTypeBuildin6 ELogLayoutType = 6
)

// STStatistics holds logging statistics counters (fully aligned with C++ CYLogger).
type STStatistics struct {
	NTotalLine         uint64  // The total number of logs written.
	NTotalByte         uint64  // Total log bytes written.
	FTotalCurrentFps   float64 // The current total fps written to the log.
	FTotalAverageFps   float64 // The average total fps written to the log.
	NTotalPublicQueue  uint32  // Total public queue length.
	NTotalPrivateQueue uint32  // Total private queue length.

	NConsoleLine         uint64  // The total number of lines written to the console log.
	NConsoleByte         uint64  // Total bytes written to the console log.
	FConsoleCurrentFps   float64 // The current fps written to the console log.
	FConsoleAverageFps   float64 // The average fps written to the console log.
	NConsolePublicDQueue uint32  // The console public debug queue length.
	NConsolePublicTQueue uint32  // The console public trace queue length.
	NConsolePublicIQueue uint32  // The console public info queue length.
	NConsolePublicWQueue uint32  // The console public warn queue length.
	NConsolePublicEQueue uint32  // The console public error queue length.
	NConsolePublicFQueue uint32  // The console public fatal queue length.
	NConsolePrivateQueue uint32  // The console private queue length.

	NTraceLine         uint64  // The total number of lines written to the trace log.
	NTraceByte         uint64  // Total bytes written to the trace log.
	FTraceCurrentFps   float64 // The current fps written to the trace log.
	FTraceAverageFps   float64 // The average fps written to the trace log.
	NTracePublicQueue  uint32  // The trace public queue length.
	NTracePrivateQueue uint32  // The trace private queue length.

	NDebugLine         uint64
	NDebugByte         uint64
	FDebugCurrentFps   float64
	FDebugAverageFps   float64
	NDebugPublicQueue  uint32
	NDebugPrivateQueue uint32

	NInfoLine         uint64
	NInfoByte         uint64
	FInfoCurrentFps   float64
	FInfoAverageFps   float64
	NInfoPublicQueue  uint32
	NInfoPrivateQueue uint32

	NWarnLine         uint64
	NWarnByte         uint64
	FWarnCurrentFps   float64
	FWarnAverageFps   float64
	NWarnPublicQueue  uint32
	NWarnPrivateQueue uint32

	NErrorLine         uint64
	NErrorByte         uint64
	FErrorCurrentFps   float64
	FErrorAverageFps   float64
	NErrorPublicQueue  uint32
	NErrorPrivateQueue uint32

	NFatalLine         uint64
	NFatalByte         uint64
	FFatalCurrentFps   float64
	FFatalAverageFps   float64
	NFatalPublicQueue  uint32
	NFatalPrivateQueue uint32

	NMainLine         uint64
	NMainByte         uint64
	FMainCurrentFps   float64
	FMainAverageFps   float64
	NMainPublicDQueue uint32
	NMainPublicTQueue uint32
	NMainPublicIQueue uint32
	NMainPublicWQueue uint32
	NMainPublicEQueue uint32
	NMainPublicFQueue uint32
	NMainPrivateQueue uint32

	NRemoteLine         uint64
	NRemoteByte         uint64
	FRemoteCurrentFps   float64
	FRemoteAverageFps   float64
	NRemotePublicDQueue uint32
	NRemotePublicTQueue uint32
	NRemotePublicIQueue uint32
	NRemotePublicWQueue uint32
	NRemotePublicEQueue uint32
	NRemotePublicFQueue uint32
	NRemotePrivateQueue uint32

	NSysLine         uint64
	NSysByte         uint64
	FSysCurrentFps   float64
	FSysAverageFps   float64
	NSysPublicQueue  uint32
	NSysPrivateQueue uint32
}

// Default configuration constants.
const (
	DefaultLogShowConsoleWindow = false
	DefaultLogWriteRemote       = false
	DefaultLogWriteSys          = false
	DefaultLogFileMode          = LogFileModeTime
	DefaultLogLayoutType        = LogLayoutTypeBuildin1
)

const (
	DefaultLogLimitEnable         = true
	DefaultLogLimitClearUnLogFile = true
	DefaultLogTimeClearLog        = 60
	DefaultLogTimeExpiredFile     = 24
	DefaultLogCheckFileSizeTime   = 60 * 5
	DefaultLogCheckFileCountTime  = 60
	DefaultLogCheckFileSize       = 1024 * 1024 * 5
	DefaultLogCountPerType        = 20
	DefaultLogCheckFileTypeSize   = 1024 * 1024 * 500
	DefaultLogCheckAllFileSize    = 1024 * 1024 * 1024
)

const DefaultLogLevelFilter = ELogLevelFilter(LogLevelConsole | LogLevelTrace | LogLevelDebug | LogLevelInfo | LogLevelWarn | LogLevelError | LogLevelFatal)

// ERemoteProto selects the transport protocol used by the remote appender.
// C++ CYLogger uses UDP (SOCK_DGRAM, 900-byte packets). Go keeps TCP by
// default for reliability, but exposes UDP to align wire-compatibility with C++.
type ERemoteProto int

const (
	RemoteProtoTCP ERemoteProto = iota
	RemoteProtoUDP
)

// ERetCode mirrors the C++ RetCode used by the named condition variable.
type ERetCode int

const (
	RetCodeOK          ERetCode = 0
	RetCodeCondTimeout ERetCode = 1
	RetCodeError       ERetCode = -1
)

// LOG_FPS_CHECK_DURATION is the FPS measurement window (milliseconds) used by
// every appender's CYFPSCounter, mirroring C++ LOG_FPS_CHECK_DURATION (5
// seconds). C++ CYFPSCounter treats this value as a duration in seconds for its
// average-FPS window; the Go CYFPSCounter uses milliseconds for its sampling
// tick, so 5s == 5000ms keeps the statistics semantics aligned.
const LOG_FPS_CHECK_DURATION = 5000
