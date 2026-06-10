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

// Package logger provides the core logger control and implementation.
package logger

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Filter"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Layout"
	Common "github.com/maxhaosl/CYGoLogger/ICYLogger/Common"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Entity"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Appender"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Schedule"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Config"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Statistics"
)

// CYLoggerControl routes log messages to the correct Entity.
type CYLoggerControl struct {
	Common.CYNoCopy
	mu          sync.RWMutex
	eLogLevel   Core.ELogLevelFilter
	szLogPath   string
	entities    map[Core.ELogType]*Entity.CYLoggerEntity
	consoleApp  *Appender.CYLoggerConsoleAppender
	schedule    *Schedule.CYLoggerSchedule
	restriction *Common.CYFileRestriction
	filter      *Filter.ICYLoggerPatternFilter
	layout      Layout.ICYLoggerTemplateLayout
}

var g_CYLoggerControlInstance *CYLoggerControl
var g_CYLoggerControlOnce sync.Once

func GetCYLoggerControlInstance() *CYLoggerControl {
	g_CYLoggerControlOnce.Do(func() {
		factory := Entity.GetCYLoggerEntityFactoryInstance()
		entities := make(map[Core.ELogType]*Entity.CYLoggerEntity)
		for t := Core.LogTypeNone + 1; t < Core.LogTypeMax; t++ {
			entities[t] = factory.GetEntity(t)
		}
		g_CYLoggerControlInstance = &CYLoggerControl{
			eLogLevel:   Core.DefaultLogLevelFilter,
			entities:    entities,
			restriction: Common.NewCYFileRestriction(),
		}
	})
	return g_CYLoggerControlInstance
}

func (c *CYLoggerControl) Init(szLogPath string, bShowConsole, bWriteRemote, bWriteSys bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.szLogPath = szLogPath
	c.filter = Filter.GetCYLoggerPatternFilterManagerInstance().GetFilter()
	c.layout = Layout.GetCYLoggerTemplateLayoutManagerInstance().GetLayout(Core.DefaultLogLayoutType)

	if bShowConsole || c.consoleApp == nil {
		if c.consoleApp != nil {
			c.consoleApp.UnInit()
		}
		c.consoleApp = Appender.GetCYLoggerAppenderFactoryInstance().CreateConsoleAppender(Core.LogTypeConsole, "", "", szLogPath, Core.LogFileModeAppend)
		c.consoleApp.SetLayout(c.layout)
		c.consoleApp.SetFilter(c.filter)
	}

	if c.schedule == nil {
		c.schedule = Schedule.GetCYLoggerScheduleInstance()
		c.schedule.Init(szLogPath, Core.DefaultLogTimeExpiredFile)
	}
	c.schedule.StartSchedule()

	return true
}

func (c *CYLoggerControl) UnInit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.schedule != nil {
		c.schedule.StopSchedule()
	}
	if c.consoleApp != nil {
		c.consoleApp.UnInit()
	}
	Entity.GetCYLoggerEntityFactoryInstance().UnInitAll()
}

func (c *CYLoggerControl) Flush(eLogType Core.ELogType) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if eLogType == Core.LogTypeMax {
		if c.consoleApp != nil {
			c.consoleApp.Flush()
		}
		Entity.GetCYLoggerEntityFactoryInstance().FlushAll()
		return
	}

	if logEntity := c.entities[eLogType]; logEntity != nil {
		logEntity.Flush()
	}
}

func (c *CYLoggerControl) AddAppender(eLogType Core.ELogType, szChannel, szFile string, eFileMode Core.ELogFileMode) bool {
	factory := Appender.GetCYLoggerAppenderFactoryInstance()
	appender_ := factory.CreateAppender(eLogType, szChannel, szFile, c.szLogPath, eFileMode)
	if appender_ == nil {
		return false
	}

	appender_.SetLayout(c.layout)
	appender_.SetFilter(c.filter)
	appender_.SetEnable(true)

	if eLogType == Core.LogTypeConsole {
		if c.consoleApp != nil {
			c.consoleApp.UnInit()
		}
		c.consoleApp = factory.CreateConsoleAppender(eLogType, szChannel, szFile, c.szLogPath, eFileMode)
		c.consoleApp.SetLayout(c.layout)
		c.consoleApp.SetFilter(c.filter)
		c.consoleApp.SetEnable(true)
		return true
	}

	entityItem := c.entities[eLogType]
	if entityItem == nil {
		entityFactory := Entity.GetCYLoggerEntityFactoryInstance()
		entityItem = entityFactory.CreateEntity(eLogType)
		c.mu.Lock()
		c.entities[eLogType] = entityItem
		c.mu.Unlock()
	}

	entityItem.AddAppender(appender_)
	factory.RegisterAppender(appender_)

	if !entityItem.Init() {
		return false
	}

	appender_.Start(appender_.Run)

	return true
}

