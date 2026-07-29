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

// Package ICYLogger is the single import entry point for the cygologger library.
// All public types, constants, and functions are re-exported from sub-packages,
// allowing users to import everything with a single import:
//
//	import gologger "github.com/maxhaosl/cygologger/ICYLogger"
//
// Example usage (new Go-idiomatic API with auto caller info):
//
//	gologger.InitDefault("./logs")
//	defer gologger.Scope()()
//	gologger.Info("Hello, %s!", "world")
//	gologger.Debug("value = %d", val)
//	gologger.FlushLogger()
//	gologger.UnInitLogger()
//
// Legacy C-style API:
//
//	gologger.InitLogger("./Log", true)
//	gologger.LOG_INFO("Hello, %s!", "world")
package ICYLogger

import (
	Common "github.com/maxhaosl/cygologger/ICYLogger/Common"
	Core "github.com/maxhaosl/cygologger/ICYLogger/Core"
	Encryption "github.com/maxhaosl/cygologger/ICYLogger/Encryption"
	Entity "github.com/maxhaosl/cygologger/ICYLogger/Entity"
	Appender "github.com/maxhaosl/cygologger/ICYLogger/Appender"
	Filter "github.com/maxhaosl/cygologger/ICYLogger/Filter"
	Layout "github.com/maxhaosl/cygologger/ICYLogger/Layout"
	Logger "github.com/maxhaosl/cygologger/ICYLogger/Logger"
	UpLoad "github.com/maxhaosl/cygologger/ICYLogger/UpLoad"
)

// =============================================================================
// Core types and constants (github.com/maxhaosl/cygologger/ICYLogger/Core)
// =============================================================================

type (
	ELogType         = Core.ELogType
	ELogLevel        = Core.ELogLevel
	ELogLevelFilter  = Core.ELogLevelFilter
	ELogFileMode     = Core.ELogFileMode
	ELogLayoutType   = Core.ELogLayoutType
	STStatistics     = Core.STStatistics
	CYLoggerConfig   = Core.CYLoggerConfig
)

const (
	LogTypeNone    = Core.LogTypeNone
	LogTypeTrace   = Core.LogTypeTrace
	LogTypeDebug   = Core.LogTypeDebug
	LogTypeInfo    = Core.LogTypeInfo
	LogTypeWarn    = Core.LogTypeWarn
	LogTypeError   = Core.LogTypeError
	LogTypeFatal   = Core.LogTypeFatal
	LogTypeMain    = Core.LogTypeMain
	LogTypeRemote  = Core.LogTypeRemote
	LogTypeSys     = Core.LogTypeSys
	LogTypeConsole = Core.LogTypeConsole
	LogTypeMax     = Core.LogTypeMax

	LogLevelConsole = Core.LogLevelConsole
	LogLevelTrace   = Core.LogLevelTrace
	LogLevelDebug   = Core.LogLevelDebug
	LogLevelInfo    = Core.LogLevelInfo
	LogLevelWarn    = Core.LogLevelWarn
	LogLevelError   = Core.LogLevelError
	LogLevelFatal   = Core.LogLevelFatal
	LogLevelRemote  = Core.LogLevelRemote
	LogLevelSys     = Core.LogLevelSys

	LogFileModeAppend = Core.LogFileModeAppend
	LogFileModeTime   = Core.LogFileModeTime

	LogLayoutTypeCustom   = Core.LogLayoutTypeCustom
	LogLayoutTypeBuildin1 = Core.LogLayoutTypeBuildin1
	LogLayoutTypeBuildin2 = Core.LogLayoutTypeBuildin2
	LogLayoutTypeBuildin3 = Core.LogLayoutTypeBuildin3
	LogLayoutTypeBuildin4 = Core.LogLayoutTypeBuildin4
	LogLayoutTypeBuildin5 = Core.LogLayoutTypeBuildin5
	LogLayoutTypeBuildin6 = Core.LogLayoutTypeBuildin6

	LogFilterAll             = Core.LogFilterAll
	LogFilterWarnsAndErrors = Core.LogFilterWarnsAndErrors
	LogFilterErrors         = Core.LogFilterErrors
	LogFilterNone           = Core.LogFilterNone

	DefaultLogShowConsoleWindow = Core.DefaultLogShowConsoleWindow
	DefaultLogWriteRemote      = Core.DefaultLogWriteRemote
	DefaultLogWriteSys         = Core.DefaultLogWriteSys
	DefaultLogFileMode         = Core.LogFileModeTime
	DefaultLogLayoutType       = Core.LogLayoutTypeBuildin1
	DefaultLogLevelFilter      = Core.DefaultLogLevelFilter

	DefaultLogLimitEnable          = Core.DefaultLogLimitEnable
	DefaultLogLimitClearUnLogFile = Core.DefaultLogLimitClearUnLogFile
	DefaultLogTimeClearLog        = Core.DefaultLogTimeClearLog
	DefaultLogTimeExpiredFile      = Core.DefaultLogTimeExpiredFile
	DefaultLogCheckFileSizeTime    = Core.DefaultLogCheckFileSizeTime
	DefaultLogCheckFileCountTime   = Core.DefaultLogCheckFileCountTime
	DefaultLogCheckFileSize        = Core.DefaultLogCheckFileSize
	DefaultLogCountPerType        = Core.DefaultLogCountPerType
	DefaultLogCheckFileTypeSize   = Core.DefaultLogCheckFileTypeSize
	DefaultLogCheckAllFileSize     = Core.DefaultLogCheckAllFileSize
)

