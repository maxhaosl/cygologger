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

// Package ICYLogger is the single import entry point for the CYGoLogger library.
// All public types, constants, and functions are re-exported from sub-packages,
// allowing users to import everything with a single import:
//
//	import gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"
//
// Example usage:
//
//	gologger.InitLogger("./Log", true)
//	gologger.LOG_INFO("Hello, %s!", "world")
//	gologger.FlushLogger()
//	gologger.UnInitLogger()
package ICYLogger

import (
	Core "github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
	Filter "github.com/maxhaosl/CYGoLogger/ICYLogger/Filter"
	Layout "github.com/maxhaosl/CYGoLogger/ICYLogger/Layout"
	Logger "github.com/maxhaosl/CYGoLogger/ICYLogger/Logger"
)

// =============================================================================
// Core types and constants (github.com/maxhaosl/CYGoLogger/ICYLogger/Core)
// =============================================================================

type (
	ELogType         = Core.ELogType
	ELogLevel        = Core.ELogLevel
	ELogLevelFilter  = Core.ELogLevelFilter
	ELogFileMode     = Core.ELogFileMode
	ELogLayoutType   = Core.ELogLayoutType
	STStatistics     = Core.STStatistics
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
// Layout interfaces (github.com/maxhaosl/CYGoLogger/ICYLogger/Layout)
// =============================================================================

type (
	ICYLogger              = Layout.ICYLogger
	ICYLoggerTemplateLayout = Layout.ICYLoggerTemplateLayout
)

// =============================================================================
// Filter types (github.com/maxhaosl/CYGoLogger/ICYLogger/Filter)
// =============================================================================

type (
	TupleFieldType          = Filter.TupleFieldType
	ICYLoggerPatternFilter  = Filter.ICYLoggerPatternFilter
)

const (
	LogEscape       = Filter.LogEscape
	LogFieldNameEnd = Filter.LogFieldNameEnd
	LogFieldValueEnd = Filter.LogFieldValueEnd
)

func NewPatternFilter() *Filter.ICYLoggerPatternFilter {
	return Filter.NewPatternFilter()
}

// =============================================================================
// Logger convenience functions (github.com/maxhaosl/CYGoLogger/ICYLogger/Logger)
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

// =============================================================================
// Layout functions (github.com/maxhaosl/CYGoLogger/ICYLogger/Layout)
// =============================================================================

func NewCYLoggerTemplateLayoutCustom(inner Layout.ICYLoggerTemplateLayout) *Layout.CYLoggerTemplateLayoutCustom {
	return Layout.NewCYLoggerTemplateLayoutCustom(inner)
}
func GetCYLoggerTemplateLayoutManagerInstance() *Layout.CYLoggerTemplateLayoutManager {
	return Layout.GetCYLoggerTemplateLayoutManagerInstance()
}

// =============================================================================
// Filter functions (github.com/maxhaosl/CYGoLogger/ICYLogger/Filter)
// =============================================================================

func GetCYLoggerPatternFilterChainInstance() *Filter.CYLoggerPatternFilterChain {
	return Filter.GetCYLoggerPatternFilterChainInstance()
}
func GetCYLoggerPatternFilterManagerInstance() *Filter.CYLoggerPatternFilterManager {
	return Filter.GetCYLoggerPatternFilterManagerInstance()
}