func (c *CYLoggerControl) Write(msg *Common.CYBaseMessage) {
	if msg == nil {
		return
	}

	eMsgType := Core.ELogType(msg.EMsgType)
	nLogLevel := c.levelForType(eMsgType)

	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.passesFilter(nLogLevel) {
		return
	}

	if c.consoleApp != nil && c.consoleApp.IsEnable() {
		c.consoleApp.Write(msg)
	}

	if logEntity := c.entities[eMsgType]; logEntity != nil {
		logEntity.Write(msg)
	}

	Statistics.GetCYStatisticsInstance().IncrementLine(eMsgType, uint64(len(msg.StrMsg)))
}

func (c *CYLoggerControl) levelForType(eMsgType Core.ELogType) Core.ELogLevel {
	switch eMsgType {
	case Core.LogTypeConsole:
		return Core.LogLevelConsole
	case Core.LogTypeTrace:
		return Core.LogLevelTrace
	case Core.LogTypeDebug:
		return Core.LogLevelDebug
	case Core.LogTypeInfo:
		return Core.LogLevelInfo
	case Core.LogTypeWarn:
		return Core.LogLevelWarn
	case Core.LogTypeError:
		return Core.LogLevelError
	case Core.LogTypeFatal:
		return Core.LogLevelFatal
	case Core.LogTypeRemote:
		return Core.LogLevelRemote
	case Core.LogTypeSys:
		return Core.LogLevelSys
	default:
		return Core.LogLevelConsole
	}
}

func (c *CYLoggerControl) passesFilter(nLogLevel Core.ELogLevel) bool {
	return int(c.eLogLevel)&int(nLogLevel) != 0
}

func (c *CYLoggerControl) SetLogLevel(eFilter Core.ELogLevelFilter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eLogLevel = eFilter
}

func (c *CYLoggerControl) GetLogLevel() Core.ELogLevelFilter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.eLogLevel
}

func (c *CYLoggerControl) SetFilter(pFilter *Filter.ICYLoggerPatternFilter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filter = pFilter
}

func (c *CYLoggerControl) SetLayout(eType Core.ELogLayoutType, pLayout Layout.ICYLoggerTemplateLayout) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.layout = pLayout
}

func (c *CYLoggerControl) SetRestriction(bEnable, bClear bool,
	nTimeClear, nTimeExpired, nSizeTime, nCountTime, nSize, nCount, nTypeSize, nAllSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.restriction != nil {
		c.restriction.SetRestriction(bEnable, bClear, nTimeClear, nTimeExpired, nSizeTime, nCountTime, nSize, nCount, nTypeSize, nAllSize)
	}
}

// CYLoggerImpl is the main logger implementation, exposing the ICYLogger interface.
type CYLoggerImpl struct {
	Common.CYNoCopy
	mu      sync.Mutex
	bInit   atomic.Bool
	bExit   atomic.Bool
	control *CYLoggerControl
	config  *Config.CYLoggerConfig
}

var (
	g_CYLoggerImplInstance *CYLoggerImpl
	g_CYLoggerImplOnce    sync.Once
)

func GetLoggerInstance() *CYLoggerImpl {
	g_CYLoggerImplOnce.Do(func() {
		g_CYLoggerImplInstance = &CYLoggerImpl{}
	})
	return g_CYLoggerImplInstance
}