// =============================================================================
// Layout interfaces (github.com/maxhaosl/cygologger/ICYLogger/Layout)
// =============================================================================

type (
	ICYLogger              = Layout.ICYLogger
	ICYLoggerTemplateLayout = Layout.ICYLoggerTemplateLayout
)

// =============================================================================
// Filter types (github.com/maxhaosl/cygologger/ICYLogger/Filter)
// =============================================================================

type (
	TupleFieldType          = Filter.TupleFieldType
	ICYLoggerPatternFilter  = Filter.ICYLoggerPatternFilter
)

const (
	LogEscape                 = Filter.LogEscape
	LogHeaderStart            = Filter.LogHeaderStart
	LogHeaderEnd              = Filter.LogHeaderEnd
	LogFieldNameEnd           = Filter.LogFieldNameEnd
	LogFieldValueEnd          = Filter.LogFieldValueEnd
	LogExtensionFieldValueEnd = Filter.LogExtensionFieldValueEnd
)

func NewPatternFilter() *Filter.ICYLoggerPatternFilter {
	return Filter.NewPatternFilter()
}

// =============================================================================
// Logger convenience functions (github.com/maxhaosl/cygologger/ICYLogger/Logger)
// =============================================================================

func InitLogger(szLogPath string, bShowConsoleWindow bool) bool {
	return Logger.InitLogger(szLogPath, bShowConsoleWindow)
}
func UnInitLogger()                     { Logger.UnInitLogger() }
func FreeInstance()                     { Logger.FreeInstance() }
func GetInstance() *Logger.CYLoggerImpl { return Logger.GetInstance() }
func FlushLogger()                     { Logger.FlushLogger() }
func FlushLoggerType(eType Core.ELogType) { Logger.FlushLoggerType(eType) }
func LOG_TRACE(szMsg string, args ...any)  { Logger.LOG_TRACE(szMsg, args...) }
func LOG_DEBUG(szMsg string, args ...any)  { Logger.LOG_DEBUG(szMsg, args...) }
func LOG_INFO(szMsg string, args ...any)   { Logger.LOG_INFO(szMsg, args...) }
func LOG_WARN(szMsg string, args ...any)   { Logger.LOG_WARN(szMsg, args...) }
func LOG_ERROR(szMsg string, args ...any)  { Logger.LOG_ERROR(szMsg, args...) }
func LOG_FATAL(szMsg string, args ...any)  { Logger.LOG_FATAL(szMsg, args...) }
func LOG_MAIN(szMsg string, args ...any)   { Logger.LOG_MAIN(szMsg, args...) }
func LOG_SYS(szMsg string, args ...any)    { Logger.LOG_SYS(szMsg, args...) }
func LOG_REMOTE(szMsg string, args ...any) { Logger.LOG_REMOTE(szMsg, args...) }

