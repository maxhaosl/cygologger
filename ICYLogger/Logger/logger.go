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
	"runtime"
	"strings"
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
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Statistics"
)

// gWithThreadId caches the WithThreadId configuration switch as an atomic so
// the hot logging path can consult it without taking the config RWMutex. It is
// refreshed from the config on every Init. When off, currentThreadId skips the
// runtime.Stack call entirely — CPU profiles showed runtime.Stack's internal
// runtime lock serialising ALL concurrent logging goroutines (>90% of logging
// CPU at 8+ goroutines), making it the dominant scalability bottleneck.
var gWithThreadId atomic.Bool

func init() { gWithThreadId.Store(Core.LOG_WITH_THREAD_ID) }

// currentThreadId returns the goroutine ID for the T: field, or 0 when the
// WithThreadId switch is off (the expensive runtime.Stack call is skipped).
func currentThreadId() uint64 {
	if !gWithThreadId.Load() {
		return 0
	}
	return Common.GetGID()
}

// CYLoggerControl routes log messages to the correct Entity.
type CYLoggerControl struct {
	Common.CYNoCopy
	mu          sync.RWMutex
	restrictionMu sync.Mutex
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

func (c *CYLoggerControl) Init(szLogPath string, bShowConsole, bWriteRemote, bWriteSys bool,
	eFileMode Core.ELogFileMode, szRemoteAddr string) bool {
	c.mu.Lock()
	c.szLogPath = szLogPath
	c.filter = Filter.GetCYLoggerPatternFilterManagerInstance().GetFilter()
	// Honour the LOG_LAYOUT_TYPE config option (set via WithLayoutType / the
	// C++ LOG_LAYOUT_TYPE default) instead of the hardcoded default, so a caller
	// can select a different template at Init time. The runtime SetLayout shortcut
	// still works for post-Init changes.
	cfg := Core.GetCYLoggerConfigInstance()
	c.layout = Layout.GetCYLoggerTemplateLayoutManagerInstance().GetLayout(cfg.GetLayoutType())

	if bShowConsole {
		if c.consoleApp != nil {
			c.consoleApp.UnInit()
		}
		c.consoleApp = Appender.GetCYLoggerAppenderFactoryInstance().CreateConsoleAppender(Core.LogTypeConsole, "", "", szLogPath, eFileMode)
		c.consoleApp.SetLayout(c.layout)
		c.consoleApp.SetFilter(c.filter)
	} else if c.consoleApp != nil {
		// LOG_SHOW_CONSOLE_WINDOW=false must actually drop the console
		// appender. The previous condition (bShowConsole || c.consoleApp == nil)
		// created an enabled console appender on the very first Init even when
		// the caller passed WithConsole(false), flooding stdout under load.
		c.consoleApp.UnInit()
		c.consoleApp = nil
	}
	c.mu.Unlock()

	// Auto-mount per-type file appenders, mirroring the C++ CY_LOG_APPENDER macro.
	// The exact set of mounted types is driven by the configured level filter
	// (LOG_LEVEL_FILTER): each enabled level bit produces its own file. This is
	// the single source of truth for both which messages are written AND which
	// files are created, so callers select their file set purely via WithLogLevel.
	//
	// IMPORTANT: AddAppender locks c.mu itself, so we must NOT hold c.mu here —
	// holding it across the call would deadlock (sync.RWMutex is not reentrant).
	// c.SetLogLevel also locks c.mu, so it is called here (outside the loop),
	// never inside a c.mu-held section.
	c.SetLogLevel(cfg.GetLogLevelFilter())
	filter := c.GetLogLevel()
	// Mount one file appender per log type whose level bit is enabled by the
	// active filter. Because typeEnabledByFilter maps each type to its level bit,
	// LOG_LEVEL_FILTER alone decides which files exist — e.g. a filter of
	// LogLevelError creates only Error.log; Trace|Info|Warn|Error creates the
	// four debug files; an arbitrary subset creates exactly that subset.
	var fileTypes []Core.ELogType
	for _, m := range []struct {
		t   Core.ELogType
		lvl Core.ELogLevel
	}{
		{Core.LogTypeTrace, Core.LogLevelTrace},
		{Core.LogTypeInfo, Core.LogLevelInfo},
		{Core.LogTypeWarn, Core.LogLevelWarn},
		{Core.LogTypeError, Core.LogLevelError},
		{Core.LogTypeDebug, Core.LogLevelDebug},
		{Core.LogTypeFatal, Core.LogLevelFatal},
	} {
		if int(filter)&int(m.lvl) != 0 {
			fileTypes = append(fileTypes, m.t)
		}
	}
	// Main aggregates every enabled type. It is mounted only when the
	// bMountMain switch is on AND at least one level is enabled. The default is
	// on (backward compatible with the historical all-types behaviour); callers
	// may turn it off (WithMountMain(false)) to keep a strict per-level file set
	// without a Main.log.
	if cfg.IsMountMain() && filter != 0 {
		fileTypes = append(fileTypes, Core.LogTypeMain)
	}
	for _, t := range fileTypes {
		if !c.hasAppender(t) {
			c.AddAppender(t, "", "", eFileMode)
		}
	}
	// Sys/Remote are independent sinks controlled by their own switches
	// (LOG_WRITE_SYS / LOG_WRITE_REMOTE), NOT by the per-level LOG_LEVEL_FILTER.
	// Mounting them here is driven solely by the switch; whether any message
	// actually reaches them is still decided per-write by passesFilter (their
	// LogLevelSys / LogLevelRemote bits). Gating the mount by typeEnabledByFilter
	// would be wrong: the default LogFilterAll does not include those bits, so a
	// switch-on Sys/Remote would never create its file.
	if bWriteSys && !c.hasAppender(Core.LogTypeSys) {
		c.AddAppender(Core.LogTypeSys, "", "", eFileMode)
	}
	if bWriteRemote && !c.hasAppender(Core.LogTypeRemote) {
		c.AddAppender(Core.LogTypeRemote, "", szRemoteAddr, eFileMode)
	}

	// Apply the restriction configuration and start the cleanup schedule. These
	// touch c.schedule / c.restriction, so guard them under c.mu again.
	c.mu.Lock()
	defer c.mu.Unlock()

	// Apply the restriction configuration stored in CYLoggerConfig to the runtime
	// restriction object so a single Init applies the same defaults/options as C++.
	c.SetRestriction(cfg.IsLimitEnable(), cfg.IsClearUnLogFile(),
		cfg.GetTimeClearLog(), cfg.GetTimeExpiredFile(),
		cfg.GetCheckFileSizeTime(), cfg.GetCheckFileCountTime(),
		cfg.GetCheckFileSize(), cfg.GetCountPerType(),
		cfg.GetCheckFileTypeSize(), cfg.GetCheckAllFileSize())

	if c.schedule == nil {
		c.schedule = Schedule.GetCYLoggerScheduleInstance()
		c.schedule.Init(szLogPath, cfg.GetTimeExpiredFile())
	}
	// The schedule singleton (and its clear task) survives UnInit, so the
	// per-Init settings must be re-asserted on every Init — otherwise a
	// re-Init in the same process keeps cleaning the PREVIOUS log directory
	// with the PREVIOUS period and the new directory is never cleaned.
	c.schedule.SetLogDir(szLogPath)
	c.schedule.SetExpiredHours(cfg.GetTimeExpiredFile())
	c.schedule.SetClearPeriodSec(cfg.GetClearPeriodSec())
	// Propagate the master detection switch (LOG_LIMIT_ENABLE) to the cleanup
	// task: when disabled, no expired/count/size cleanup runs at all.
	c.schedule.SetEnable(cfg.IsLimitEnable())
	c.schedule.SetRestriction(
		c.restriction.GetFileCountPerType(),
		c.restriction.GetCheckFileTypeSize(),
		c.restriction.GetCheckAllFileSize(),
	)
	c.schedule.SetCheckFileSizeTime(c.restriction.GetCheckFileSizeTime())
	c.schedule.SetCheckFileCountTime(c.restriction.GetCheckFileCountTime())
	c.schedule.SetClearUnLogFile(c.restriction.IsClearUnLogFile())
	c.schedule.StartSchedule()

	return true
}

// hasAppender reports whether the entity for eLogType already holds an appender,
// so auto-mounting never duplicates a manually registered appender.
func (c *CYLoggerControl) hasAppender(eLogType Core.ELogType) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if e := c.entities[eLogType]; e != nil {
		return e.GetAppenderCount() > 0
	}
	return false
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
	// Drop the cached console appender and the appender registry so a later
	// InitDefaultWithOpts in the same process re-evaluates every config option
	// (LOG_SHOW_CONSOLE_WINDOW / LOG_WRITE_REMOTE / LOG_WRITE_SYS) from scratch
	// instead of leaking appenders mounted by a previous initialization.
	c.consoleApp = nil
	Appender.GetCYLoggerAppenderFactoryInstance().Reset()
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
		// c.consoleApp is read by Write under c.mu.RLock, so the write must be
		// serialized with the same lock.
		c.mu.Lock()
		if c.consoleApp != nil {
			c.consoleApp.UnInit()
		}
		c.consoleApp = factory.CreateConsoleAppender(eLogType, szChannel, szFile, c.szLogPath, eFileMode)
		c.consoleApp.SetLayout(c.layout)
		c.consoleApp.SetFilter(c.filter)
		c.consoleApp.SetEnable(true)
		c.mu.Unlock()
		return true
	}

	// The entities map is read by Write under c.mu.RLock, so the lookup+insert
	// must be serialized with the same lock to avoid a data race.
	entityFactory := Entity.GetCYLoggerEntityFactoryInstance()
	entityItem := entityFactory.CreateEntity(eLogType)
	c.mu.Lock()
	c.entities[eLogType] = entityItem
	c.mu.Unlock()

	entityItem.AddAppender(appender_)
	factory.RegisterAppender(appender_)

	if !entityItem.Init() {
		return false
	}

	appender_.Start(appender_.Run)

	// Share the control's restriction object with the file/main appenders so that
	// runtime policy changes (master switch, per-file size threshold, …) take
	// effect on their rotation logic immediately. Without this, WithRestriction's
	// nCheckFileSize / LOG_LIMIT_ENABLE never reach the live appenders.
	if fa, ok := appender_.(*Appender.CYLoggerFileAppender); ok {
		fa.SetRestriction(c.restriction)
	} else if ma, ok := appender_.(*Appender.CYLoggerMainAppender); ok {
		ma.SetRestriction(c.restriction)
	}

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

	// Aggregate every message into the Main log, mirroring the C++ Main appender
	// behaviour. A message whose own type is Main is already written above, so it
	// is skipped here to avoid duplication. When the Main appender is not mounted
	// (bMountMain=false), GetAppenderCount()==0 and the aggregate write is skipped.
	if eMsgType != Core.LogTypeMain {
		if mainEntity := c.entities[Core.LogTypeMain]; mainEntity != nil && mainEntity.GetAppenderCount() > 0 {
			mainEntity.Write(msg)
		}
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

// typeEnabledByFilter reports whether eMsgType should be mounted as a file
// appender given the effective level filter (c.eLogLevel). A level disabled by
// the filter neither receives output nor causes its dedicated file to be created,
// mirroring the C++ LOG_LEVEL_FILTER semantics where a suppressed level is fully
// turned off (no file, no writes). Main is the aggregate of every enabled type, so
// for the purpose of this per-type gate it is always considered enabled; the
// actual Main appender is mounted only when both the level filter is non-empty
// AND the bMountMain switch is on (see Init).
func (c *CYLoggerControl) typeEnabledByFilter(eMsgType Core.ELogType) bool {
	if eMsgType == Core.LogTypeMain {
		return true
	}
	return int(c.eLogLevel)&int(c.levelForType(eMsgType)) != 0
}

// WriteDirect writes a message directly to the entity, bypassing level filtering.
// This is the Go equivalent of C++ CY_LOG_DIRECT_* macros.
func (c *CYLoggerControl) WriteDirect(eMsgType Core.ELogType, msg *Common.CYBaseMessage) {
	if msg == nil {
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Bypass level filter — always write to console and target entity.
	if c.consoleApp != nil && c.consoleApp.IsEnable() {
		c.consoleApp.Write(msg)
	}

	if logEntity := c.entities[eMsgType]; logEntity != nil {
		logEntity.Write(msg)
	}

	// Aggregate every message into the Main log, mirroring the C++ Main appender
	// behaviour. A message whose own type is Main is already written above, so it
	// is skipped here to avoid duplication. When the Main appender is not mounted
	// (bMountMain=false), GetAppenderCount()==0 and the aggregate write is skipped.
	if eMsgType != Core.LogTypeMain {
		if mainEntity := c.entities[Core.LogTypeMain]; mainEntity != nil && mainEntity.GetAppenderCount() > 0 {
			mainEntity.Write(msg)
		}
	}

	Statistics.GetCYStatisticsInstance().IncrementLine(eMsgType, uint64(len(msg.StrMsg)))
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
	c.restrictionMu.Lock()
	defer c.restrictionMu.Unlock()
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
	config  *Core.CYLoggerConfig
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

	l.config = Core.GetCYLoggerConfigInstance()
	// Refresh the hot-path thread-id switch from the (possibly re-)applied
	// configuration options.
	gWithThreadId.Store(l.config.IsWithThreadId())
	l.control = GetCYLoggerControlInstance()

	if !l.control.Init(l.config.GetLogPath(), l.config.IsShowConsole(), l.config.IsWriteRemote(), l.config.IsWriteSys(),
		l.config.GetFileMode(), l.config.GetRemoteAddr()) {
		return false
	}

	// Reset the exit flag so a re-Init after UnInit (Close) accepts writes
	// again. Without this, every Write* silently drops messages for the rest
	// of the process lifetime after the first Close.
	l.bExit.Store(false)
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
	msg.NThreadId = currentThreadId()
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
	msg.NThreadId = currentThreadId()
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
	msg.NThreadId = currentThreadId()
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
	msg.NThreadId = currentThreadId()
	l.control.Write(msg)
	Common.ReleaseBaseMessage(msg)
}

// ============================================================================
// Channel-aware logging methods — mirror the C++ ICYLogger::WriteLog(szChannel)
// overloads. The channel is stored on the message and rendered by the layout's
// [channel] / Channel: field, overriding the appender channel for that message.
// ============================================================================

// WriteLogCh writes a raw log message with an explicit channel.
func (l *CYLoggerImpl) WriteLogCh(nLogLevel int, eMsgType Core.ELogType, nSeverCode int, szChannel, szMsg string) {
	if !l.bInit.Load() || l.bExit.Load() {
		return
	}
	if l.control == nil {
		return
	}
	msg := Common.AcquireBaseMessage()
	msg.EMsgType = int(eMsgType)
	msg.NSeverCode = nSeverCode
	msg.StrChannel = szChannel
	msg.StrMsg = szMsg
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = currentThreadId()
	l.control.Write(msg)
	Common.ReleaseBaseMessage(msg)
}

// WriteLogFmtCh writes a formatted log message with an explicit channel and caller info.
func (l *CYLoggerImpl) WriteLogFmtCh(nLogLevel int, eMsgType Core.ELogType, nSeverCode int,
	szChannel, pszFile, pszFuncName string, nLine int, szMsg string, args ...any) {

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
	msg.StrChannel = szChannel
	msg.StrMsg = formatted
	msg.StrFile = pszFile
	msg.StrFunc = pszFuncName
	msg.NLine = nLine
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = currentThreadId()
	l.control.Write(msg)
	Common.ReleaseBaseMessage(msg)
}

// WriteEscapeLogFmtCh writes an escape-formatted log message with an explicit channel.
func (l *CYLoggerImpl) WriteEscapeLogFmtCh(nLogLevel int, eMsgType Core.ELogType, nSeverCode int,
	szChannel, pszFile, pszFuncName string, nLine int, szMsg string, args ...any) {

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
	msg.StrChannel = szChannel
	msg.StrMsg = formatted
	msg.StrFile = pszFile
	msg.StrFunc = pszFuncName
	msg.NLine = nLine
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = currentThreadId()
	l.control.Write(&msg.CYBaseMessage)
	Common.ReleaseEscapeMessage(msg)
}

// WriteHexLogCh writes a hex dump with an explicit channel.
func (l *CYLoggerImpl) WriteHexLogCh(nLogLevel int, eMsgType Core.ELogType, nSeverCode int,
	szChannel, pszFile, pszFuncName string, nLine int, data []byte) {

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
	msg.StrChannel = szChannel
	msg.StrMsg = hex
	msg.StrFile = pszFile
	msg.StrFunc = pszFuncName
	msg.NLine = nLine
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = currentThreadId()
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
		l.config = Core.GetCYLoggerConfigInstance()
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

// ForceNewFile forces every log entity to rotate to a fresh file (mirrors C++ ForceEntityNewFile).
func (l *CYLoggerImpl) ForceNewFile() {
	Entity.GetCYLoggerEntityFactoryInstance().ForceEntityNewFile()
}

// GetLoggerEntity returns the entity registered for eLogType, mirroring C++ GetLoggerEntity.
func (l *CYLoggerImpl) GetLoggerEntity(eLogType Core.ELogType) *Entity.CYLoggerEntity {
	return Entity.GetCYLoggerEntityFactoryInstance().GetLoggerEntity(eLogType)
}

// ReleaseLoggerEntity flushes, detaches and removes the entity for eLogType,
// mirroring C++ ReleaseLoggerEntity.
func (l *CYLoggerImpl) ReleaseLoggerEntity(eLogType Core.ELogType) {
	Entity.GetCYLoggerEntityFactoryInstance().ReleaseLoggerEntity(eLogType)
}

// ResetLogFile forces every file appender to rotate to a fresh file (C++ ResetLogFile).
func (l *CYLoggerImpl) ResetLogFile() {
	Schedule.GetCYLoggerScheduleInstance().ResetLogFile()
}

// AddLogType records an extra log type tracked by the schedule (C++ AddLogType).
func (l *CYLoggerImpl) AddLogType(eLogType Core.ELogType) {
	Schedule.GetCYLoggerScheduleInstance().AddLogType(eLogType)
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

// ---- Direct logging (bypasses level filter) ----

// WriteLogDirect writes a raw log message directly to the target entity, bypassing level filtering.
func (l *CYLoggerImpl) WriteLogDirect(eMsgType Core.ELogType, nSeverCode int, szMsg string) {
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
	msg.NThreadId = currentThreadId()
	l.control.WriteDirect(eMsgType, msg)
	Common.ReleaseBaseMessage(msg)
}

// WriteLogFmtDirect writes a formatted log message directly to the target entity, bypassing level filtering.
func (l *CYLoggerImpl) WriteLogFmtDirect(eMsgType Core.ELogType, nSeverCode int,
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
	msg.NThreadId = currentThreadId()
	l.control.WriteDirect(eMsgType, msg)
	Common.ReleaseBaseMessage(msg)
}

// WriteEscapeLogFmtDirect writes an escape-formatted log message directly, bypassing level filtering.
func (l *CYLoggerImpl) WriteEscapeLogFmtDirect(eMsgType Core.ELogType, nSeverCode int,
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
	msg.NThreadId = currentThreadId()
	l.control.WriteDirect(eMsgType, &msg.CYBaseMessage)
	Common.ReleaseEscapeMessage(msg)
}

// WriteHexLogDirect writes hex-formatted log data directly, bypassing level filtering.
func (l *CYLoggerImpl) WriteHexLogDirect(eMsgType Core.ELogType, nSeverCode int,
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
	msg.NThreadId = currentThreadId()
	l.control.WriteDirect(eMsgType, msg)
	Common.ReleaseBaseMessage(msg)
}

// ============================================================================
// Channel-aware direct logging (bypass level filter) — mirror the C++ channel
// WriteLog overloads but routed through WriteDirect, so the channel is carried
// on the message and rendered by the layout.
// ============================================================================

// WriteLogDirectCh writes a raw log message with an explicit channel, bypassing level filtering.
func (l *CYLoggerImpl) WriteLogDirectCh(eMsgType Core.ELogType, nSeverCode int, szChannel, szMsg string) {
	if !l.bInit.Load() || l.bExit.Load() {
		return
	}
	if l.control == nil {
		return
	}
	msg := Common.AcquireBaseMessage()
	msg.EMsgType = int(eMsgType)
	msg.NSeverCode = nSeverCode
	msg.StrChannel = szChannel
	msg.StrMsg = szMsg
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = currentThreadId()
	l.control.WriteDirect(eMsgType, msg)
	Common.ReleaseBaseMessage(msg)
}

// WriteLogFmtDirectCh writes a formatted log message with an explicit channel, bypassing level filtering.
func (l *CYLoggerImpl) WriteLogFmtDirectCh(eMsgType Core.ELogType, nSeverCode int,
	szChannel, pszFile, pszFuncName string, nLine int, szMsg string, args ...any) {

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
	msg.StrChannel = szChannel
	msg.StrMsg = formatted
	msg.StrFile = pszFile
	msg.StrFunc = pszFuncName
	msg.NLine = nLine
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = currentThreadId()
	l.control.WriteDirect(eMsgType, msg)
	Common.ReleaseBaseMessage(msg)
}

// WriteEscapeLogFmtDirectCh writes an escape-formatted log message with an explicit channel, bypassing filtering.
func (l *CYLoggerImpl) WriteEscapeLogFmtDirectCh(eMsgType Core.ELogType, nSeverCode int,
	szChannel, pszFile, pszFuncName string, nLine int, szMsg string, args ...any) {

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
	msg.StrChannel = szChannel
	msg.StrMsg = formatted
	msg.StrFile = pszFile
	msg.StrFunc = pszFuncName
	msg.NLine = nLine
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = currentThreadId()
	l.control.WriteDirect(eMsgType, &msg.CYBaseMessage)
	Common.ReleaseEscapeMessage(msg)
}

// WriteHexLogDirectCh writes a hex dump with an explicit channel, bypassing level filtering.
func (l *CYLoggerImpl) WriteHexLogDirectCh(eMsgType Core.ELogType, nSeverCode int,
	szChannel, pszFile, pszFuncName string, nLine int, data []byte) {

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
	msg.StrChannel = szChannel
	msg.StrMsg = hex
	msg.StrFile = pszFile
	msg.StrFunc = pszFuncName
	msg.NLine = nLine
	msg.Time = time.Now()
	msg.NProcessId = uint64(Common.GetCYPublicFunctionInstance().GetCurrentProcessId())
	msg.NThreadId = currentThreadId()
	l.control.WriteDirect(eMsgType, msg)
	Common.ReleaseBaseMessage(msg)
}

// ---- Internal helpers ----

// callerInfo captures the caller's file, function name, and line number.
// skip=0 is callerInfo itself, skip=1 is its immediate caller, etc.
func callerInfo(skip int) (file, funcName string, line int) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "???", "???", 0
	}
	funcName = runtime.FuncForPC(pc).Name()
	// Shorten file to base name.
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		file = file[idx+1:]
	}
	if idx := strings.LastIndex(file, "\\"); idx >= 0 {
		file = file[idx+1:]
	}
	// Keep only the last package-qualified function name segment.
	if idx := strings.LastIndex(funcName, "/"); idx >= 0 {
		funcName = funcName[idx+1:]
	}
	return
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
	Statistics.GetCYStatisticsInstance().Reset()
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

// ---- Logging convenience functions (auto caller info via runtime.Caller) ----

// LOG_TRACE writes a trace-level log with automatic caller file/line/function capture.
func LOG_TRACE(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelTrace), Core.LogTypeTrace, -1, file, funcName, line, szMsg, args...)
}