func (l *CYLoggerImpl) Init() bool {
	if l.bInit.Load() {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.bInit.Load() {
		return true
	}

l.config = Config.GetCYLoggerConfigInstance()
	l.control = GetCYLoggerControlInstance()

	if !l.control.Init(l.config.GetLogPath(), l.config.IsShowConsole(), l.config.IsWriteRemote(), l.config.IsWriteSys()) {
		return false
	}

	l.bInit.Store(true)
	return true
}

func (l *CYLoggerImpl) UnInit() {
	if !l.bInit.Load() {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.bInit.Load() {
		return
	}
	l.bExit.Store(true)
	l.control.UnInit()
	l.bInit.Store(false)
}

func (l *CYLoggerImpl) Flush(eLogType Core.ELogType) {
	if l.control != nil {
		l.control.Flush(eLogType)
	}
}

func (l *CYLoggerImpl) AddAppender(eLogType Core.ELogType, szChannel, szFile string, eFileMode Core.ELogFileMode) bool {
	if l.control != nil {
		return l.control.AddAppender(eLogType, szChannel, szFile, eFileMode)
	}
	return false
}

func (l *CYLoggerImpl) WriteLog(nLogLevel int, eMsgType Core.ELogType, nSeverCode int, szMsg string) {
	if !l.bInit.Load() || l.bExit.Load() {
		return
	}
	if l.control == nil {
		return
	}
	msg := Common.AcquireBaseMessage()
	msg.EMsgType = int(eMsgType)
	msg.NSeverCode = nSeverCode
	msg.StrMsg = szMsg
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = Common.GetCYPublicFunctionInstance().GetCurrentThreadId()
	l.control.Write(msg)
	Common.ReleaseBaseMessage(msg)
}

func (l *CYLoggerImpl) WriteLogFmt(nLogLevel int, eMsgType Core.ELogType, nSeverCode int,
	pszFile, pszFuncName string, nLine int, szMsg string, args ...any) {

	if !l.bInit.Load() || l.bExit.Load() {
		return
	}
	if l.control == nil {
		return
	}
	formatted := szMsg
	if len(args) > 0 {
		formatted = fmt.Sprintf(szMsg, args...)
	}
	msg := Common.AcquireBaseMessage()
	msg.EMsgType = int(eMsgType)
	msg.NSeverCode = nSeverCode
	msg.StrMsg = formatted
	msg.StrFile = pszFile
	msg.StrFunc = pszFuncName
	msg.NLine = nLine
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = Common.GetCYPublicFunctionInstance().GetCurrentThreadId()
	l.control.Write(msg)
	Common.ReleaseBaseMessage(msg)
}

func (l *CYLoggerImpl) WriteEscapeLogFmt(nLogLevel int, eMsgType Core.ELogType, nSeverCode int,
	pszFile, pszFuncName string, nLine int, szMsg string, args ...any) {

	if !l.bInit.Load() || l.bExit.Load() {
		return
	}
	if l.control == nil {
		return
	}
	formatted := szMsg
	if len(args) > 0 {
		formatted = fmt.Sprintf(szMsg, args...)
	}
	msg := Common.AcquireEscapeMessage()
	msg.EMsgType = int(eMsgType)
	msg.NSeverCode = nSeverCode
	msg.StrMsg = formatted
	msg.StrFile = pszFile
	msg.StrFunc = pszFuncName
	msg.NLine = nLine
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = Common.GetCYPublicFunctionInstance().GetCurrentThreadId()
	l.control.Write(&msg.CYBaseMessage)
	Common.ReleaseEscapeMessage(msg)
}

func (l *CYLoggerImpl) WriteHexLog(nLogLevel int, eMsgType Core.ELogType, nSeverCode int,
	pszFile, pszFuncName string, nLine int, data []byte) {

	if !l.bInit.Load() || l.bExit.Load() {
		return
	}
	if l.control == nil {
		return
	}
	hex := formatHex(data)
	msg := Common.AcquireBaseMessage()
	msg.EMsgType = int(eMsgType)
	msg.NSeverCode = nSeverCode
	msg.StrMsg = hex
	msg.StrFile = pszFile
	msg.StrFunc = pszFuncName
	msg.NLine = nLine
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = Common.GetCYPublicFunctionInstance().GetCurrentThreadId()
	l.control.Write(msg)
	Common.ReleaseBaseMessage(msg)
}

func formatHex(data []byte) string {
	const lineWidth = 16
	result := ""
	for i := 0; i < len(data); i += lineWidth {
		end := i + lineWidth
		if end > len(data) {
			end = len(data)
		}
		result += fmt.Sprintf("%04x: ", i)
		for j := i; j < end; j++ {
			result += fmt.Sprintf("%02x ", data[j])
		}
		if end-i < lineWidth {
			for j := end - i; j < lineWidth; j++ {
				result += "   "
			}
		}
		result += " |"
		for j := i; j < end; j++ {
			c := data[j]
			if c >= 32 && c < 127 {
				result += string(c)
			} else {
				result += "."
			}
		}
		result += "|\n"
	}
	return result
}

func (l *CYLoggerImpl) SetConfig(szLogPath string, bShowConsoleWindow bool) {
	if l.config == nil {
		l.config = Config.GetCYLoggerConfigInstance()
	}
	l.config.SetLogPath(szLogPath)
	l.config.SetShowConsole(bShowConsoleWindow)
}

func (l *CYLoggerImpl) SetRestriction(bEnableCheck, bClearUnLogFile bool,
	nLimitTimeClearLog, nLimitTimeExpiredFile, nCheckFileSizeTime, nCheckFileCountTime,
	nCheckFileSize, nFileCountPerType, nCheckFileTypeSize, nCheckAllFileSize int) {

	if l.control != nil {
		l.control.SetRestriction(bEnableCheck, bClearUnLogFile,
			nLimitTimeClearLog, nLimitTimeExpiredFile, nCheckFileSizeTime, nCheckFileCountTime,
			nCheckFileSize, nFileCountPerType, nCheckFileTypeSize, nCheckAllFileSize)
	}
}

func (l *CYLoggerImpl) SetFilter(pFilter *Filter.ICYLoggerPatternFilter) {
	if l.control != nil {
		l.control.SetFilter(pFilter)
	}
}

func (l *CYLoggerImpl) SetLayout(eLayoutType Core.ELogLayoutType, pLayout Layout.ICYLoggerTemplateLayout) {
	if l.control != nil {
		l.control.SetLayout(eLayoutType, pLayout)
	}
}

func (l *CYLoggerImpl) GetStats(pStats *Core.STStatistics) bool {
	if l.control == nil {
		return false
	}
	return Statistics.GetCYStatisticsInstance().GetStats(pStats)
}

func (l *CYLoggerImpl) GetLogLevel() Core.ELogLevelFilter {
	if l.config != nil {
		return l.config.GetLogLevelFilter()
	}
	return Core.DefaultLogLevelFilter
}

func (l *CYLoggerImpl) SetLogLevel(eLogLevelFilter Core.ELogLevelFilter) {
	if l.config != nil {
		l.config.SetLogLevelFilter(eLogLevelFilter)
	}
	if l.control != nil {
		l.control.SetLogLevel(eLogLevelFilter)
	}
}

func (l *CYLoggerImpl) IsInit() bool {
	return l.bInit.Load()
}

// ---- Public package-level convenience functions ----

func InitLogger(szLogPath string, bShowConsoleWindow bool) bool {
	l := GetLoggerInstance()
	l.SetConfig(szLogPath, bShowConsoleWindow)
	return l.Init()
}

func UnInitLogger() {
	GetLoggerInstance().UnInit()
}

func FreeInstance() {
	UnInitLogger()
}

func GetInstance() *CYLoggerImpl {
	return GetLoggerInstance()
}

func FlushLogger() {
	GetLoggerInstance().Flush(Core.LogTypeMax)
}

func FlushLoggerType(eType Core.ELogType) {
	GetLoggerInstance().Flush(eType)
}

// ---- Logging convenience functions ----

func LOG_TRACE(szMsg string, args ...any) {
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelTrace), Core.LogTypeTrace, -1, "", "", 0, szMsg, args...)
}

func LOG_DEBUG(szMsg string, args ...any) {
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelDebug), Core.LogTypeDebug, -1, "", "", 0, szMsg, args...)
}

func LOG_INFO(szMsg string, args ...any) {
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, "", "", 0, szMsg, args...)
}

func LOG_WARN(szMsg string, args ...any) {
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelWarn), Core.LogTypeWarn, -1, "", "", 0, szMsg, args...)
}

func LOG_ERROR(szMsg string, args ...any) {
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelError), Core.LogTypeError, -1, "", "", 0, szMsg, args...)
}

func LOG_FATAL(szMsg string, args ...any) {
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelFatal), Core.LogTypeFatal, -1, "", "", 0, szMsg, args...)
}

func LOG_MAIN(szMsg string, args ...any) {
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeMain, -1, "", "", 0, szMsg, args...)
}

func LOG_SYS(szMsg string, args ...any) {
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelSys), Core.LogTypeSys, -1, "", "", 0, szMsg, args...)
}

func LOG_REMOTE(szMsg string, args ...any) {
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelRemote), Core.LogTypeRemote, -1, "", "", 0, szMsg, args...)
}