// ---- Channel-aware logging convenience functions ----

func LOG_TRACE_CH(szChannel, szMsg string, args ...any)   { Logger.LOG_TRACE_CH(szChannel, szMsg, args...) }
func LOG_DEBUG_CH(szChannel, szMsg string, args ...any)   { Logger.LOG_DEBUG_CH(szChannel, szMsg, args...) }
func LOG_INFO_CH(szChannel, szMsg string, args ...any)    { Logger.LOG_INFO_CH(szChannel, szMsg, args...) }
func LOG_WARN_CH(szChannel, szMsg string, args ...any)    { Logger.LOG_WARN_CH(szChannel, szMsg, args...) }
func LOG_ERROR_CH(szChannel, szMsg string, args ...any)   { Logger.LOG_ERROR_CH(szChannel, szMsg, args...) }
func LOG_FATAL_CH(szChannel, szMsg string, args ...any)   { Logger.LOG_FATAL_CH(szChannel, szMsg, args...) }
func LOG_MAIN_CH(szChannel, szMsg string, args ...any)    { Logger.LOG_MAIN_CH(szChannel, szMsg, args...) }
func LOG_SYS_CH(szChannel, szMsg string, args ...any)     { Logger.LOG_SYS_CH(szChannel, szMsg, args...) }
func LOG_REMOTE_CH(szChannel, szMsg string, args ...any)  { Logger.LOG_REMOTE_CH(szChannel, szMsg, args...) }

// ---- Channel-aware direct logging convenience functions (bypass level filter) ----

func LOG_DIRECT_TRACE_CH(szChannel, szMsg string, args ...any) { Logger.LOG_DIRECT_TRACE_CH(szChannel, szMsg, args...) }
func LOG_DIRECT_DEBUG_CH(szChannel, szMsg string, args ...any) { Logger.LOG_DIRECT_DEBUG_CH(szChannel, szMsg, args...) }
func LOG_DIRECT_INFO_CH(szChannel, szMsg string, args ...any)  { Logger.LOG_DIRECT_INFO_CH(szChannel, szMsg, args...) }
func LOG_DIRECT_WARN_CH(szChannel, szMsg string, args ...any)  { Logger.LOG_DIRECT_WARN_CH(szChannel, szMsg, args...) }
func LOG_DIRECT_ERROR_CH(szChannel, szMsg string, args ...any) { Logger.LOG_DIRECT_ERROR_CH(szChannel, szMsg, args...) }
func LOG_DIRECT_FATAL_CH(szChannel, szMsg string, args ...any) { Logger.LOG_DIRECT_FATAL_CH(szChannel, szMsg, args...) }
func LOG_DIRECT_MAIN_CH(szChannel, szMsg string, args ...any)  { Logger.LOG_DIRECT_MAIN_CH(szChannel, szMsg, args...) }

// The idiomatic channel-aware helpers (TraceCh/DebugCh/.../SysCh, their Direct*
// and Hex*/Escape* variants) are defined in api.go and exported directly from
// the ICYLogger package, so they are not re-declared here.


// =============================================================================
// Entity / file management helpers (github.com/maxhaosl/cygologger/ICYLogger/Entity)
// =============================================================================

// ForceNewFile forces every log entity to rotate to a fresh file (C++ ForceEntityNewFile).
func ForceNewFile() { Logger.GetInstance().ForceNewFile() }

// GetLoggerEntity returns the entity registered for eLogType (C++ GetLoggerEntity).
func GetLoggerEntity(eLogType Core.ELogType) *Entity.CYLoggerEntity {
	return Logger.GetInstance().GetLoggerEntity(eLogType)
}

// ReleaseLoggerEntity flushes, detaches and removes the entity for eLogType (C++ ReleaseLoggerEntity).
func ReleaseLoggerEntity(eLogType Core.ELogType) { Logger.GetInstance().ReleaseLoggerEntity(eLogType) }

// ResetLogFile forces every file appender to rotate to a fresh file (C++ ResetLogFile).
func ResetLogFile() { Logger.GetInstance().ResetLogFile() }