// LOG_DEBUG writes a debug-level log with automatic caller file/line/function capture.
func LOG_DEBUG(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelDebug), Core.LogTypeDebug, -1, file, funcName, line, szMsg, args...)
}

// LOG_INFO writes an info-level log with automatic caller file/line/function capture.
func LOG_INFO(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, file, funcName, line, szMsg, args...)
}

// LOG_WARN writes a warn-level log with automatic caller file/line/function capture.
func LOG_WARN(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelWarn), Core.LogTypeWarn, -1, file, funcName, line, szMsg, args...)
}

// LOG_ERROR writes an error-level log with automatic caller file/line/function capture.
func LOG_ERROR(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelError), Core.LogTypeError, -1, file, funcName, line, szMsg, args...)
}

// LOG_FATAL writes a fatal-level log with automatic caller file/line/function capture.
func LOG_FATAL(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelFatal), Core.LogTypeFatal, -1, file, funcName, line, szMsg, args...)
}

// LOG_MAIN writes a main-type log with automatic caller file/line/function capture.
func LOG_MAIN(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeMain, -1, file, funcName, line, szMsg, args...)
}

// LOG_SYS writes a system-type log with automatic caller file/line/function capture.
func LOG_SYS(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelSys), Core.LogTypeSys, -1, file, funcName, line, szMsg, args...)
}

