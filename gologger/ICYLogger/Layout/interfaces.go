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

// Package Layout provides the logging layout interfaces.
package Layout

import "github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
import "github.com/maxhaosl/CYGoLogger/ICYLogger/Filter"

// ICYLogger is the main public interface for the logging library.
type ICYLogger interface {
	Init() bool
	UnInit()
	Flush(eLogType Core.ELogType)
	AddAppender(eLogType Core.ELogType, szChannel, szFile string, eFileMode Core.ELogFileMode) bool
	WriteLog(nLogLevel int, eMsgType Core.ELogType, nSeverCode int, szMsg string)
	WriteLogFmt(nLogLevel int, eMsgType Core.ELogType, nSeverCode int, pszFile, pszFuncName string, nLine int, szMsg string, args ...any)
	WriteEscapeLogFmt(nLogLevel int, eMsgType Core.ELogType, nSeverCode int, pszFile, pszFuncName string, nLine int, szMsg string, args ...any)
	WriteHexLog(nLogLevel int, eMsgType Core.ELogType, nSeverCode int, pszFile, pszFuncName string, nLine int, data []byte)
	SetConfig(szLogPath string, bShowConsoleWindow bool)
	SetRestriction(bEnableCheck, bClearUnLogFile bool, nLimitTimeClearLog, nLimitTimeExpiredFile, nCheckFileSizeTime, nCheckFileCountTime, nCheckFileSize, nFileCountPerType, nCheckFileTypeSize, nCheckAllFileSize int)
	SetFilter(pFilter *Filter.ICYLoggerPatternFilter)
	SetLayout(eLayoutType Core.ELogLayoutType, pLayout ICYLoggerTemplateLayout)
	GetStats(pStats *Core.STStatistics) bool
	GetLogLevel() Core.ELogLevelFilter
	SetLogLevel(eLogLevelFilter Core.ELogLevelFilter)
}

// ICYLoggerTemplateLayout formats log messages into strings.
type ICYLoggerTemplateLayout interface {
	GetFormatMessage(strChannel string, eMsgType Core.ELogType, nSeverCode int,
		strMsg, strFile, strFunction string,
		nLine int, nProcessId, nThreadId uint64,
		nYY, nMM, nDD, nHR, nMN, nSC, nMMN int,
		bEscape bool) string
	GetTypeIndex() int32
	GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string
}