// AddLogType records an extra log type tracked by the schedule (C++ AddLogType).
func AddLogType(eLogType Core.ELogType) { Logger.GetInstance().AddLogType(eLogType) }

// ClearConsole clears the console screen via the active console appender
// (mirrors C++ ClearConsole).
func ClearConsole() {
	if a := Appender.GetConsoleAppender(); a != nil {
		a.ClearConsole()
	}
}

// =============================================================================
// Layout functions (github.com/maxhaosl/cygologger/ICYLogger/Layout)
// =============================================================================

func NewCYLoggerTemplateLayoutCustom(inner Layout.ICYLoggerTemplateLayout) *Layout.CYLoggerTemplateLayoutCustom {
	return Layout.NewCYLoggerTemplateLayoutCustom(inner)
}

func NewCYLoggerTemplateLayout6() *Layout.CYLoggerTemplateLayout6 {
	return Layout.NewCYLoggerTemplateLayout6()
}
func GetCYLoggerTemplateLayoutManagerInstance() *Layout.CYLoggerTemplateLayoutManager {
	return Layout.GetCYLoggerTemplateLayoutManagerInstance()
}

// =============================================================================
// Filter functions (github.com/maxhaosl/cygologger/ICYLogger/Filter)
// =============================================================================

func GetCYLoggerPatternFilterChainInstance() *Filter.CYLoggerPatternFilterChain {
	return Filter.GetCYLoggerPatternFilterChainInstance()
}
func GetCYLoggerPatternFilterManagerInstance() *Filter.CYLoggerPatternFilterManager {
	return Filter.GetCYLoggerPatternFilterManagerInstance()
}

// =============================================================================
// Config functions (re-exported from Core for unified access)
// =============================================================================

func GetCYLoggerConfigInstance() *Core.CYLoggerConfig {
	return Core.GetCYLoggerConfigInstance()
}

// =============================================================================
// SimpleLog / Exception types (github.com/maxhaosl/cygologger/ICYLogger/Common)
// =============================================================================

type (
	ESimpleLogType     = Common.ESimpleLogType
	CYSimpleLogFile    = Common.CYSimpleLogFile
	CYSimpleLogConsole = Common.CYSimpleLogConsole
	ISimpleLog         = Common.ISimpleLog
	PanicHandler       = Common.PanicHandler
)

const (
	SimpleLogTypeNone    = Common.SimpleLogTypeNone
	SimpleLogTypeFile    = Common.SimpleLogTypeFile
	SimpleLogTypeConsole = Common.SimpleLogTypeConsole
	SimpleLogTypeAll     = Common.SimpleLogTypeAll
)

func NewCYSimpleLogFile() *Common.CYSimpleLogFile       { return Common.NewCYSimpleLogFile() }
func NewCYSimpleLogConsole() *Common.CYSimpleLogConsole { return Common.NewCYSimpleLogConsole() }
func GetCYExceptionLogFileInstance() *Common.CYExceptionLogFile {
	return Common.GetCYExceptionLogFileInstance()
}

// =============================================================================
// UpLoad types (github.com/maxhaosl/cygologger/ICYLogger/UpLoad)
// =============================================================================

type (
	EUpLoadType = UpLoad.EUpLoadType
	IUpLoad     = UpLoad.IUpLoad
)

const (
	UpLoadTypeNone = UpLoad.UpLoadTypeNone
	UpLoadTypeFTP  = UpLoad.UpLoadTypeFTP
)

func GetCYUpLoadFactoryInstance() *UpLoad.CYUpLoadFactory {
	return UpLoad.GetCYUpLoadFactoryInstance()
}

// =============================================================================
// Encryption types (github.com/maxhaosl/cygologger/ICYLogger/Encryption)
// =============================================================================

type (
	EEncryptionType = Encryption.EEncryptionType
	IEncryption     = Encryption.IEncryption
)

const (
	EncryptionTypeNone   = Encryption.EncryptionTypeNone
	EncryptionTypeAESGCM = Encryption.EncryptionTypeAESGCM
)

func GetCYEncryptionFactoryInstance() *Encryption.CYEncryptionFactory {
	return Encryption.GetCYEncryptionFactoryInstance()
}