// LOG_REMOTE writes a remote-type log with automatic caller file/line/function capture.
func LOG_REMOTE(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmt(int(Core.LogLevelRemote), Core.LogTypeRemote, -1, file, funcName, line, szMsg, args...)
}

// ---- Direct logging convenience functions (bypass level filter, auto caller info) ----

// LOG_DIRECT_TRACE writes a trace log directly, bypassing level filtering.
func LOG_DIRECT_TRACE(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirect(Core.LogTypeTrace, -1, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_DEBUG writes a debug log directly, bypassing level filtering.
func LOG_DIRECT_DEBUG(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirect(Core.LogTypeDebug, -1, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_INFO writes an info log directly, bypassing level filtering.
func LOG_DIRECT_INFO(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirect(Core.LogTypeInfo, -1, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_WARN writes a warn log directly, bypassing level filtering.
func LOG_DIRECT_WARN(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirect(Core.LogTypeWarn, -1, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_ERROR writes an error log directly, bypassing level filtering.
func LOG_DIRECT_ERROR(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirect(Core.LogTypeError, -1, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_FATAL writes a fatal log directly, bypassing level filtering.
func LOG_DIRECT_FATAL(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirect(Core.LogTypeFatal, -1, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_MAIN writes a main log directly, bypassing level filtering.
func LOG_DIRECT_MAIN(szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirect(Core.LogTypeMain, -1, file, funcName, line, szMsg, args...)
}

// ---- Channel-aware logging convenience functions (auto caller info) ----
// These mirror the C++ macros (e.g. CY_LOG_TRACE) but add an explicit channel
// argument, so the per-message channel is rendered by the layout.

// LOG_TRACE_CH writes a trace-level log to the given channel with automatic caller info.
func LOG_TRACE_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtCh(int(Core.LogLevelTrace), Core.LogTypeTrace, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_DEBUG_CH writes a debug-level log to the given channel with automatic caller info.
func LOG_DEBUG_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtCh(int(Core.LogLevelDebug), Core.LogTypeDebug, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_INFO_CH writes an info-level log to the given channel with automatic caller info.
func LOG_INFO_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtCh(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_WARN_CH writes a warn-level log to the given channel with automatic caller info.
func LOG_WARN_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtCh(int(Core.LogLevelWarn), Core.LogTypeWarn, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_ERROR_CH writes an error-level log to the given channel with automatic caller info.
func LOG_ERROR_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtCh(int(Core.LogLevelError), Core.LogTypeError, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_FATAL_CH writes a fatal-level log to the given channel with automatic caller info.
func LOG_FATAL_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtCh(int(Core.LogLevelFatal), Core.LogTypeFatal, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_MAIN_CH writes a main-type log to the given channel with automatic caller info.
func LOG_MAIN_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtCh(int(Core.LogLevelInfo), Core.LogTypeMain, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_SYS_CH writes a system-type log to the given channel with automatic caller info.
func LOG_SYS_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtCh(int(Core.LogLevelSys), Core.LogTypeSys, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_REMOTE_CH writes a remote-type log to the given channel with automatic caller info.
func LOG_REMOTE_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtCh(int(Core.LogLevelRemote), Core.LogTypeRemote, -1, szChannel, file, funcName, line, szMsg, args...)
}

// ---- Channel-aware direct logging convenience functions (bypass level filter) ----

// LOG_DIRECT_TRACE_CH writes a trace log to the given channel directly, bypassing level filtering.
func LOG_DIRECT_TRACE_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirectCh(Core.LogTypeTrace, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_DEBUG_CH writes a debug log to the given channel directly, bypassing level filtering.
func LOG_DIRECT_DEBUG_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirectCh(Core.LogTypeDebug, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_INFO_CH writes an info log to the given channel directly, bypassing level filtering.
func LOG_DIRECT_INFO_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirectCh(Core.LogTypeInfo, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_WARN_CH writes a warn log to the given channel directly, bypassing level filtering.
func LOG_DIRECT_WARN_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirectCh(Core.LogTypeWarn, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_ERROR_CH writes an error log to the given channel directly, bypassing level filtering.
func LOG_DIRECT_ERROR_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirectCh(Core.LogTypeError, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_FATAL_CH writes a fatal log to the given channel directly, bypassing level filtering.
func LOG_DIRECT_FATAL_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirectCh(Core.LogTypeFatal, -1, szChannel, file, funcName, line, szMsg, args...)
}

// LOG_DIRECT_MAIN_CH writes a main log to the given channel directly, bypassing level filtering.
func LOG_DIRECT_MAIN_CH(szChannel, szMsg string, args ...any) {
	file, funcName, line := callerInfo(2)
	GetLoggerInstance().WriteLogFmtDirectCh(Core.LogTypeMain, -1, szChannel, file, funcName, line, szMsg, args...)
}
