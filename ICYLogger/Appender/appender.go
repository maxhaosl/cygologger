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

// Package Appender provides all log appender implementations.
package Appender

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Filter"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Layout"
	Common "github.com/maxhaosl/CYGoLogger/ICYLogger/Common"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Statistics"
)

// IAppender is the common interface for all appender types.
type IAppender interface {
	Init() bool
	UnInit()
	Flush()
	Write(pMsg *Common.CYBaseMessage) bool
	GetLogType() Core.ELogType
	GetChannel() string
	GetFile() string
	GetLogPath() string
	SetLogPath(string)
	GetFileMode() Core.ELogFileMode
	IsEnable() bool
	SetEnable(bool)
	GetLayout() Layout.ICYLoggerTemplateLayout
	SetLayout(Layout.ICYLoggerTemplateLayout)
	GetFilter() *Filter.ICYLoggerPatternFilter
	SetFilter(*Filter.ICYLoggerPatternFilter)
	GetFPSCounter() *Common.CYFPSCounter
	GetQueueSize() int
	Run()
	Start(run func())
	// GetId returns the unique log-type id of this appender (mirrors C++ GetId).
	GetId() Core.ELogType
	// GetLogName returns the active log file path (mirrors C++ GetLogName).
	GetLogName() string
	// ForceNewFile forces rotation to a fresh log file (mirrors C++ ForceNewFile).
	ForceNewFile()
	// GetSize returns the current size of the backing store in bytes.
	GetSize() int64
	// Copy copies the backing store to target (mirrors C++ Copy).
	Copy(target string)
	// ClearContents truncates the backing store (mirrors C++ ClearContents).
	ClearContents()
}

// LOG_TYPE_BUFFER is a virtual type for buffer appenders.
const LOG_TYPE_BUFFER Core.ELogType = 100

// CYLoggerBaseAppender is the base appender with true double-buffering.
// Producers write to the "front" queue; a periodic swap moves it to the
// "back" queue where the consumer drains it lock-free.  This mimics the
// C++ atomic swap of FrontQueue <-> BackQueue.
type CYLoggerBaseAppender struct {
	Common.CYNamedThread
	Common.CYNoCopy
	mu              sync.Mutex
	closeOnce       sync.Once // guarantees the queues are closed exactly once
	eLogType        Core.ELogType
	szChannel       string
	szFile          string
	szLogPath       string
	eFileMode       Core.ELogFileMode
	bEnable         atomic.Bool
	nBufferCapacity int

	PublicQueue  chan *Common.CYBaseMessage
	PrivateQueue chan *Common.CYBaseMessage
	swapCh       chan struct{} // signals the consumer to swap buffers
	flushCh      chan chan struct{} // flush handshake: send a done chan, receive when drained
	swapTick     *time.Ticker  // periodic swap trigger
	fpsCounter   *Common.CYFPSCounter
	layout       Layout.ICYLoggerTemplateLayout
	filter       *Filter.ICYLoggerPatternFilter
}

func newBaseAppender(eLogType Core.ELogType, szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode) *CYLoggerBaseAppender {
	app := &CYLoggerBaseAppender{
		eLogType:        eLogType,
		szChannel:       szChannel,
		szFile:          szFile,
		szLogPath:       szLogPath,
		eFileMode:       eFileMode,
		nBufferCapacity: 8192,
		PublicQueue:     make(chan *Common.CYBaseMessage, 8192),
		PrivateQueue:    make(chan *Common.CYBaseMessage, 8192),
		swapCh:          make(chan struct{}, 1),
		flushCh:         make(chan chan struct{}),
	}
	app.fpsCounter = Common.NewCYFPSCounter(Core.LOG_FPS_CHECK_DURATION)
	app.fpsCounter.Start()
	return app
}

func (a *CYLoggerBaseAppender) GetLogType() Core.ELogType              { return a.eLogType }
func (a *CYLoggerBaseAppender) GetChannel() string                      { return a.szChannel }
func (a *CYLoggerBaseAppender) GetFile() string                        { return a.szFile }
func (a *CYLoggerBaseAppender) GetFileMode() Core.ELogFileMode         { return a.eFileMode }
func (a *CYLoggerBaseAppender) GetLogPath() string                        { return a.szLogPath }
func (a *CYLoggerBaseAppender) SetLogPath(szLogPath string)               { a.szLogPath = szLogPath }
func (a *CYLoggerBaseAppender) IsEnable() bool                        { return a.bEnable.Load() }
func (a *CYLoggerBaseAppender) SetEnable(bEnable bool)                 { a.bEnable.Store(bEnable) }
func (a *CYLoggerBaseAppender) GetLayout() Layout.ICYLoggerTemplateLayout { return a.layout }
func (a *CYLoggerBaseAppender) SetLayout(pLayout Layout.ICYLoggerTemplateLayout) { a.layout = pLayout }
func (a *CYLoggerBaseAppender) GetFilter() *Filter.ICYLoggerPatternFilter   { return a.filter }
func (a *CYLoggerBaseAppender) SetFilter(pFilter *Filter.ICYLoggerPatternFilter) { a.filter = pFilter }
func (a *CYLoggerBaseAppender) GetFPSCounter() *Common.CYFPSCounter  { return a.fpsCounter }
func (a *CYLoggerBaseAppender) GetQueueSize() int                     { return len(a.PublicQueue) + len(a.PrivateQueue) }

// GetId returns the log-type id of this appender.
func (a *CYLoggerBaseAppender) GetId() Core.ELogType { return a.eLogType }

// GetLogName returns the active backing-store name. The base (in-memory) appender
// has no file, so it returns an empty string. File/Main appenders override this.
func (a *CYLoggerBaseAppender) GetLogName() string { return "" }

// ForceNewFile is a no-op for in-memory appenders. File/Main appenders override it.
func (a *CYLoggerBaseAppender) ForceNewFile() {}

// GetSize returns the size in bytes of the backing store. The base appender has
// no file, so it returns 0.
func (a *CYLoggerBaseAppender) GetSize() int64 { return 0 }

// Copy copies the backing store to target. No-op for in-memory appenders.
func (a *CYLoggerBaseAppender) Copy(target string) {}

// ClearContents truncates the backing store. No-op for in-memory appenders.
func (a *CYLoggerBaseAppender) ClearContents() {}

func (a *CYLoggerBaseAppender) Init() bool { return true }

func (a *CYLoggerBaseAppender) UnInit() {
	// Stop the goroutine and swap ticker.
	a.Stop()
	if a.swapTick != nil {
		a.swapTick.Stop()
	}
	// Close the queues exactly once, even if UnInit is invoked more than once
	// (e.g. via both Flush and UnInit paths) or concurrently. Closing an already
	// closed channel panics, so this guard is essential for safe shutdown.
	a.closeOnce.Do(func() {
		if a.PublicQueue != nil {
			close(a.PublicQueue)
			a.PublicQueue = nil
		}
		if a.PrivateQueue != nil {
			close(a.PrivateQueue)
			a.PrivateQueue = nil
		}
	})
}

func (a *CYLoggerBaseAppender) Flush() {
	// Ask the consumer goroutine to drain synchronously and wait for it to
	// acknowledge. The handshake channel is unbuffered, so if the consumer is
	// not currently waiting (e.g. it has already stopped) the send does not
	// block and we fall back to draining directly on this goroutine. This
	// replaces the previous flushCond-based wait, which could deadlock because
	// the condition was never signalled.
	if a.IsRunning() {
		done := make(chan struct{})
		select {
		case a.flushCh <- done:
			<-done
			return
		default:
			a.processMessages()
			return
		}
	}
	a.processMessages()
}

// Write enqueues a message to the front (public) queue.
// The message is cloned because the original is owned by the caller (which
// releases it after routing). Each clone is released exactly once by the
// consumer in writeMessage, so there is never a double-free.
// Returns true if the message was accepted, false if the queue is full.
func (a *CYLoggerBaseAppender) Write(pMsg *Common.CYBaseMessage) bool {
	if !a.bEnable.Load() || pMsg == nil {
		return false
	}
	clone := pMsg.Clone()
	select {
	case a.PublicQueue <- clone:
		return true
	default:
		Common.ReleaseBaseMessage(clone)
		return false
	}
}

// swapLoop periodically signals the consumer to swap front/back buffers.
func (a *CYLoggerBaseAppender) swapLoop() {
	a.swapTick = time.NewTicker(time.Second)
	defer a.swapTick.Stop()
	stats := Statistics.GetCYStatisticsInstance()
	for a.IsRunning() {
		select {
		case <-a.swapTick.C:
			// Snapshot the live queue lengths and push them to the global
			// statistics (mirrors C++ appender::UpdatePublicStats /
			// UpdatePrivateStats). The read is taken under a.mu to avoid racing
			// with the pointer swap performed in swapAndProcess.
			a.mu.Lock()
			pub := uint32(len(a.PublicQueue))
			pri := uint32(len(a.PrivateQueue))
			a.mu.Unlock()
			stats.ReportQueueLengths(a.eLogType, pub, pri)
			select {
			case a.swapCh <- struct{}{}:
			default:
			}
		}
	}
}

// Run is the main message processing loop.  It consumes from the front
// (PublicQueue) and periodically swaps to drain the accumulated backlog
// from the back (PrivateQueue) lock-free.
func (a *CYLoggerBaseAppender) Run() {
	go a.swapLoop()
	for a.IsRunning() {
		select {
		case msg, ok := <-a.PublicQueue:
			if !ok {
				a.drainPrivateQueue()
				return
			}
			a.handleMessage(msg)
		case <-a.swapCh:
			a.swapAndProcess()
		case done := <-a.flushCh:
			a.swapAndProcess()
			a.processMessages()
			close(done)
		}
	}
}

// swapAndProcess atomically swaps front/back queues and drains the old
// front (now PrivateQueue).  Producers continue writing to the new
// PublicQueue without contention.
func (a *CYLoggerBaseAppender) swapAndProcess() {
	a.mu.Lock()
	a.PublicQueue, a.PrivateQueue = a.PrivateQueue, a.PublicQueue
	a.mu.Unlock()
	a.drainPrivateQueue()
}

// drainPrivateQueue consumes all pending messages from the back queue.
func (a *CYLoggerBaseAppender) drainPrivateQueue() {
	for {
		select {
		case msg, ok := <-a.PrivateQueue:
			if !ok {
				return
			}
			a.writeMessage(msg)
		default:
			return
		}
	}
}

// processMessages drains the PublicQueue synchronously (used during shutdown).
func (a *CYLoggerBaseAppender) processMessages() {
	for {
		select {
		case msg, ok := <-a.PublicQueue:
			if !ok {
				return
			}
			a.writeMessage(msg)
		default:
			return
		}
	}
}

func (a *CYLoggerBaseAppender) handleMessage(msg *Common.CYBaseMessage) {
	a.writeMessage(msg)
}

func (a *CYLoggerBaseAppender) writeMessage(msg *Common.CYBaseMessage) {
	defer Common.ReleaseBaseMessage(msg)
	a.fpsCounter.Increment()
	formatted := a.formatMessage(msg)
	a.doWrite(formatted)
}

func (a *CYLoggerBaseAppender) formatMessage(msg *Common.CYBaseMessage) string {
	if a.layout == nil {
		return msg.StrMsg + "\n"
	}
	// Honour a per-message channel (set via the channel-aware WriteLog*Ch
	// methods, mirroring C++ WriteLog's szChannel). When a message carries no
	// channel of its own, fall back to the appender's channel so existing
	// behaviour (where the appender channel is rendered) is preserved.
	ch := msg.StrChannel
	if ch == "" {
		ch = a.szChannel
	}
	t := msg.Time
	return a.layout.GetFormatMessage(
		ch,
		Core.ELogType(msg.EMsgType),
		msg.NSeverCode,
		msg.StrMsg,
		msg.StrFile,
		msg.StrFunc,
		msg.NLine,
		msg.NProcessId,
		msg.NThreadId,
		t.Year(), int(t.Month()), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/int(time.Millisecond),
		a.filter != nil,
	)
}

func (a *CYLoggerBaseAppender) doWrite(msg string) {
	fmt.Fprint(os.Stdout, msg)
}

// CYLoggerConsoleAppender writes logs to the console with ANSI colors.
type CYLoggerConsoleAppender struct {
	*CYLoggerBaseAppender
}

func newConsoleAppender(eLogType Core.ELogType, szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode) *CYLoggerConsoleAppender {
	app := &CYLoggerConsoleAppender{
		CYLoggerBaseAppender: newBaseAppender(eLogType, szChannel, szFile, szLogPath, eFileMode),
	}
	app.SetEnable(true)
	app.Start(app.Run)
	return app
}

func (a *CYLoggerConsoleAppender) Init() bool { return true }

func (a *CYLoggerConsoleAppender) doWrite(msg string) {
	color := Common.GetLogColor(int(a.GetLogType()))
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	fmt.Fprintf(os.Stdout, "%s%s%s\n", color, msg, Common.ColorReset)
}

func (a *CYLoggerConsoleAppender) SetEnable(bEnable bool) {
	a.bEnable.Store(bEnable)
}

// ClearConsole clears the console screen (mirrors C++ ClearConsole). The actual
// clear routine is platform-specific (native Win32 on Windows, ANSI on others).
func (a *CYLoggerConsoleAppender) ClearConsole() {
	clearConsoleNative()
}

// AllocConsoleWindow allocates (or frees) a dedicated Windows console window for
// this appender, mirroring C++ CYLoggerConsoleAppender's bWindow behaviour. On
// non-Windows platforms this is a no-op.
func (a *CYLoggerConsoleAppender) AllocConsoleWindow(bShow bool, title string) {
	allocConsoleWindow(bShow, title)
}

var g_ConsoleAppender atomic.Value // stores *CYLoggerConsoleAppender

func GetConsoleAppender() *CYLoggerConsoleAppender {
	v := g_ConsoleAppender.Load()
	if v == nil {
		return nil
	}
	return v.(*CYLoggerConsoleAppender)
}

func SetConsoleAppender(app *CYLoggerConsoleAppender) {
	g_ConsoleAppender.Store(app)
}

// CYLoggerFileAppender writes logs to rotating files.
type CYLoggerFileAppender struct {
	*CYLoggerBaseAppender
	mu             sync.Mutex
	file           *os.File
	szCurrentFile  string
	restriction    *Common.CYFileRestriction
	timeRotator    *time.Ticker
	stopCh         chan struct{}
	statsLine      atomic.Int64
	statsByte      atomic.Int64
}

func newFileAppender(eLogType Core.ELogType, szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode) *CYLoggerFileAppender {
	app := &CYLoggerFileAppender{
		CYLoggerBaseAppender: newBaseAppender(eLogType, szChannel, szFile, szLogPath, eFileMode),
		restriction:          Common.NewCYFileRestriction(),
		stopCh:              make(chan struct{}),
	}
	return app
}

func (a *CYLoggerFileAppender) Init() bool {
	if err := a.openFile(); err != nil {
		return false
	}
	a.timeRotator = time.NewTicker(time.Minute)
	go func() {
		for {
			select {
			case <-a.timeRotator.C:
				a.checkTimeRotation()
			case <-a.stopCh:
				return
			}
		}
	}()
	return true
}

func (a *CYLoggerFileAppender) UnInit() {
	a.Stop()
	close(a.stopCh)
	if a.timeRotator != nil {
		a.timeRotator.Stop()
	}
	a.Flush()
	// Close the queues exactly once (shared closeOnce with the embedded base).
	a.closeOnce.Do(func() {
		if a.PublicQueue != nil {
			close(a.PublicQueue)
			a.PublicQueue = nil
		}
		if a.PrivateQueue != nil {
			close(a.PrivateQueue)
			a.PrivateQueue = nil
		}
	})
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file != nil {
		a.file.Close()
		a.file = nil
	}
}

func (a *CYLoggerFileAppender) openFile() error {
	pc := Common.GetCYPublicFunctionInstance()
	pc_ := Common.GetCYPathConvertInstance()

	fileName := a.szFile
	if fileName == "" || fileName == "." {
		// LogFileModeAppend keeps a single stable file name; LogFileModeTime uses a
		// timestamped name that rolls on each start, matching the C++ file modes.
		if a.eFileMode == Core.LogFileModeAppend {
			fileName = pc_.GetFixedLogFileName(a.szChannel, int(a.eLogType))
		} else {
			fileName = pc_.GetLogFileName(a.szChannel, int(a.eLogType))
		}
	}

	dir := filepath.Dir(fileName)
	baseName := filepath.Base(fileName)
	if dir == "." || dir == "" {
		dir = a.szLogPath
	}
	if dir == "" {
		dir = "."
	}
	dir = pc_.ConvertLogPath(dir)

	if err := pc.MakeDir(dir); err != nil {
		return err
	}
	fullPath := filepath.Join(dir, baseName)
	a.szCurrentFile = fullPath

	mode := os.O_APPEND | os.O_CREATE | os.O_WRONLY
	if !pc.IsFileExist(fullPath) {
		mode = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	}

	f, err := os.OpenFile(fullPath, mode, 0644)
	if err != nil {
		return err
	}
	a.file = f
	return nil
}

func (a *CYLoggerFileAppender) checkTimeRotation() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file == nil {
		return
	}
	info, err := a.file.Stat()
	if err != nil || info == nil {
		return
	}
	// Honour the master detection switch (LOG_LIMIT_ENABLE): when disabled, the
	// periodic (per-file-size) rotation must not fire either.
	if a.restriction.IsEnableCheck() && info.Size() >= int64(a.restriction.GetCheckFileSize()) {
		a.rotateFileLocked()
	}
}

func (a *CYLoggerFileAppender) rotateFileLocked() {
	if a.file != nil {
		a.file.Close()
		a.file = nil
	}
	// Rotate the current file to a timestamped archive name so that multiple
	// generations accumulate. The cleanup policy (see Schedule) then enforces
	// per-type file count and total size limits across those generations.
	if a.szCurrentFile != "" {
		if rotated := a.rotatedName(); rotated != a.szCurrentFile {
			_ = os.Rename(a.szCurrentFile, rotated)
		}
	}
	// Reopen: the original name no longer exists, so a fresh empty file is created.
	a.openFile()
}

// rotatedName returns a timestamped variant of the current log file name.
func (a *CYLoggerFileAppender) rotatedName() string {
	dir := filepath.Dir(a.szCurrentFile)
	base := filepath.Base(a.szCurrentFile)
	ext := filepath.Ext(base)
	name := base
	if ext != "" {
		name = base[:len(base)-len(ext)]
	}
	return filepath.Join(dir, name+"."+time.Now().Format("20060102_150405.000000")+ext)
}

func (a *CYLoggerFileAppender) Write(pMsg *Common.CYBaseMessage) bool {
	if !a.bEnable.Load() || pMsg == nil {
		return false
	}
	a.fpsCounter.Increment()
	formatted := a.formatMessage(pMsg)
	a.doWrite(formatted)
	return true
}

func (a *CYLoggerFileAppender) doWrite(msg string) {
	if a.file == nil {
		return
	}
	if a.restriction.IsEnableCheck() && a.restriction.CheckFileSize(a.szCurrentFile) {
		a.rotateFileLocked()
	}
	if a.file == nil {
		return
	}
	n, err := a.file.WriteString(msg)
	if err == nil {
		a.statsLine.Add(1)
		a.statsByte.Add(int64(n))
	}
}

func (a *CYLoggerFileAppender) GetCurrentFile() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.szCurrentFile
}

// GetLogName returns the active log file path (mirrors C++ GetLogName).
func (a *CYLoggerFileAppender) GetLogName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.szCurrentFile
}

// GetSize returns the size in bytes of the current log file (mirrors C++ GetSize).
func (a *CYLoggerFileAppender) GetSize() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file == nil {
		return 0
	}
	if info, err := a.file.Stat(); err == nil {
		return info.Size()
	}
	return 0
}

// ForceNewFile closes the current file and rotates to a fresh timestamped file
// synchronously, so callers can rely on a new file being active on return. This
// mirrors C++ ForceNewFile which uses a promise/future to block until the swap.
func (a *CYLoggerFileAppender) ForceNewFile() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rotateFileLocked()
}

// Copy duplicates the current log file to target, then reopens the original
// (mirrors C++ Copy). The original file is kept open for continued writing.
func (a *CYLoggerFileAppender) Copy(target string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file != nil {
		a.file.Close()
		a.file = nil
	}
	if a.szCurrentFile != "" {
		pc := Common.GetCYPublicFunctionInstance()
		_ = pc.CopyFile(a.szCurrentFile, target)
	}
	a.openFile()
}

// ClearContents truncates the current log file to zero bytes (mirrors C++ ClearContents).
func (a *CYLoggerFileAppender) ClearContents() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file != nil {
		a.file.Close()
		a.file = nil
	}
	a.openFile()
}

func (a *CYLoggerFileAppender) GetRestriction() *Common.CYFileRestriction { return a.restriction }
func (a *CYLoggerFileAppender) SetRestriction(r *Common.CYFileRestriction) { a.restriction = r }
func (a *CYLoggerFileAppender) GetStatsLine() int64                     { return a.statsLine.Load() }
func (a *CYLoggerFileAppender) GetStatsByte() int64                     { return a.statsByte.Load() }

// CYLoggerMainAppender is the "main" log file appender.
type CYLoggerMainAppender struct {
	*CYLoggerFileAppender
}

func newMainAppender(szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode) *CYLoggerMainAppender {
	return &CYLoggerMainAppender{
		CYLoggerFileAppender: newFileAppender(Core.LogTypeMain, szChannel, szFile, szLogPath, eFileMode),
	}
}

func (a *CYLoggerMainAppender) Init() bool {
	return a.CYLoggerFileAppender.Init()
}

func (a *CYLoggerMainAppender) Write(pMsg *Common.CYBaseMessage) bool {
	if !a.bEnable.Load() || pMsg == nil {
		return false
	}
	a.fpsCounter.Increment()
	formatted := a.formatMessage(pMsg)
	a.doWrite(formatted)
	return true
}

func (a *CYLoggerMainAppender) doWrite(msg string) {
	if a.file == nil {
		return
	}
	if a.restriction.IsEnableCheck() && a.restriction.CheckFileSize(a.szCurrentFile) {
		a.rotateFileLocked()
	}
	if a.file == nil {
		return
	}
	n, err := a.file.WriteString(msg)
	if err == nil {
		a.statsLine.Add(1)
		a.statsByte.Add(int64(n))
	}
}

// CYLoggerBufferAppender uses multiple queues per log type, sorted by timestamp.
type CYLoggerBufferAppender struct {
	*CYLoggerBaseAppender
	queueMap   map[Core.ELogType]chan *Common.CYBaseMessage
	flushTicks *time.Ticker
}

func newBufferAppender(eLogType Core.ELogType, szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode) *CYLoggerBufferAppender {
	app := &CYLoggerBufferAppender{
		CYLoggerBaseAppender: newBaseAppender(eLogType, szChannel, szFile, szLogPath, eFileMode),
		queueMap:             make(map[Core.ELogType]chan *Common.CYBaseMessage),
	}
	for _, lt := range []Core.ELogType{Core.LogTypeDebug, Core.LogTypeTrace, Core.LogTypeInfo, Core.LogTypeWarn, Core.LogTypeError, Core.LogTypeFatal} {
		app.queueMap[lt] = make(chan *Common.CYBaseMessage, 4096)
	}
	return app
}

func (a *CYLoggerBufferAppender) Write(pMsg *Common.CYBaseMessage) bool {
	if !a.bEnable.Load() || pMsg == nil {
		return false
	}
	q, ok := a.queueMap[Core.ELogType(pMsg.EMsgType)]
	if !ok {
		return a.CYLoggerBaseAppender.Write(pMsg)
	}
	clone := pMsg.Clone()
	select {
	case q <- clone:
		return true
	default:
		Common.ReleaseBaseMessage(clone)
		return false
	}
}

func (a *CYLoggerBufferAppender) Run() {
	a.flushTicks = time.NewTicker(time.Second)
	defer a.flushTicks.Stop()

	var buffer []*Common.CYBaseMessage
	var bufMu sync.Mutex

	// Collect messages into the sort buffer instead of writing immediately, so
	// that drainAndSort can emit them in timestamp order. No busy-wait: the
	// select blocks until a message arrives or the per-second tick fires.
	appendMsg := func(msg *Common.CYBaseMessage) {
		if msg == nil {
			return
		}
		bufMu.Lock()
		buffer = append(buffer, msg)
		bufMu.Unlock()
	}

	for a.IsRunning() {
		select {
		case <-a.flushTicks.C:
			a.drainAndSort(&buffer, &bufMu)
		case msg, ok := <-a.PublicQueue:
			if !ok {
				a.drainAndSort(&buffer, &bufMu)
				a.processMessages()
				return
			}
			appendMsg(msg)
		case msg, ok := <-a.PrivateQueue:
			if !ok {
				a.drainAndSort(&buffer, &bufMu)
				a.processMessages()
				return
			}
			appendMsg(msg)
		case done := <-a.flushCh:
			a.drainAndSort(&buffer, &bufMu)
			a.processMessages()
			close(done)
		}
	}
	a.drainAndSort(&buffer, &bufMu)
	a.processMessages()
}

// reportQueueStats pushes the live length of each per-subtype queue to the global
// statistics. It mirrors C++ CYLoggerBufferAppender::UpdatePublicStats, which
// reports the size of every per-subtype public queue (Debug/Trace/Info/Warn/
// Error/Fatal) of the buffer appender.
func (a *CYLoggerBufferAppender) reportQueueStats() {
	stats := Statistics.GetCYStatisticsInstance()
	for _, lt := range []Core.ELogType{
		Core.LogTypeDebug, Core.LogTypeTrace, Core.LogTypeInfo,
		Core.LogTypeWarn, Core.LogTypeError, Core.LogTypeFatal,
	} {
		stats.ReportQueueLengths(lt, uint32(len(a.queueMap[lt])), 0)
	}
}

func (a *CYLoggerBufferAppender) drainAndSort(buffer *[]*Common.CYBaseMessage, bufMu sync.Locker) {
	bufMu.Lock()
	defer bufMu.Unlock()

	// Report the live per-subtype queue lengths on every drain (which runs on the
	// per-second tick and on flush/shutdown), keeping statistics in sync.
	a.reportQueueStats()

	for _, q := range a.queueMap {
		for {
			select {
			case msg := <-q:
				*buffer = append(*buffer, msg)
			default:
				goto nextQueue
			}
		}
	nextQueue:
	}

	sortMessages(*buffer)

	for _, msg := range *buffer {
		a.writeMessage(msg)
	}
	*buffer = (*buffer)[:0]

	a.mu.Lock()
drainPublic:
	for {
		select {
		case msg := <-a.PublicQueue:
			a.writeMessage(msg)
		default:
			break drainPublic
		}
	}
	a.mu.Unlock()
}

func sortMessages(msgs []*Common.CYBaseMessage) {
	for i := 1; i < len(msgs); i++ {
		key := msgs[i]
		j := i - 1
		for j >= 0 && msgs[j].Time.After(key.Time) {
			msgs[j+1] = msgs[j]
			j--
		}
		msgs[j+1] = key
	}
}

// remoteUDPPacketSize mirrors the C++ 900-byte UDP payload chunk used by
// CYLoggerRemoteAppender::WriteSocket.
const remoteUDPPacketSize = 900

// CYLoggerRemoteAppender sends logs to a remote host over TCP (default) or UDP
// (900-byte packets, mirroring the C++ wire format).
type CYLoggerRemoteAppender struct {
	*CYLoggerBaseAppender
	mu          struct{}
	sync.RWMutex
	remoteAddr  string
	eRemoteProto Core.ERemoteProto
	conn        net.Conn
	reconnectCh chan struct{}
	stopCh      chan struct{}
}

func newRemoteAppender(eLogType Core.ELogType, szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode, eProto Core.ERemoteProto) *CYLoggerRemoteAppender {
	app := &CYLoggerRemoteAppender{
		CYLoggerBaseAppender: newBaseAppender(eLogType, szChannel, szFile, szLogPath, eFileMode),
		remoteAddr:  szFile,
		eRemoteProto: eProto,
		reconnectCh: make(chan struct{}, 1),
		stopCh:      make(chan struct{}),
	}
	return app
}

func (a *CYLoggerRemoteAppender) connect() error {
	a.Lock()
	defer a.Unlock()
	if a.remoteAddr == "" {
		return fmt.Errorf("no remote address")
	}
	network := "tcp"
	if a.eRemoteProto == Core.RemoteProtoUDP {
		network = "udp"
	}
	conn, err := net.DialTimeout(network, a.remoteAddr, 5*time.Second)
	if err != nil {
		return err
	}
	a.conn = conn
	return nil
}

func (a *CYLoggerRemoteAppender) reconnectLoop() {
	for {
		select {
		case <-a.stopCh:
			return
		case <-a.reconnectCh:
			for i := 0; i < 3; i++ {
				if err := a.connect(); err == nil {
					return
				}
				time.Sleep(2 * time.Second)
			}
		}
	}
}

func (a *CYLoggerRemoteAppender) Init() bool {
	if err := a.connect(); err != nil {
		return false
	}
	go a.reconnectLoop()
	return true
}

func (a *CYLoggerRemoteAppender) UnInit() {
	close(a.stopCh)
	a.Lock()
	defer a.Unlock()
	if a.conn != nil {
		a.conn.Close()
		a.conn = nil
	}
}

func (a *CYLoggerRemoteAppender) doWrite(msg string) {
	a.Lock()
	defer a.Unlock()
	if a.conn == nil {
		select {
		case a.reconnectCh <- struct{}{}:
		default:
		}
		return
	}
	data := []byte(msg)
	if a.eRemoteProto == Core.RemoteProtoUDP {
		// Mirror C++ WriteSocket: chop the payload into 900-byte UDP packets.
		for len(data) > 0 {
			n := remoteUDPPacketSize
			if n > len(data) {
				n = len(data)
			}
			if _, err := a.conn.Write(data[:n]); err != nil {
				select {
				case a.reconnectCh <- struct{}{}:
				default:
				}
				return
			}
			data = data[n:]
		}
		return
	}
	// TCP framing: 4-byte big-endian length prefix followed by the payload.
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	a.conn.Write(header)
	a.conn.Write(data)
}

// ForceNewFile drops the current connection so the reconnect loop establishes a
// fresh one. For UDP (connectionless) this is effectively a no-op.
func (a *CYLoggerRemoteAppender) ForceNewFile() {
	select {
	case a.reconnectCh <- struct{}{}:
	default:
	}
}

// systemLogWriter is the platform abstraction over the OS system log.
// On Windows it writes to the Windows Event Log; on Unix it writes to syslog.
// The concrete implementations live in the build-tagged files
// eventlog_windows.go and eventlog_unix.go.
type systemLogWriter interface {
	Write(eLogType int, msg string) error
	Close() error
}

// newSystemLogWriter is provided by the platform-specific build-tagged files
// (eventlog_windows.go / eventlog_unix.go); it must not be declared here.

// CYLoggerSystemAppender writes to the OS system log:
//   - Windows: Windows Event Log (via advapi32.dll, see eventlog_windows.go)
//   - Unix:    syslog (see eventlog_unix.go)
type CYLoggerSystemAppender struct {
	Common.CYNoCopy
	eLogType    Core.ELogType
	szChannel   string
	szLogPath   string
	mu          sync.Mutex
	writer      systemLogWriter
	layout      Layout.ICYLoggerTemplateLayout
	filter      *Filter.ICYLoggerPatternFilter
	bEnable     atomic.Bool
	PublicQueue chan *Common.CYBaseMessage
	stopCh      chan struct{}
	fpsCounter  *Common.CYFPSCounter
}

func newSystemAppender(eLogType Core.ELogType, szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode) *CYLoggerSystemAppender {
	_ = szFile
	_ = eFileMode
	return &CYLoggerSystemAppender{
		eLogType:    eLogType,
		szChannel:   szChannel,
		szLogPath:   szLogPath,
		PublicQueue: make(chan *Common.CYBaseMessage, 4096),
		stopCh:      make(chan struct{}),
		fpsCounter:  Common.NewCYFPSCounter(Core.LOG_FPS_CHECK_DURATION),
	}
}

func (a *CYLoggerSystemAppender) Init() bool {
	a.mu.Lock()
	if a.writer != nil {
		a.writer.Close()
		a.writer = nil
	}
	w, err := newSystemLogWriter("CYGoLogger", a.eLogType)
	if err != nil {
		a.mu.Unlock()
		return false
	}
	a.writer = w
	a.mu.Unlock()
	a.bEnable.Store(true)
	return true
}

func (a *CYLoggerSystemAppender) UnInit() {
	close(a.stopCh)
	a.mu.Lock()
	if a.writer != nil {
		a.writer.Close()
		a.writer = nil
	}
	a.mu.Unlock()
}

func (a *CYLoggerSystemAppender) Flush() {
	for {
		select {
		case msg := <-a.PublicQueue:
			if msg != nil {
				a.writeSystemMessage(msg)
				Common.ReleaseBaseMessage(msg)
			}
		default:
			return
		}
	}
}

func (a *CYLoggerSystemAppender) Write(pMsg *Common.CYBaseMessage) bool {
	if !a.bEnable.Load() || pMsg == nil {
		return false
	}
	clone := pMsg.Clone()
	select {
	case a.PublicQueue <- clone:
		return true
	default:
		Common.ReleaseBaseMessage(clone)
		return false
	}
}

func (a *CYLoggerSystemAppender) Run() {
	for {
		select {
		case <-a.stopCh:
			// Drain any remaining messages before exiting.
			for {
				select {
				case msg := <-a.PublicQueue:
					if msg != nil {
						a.writeSystemMessage(msg)
						Common.ReleaseBaseMessage(msg)
					}
				default:
					return
				}
			}
		case msg := <-a.PublicQueue:
			if msg != nil {
				a.writeSystemMessage(msg)
				Common.ReleaseBaseMessage(msg)
			}
		}
	}
}

func (a *CYLoggerSystemAppender) writeSystemMessage(msg *Common.CYBaseMessage) {
	a.fpsCounter.Increment()
	formatted := a.formatMessage(msg)
	// Strip trailing newline; the OS system log adds its own line breaks.
	if len(formatted) > 0 && formatted[len(formatted)-1] == '\n' {
		formatted = formatted[:len(formatted)-1]
	}
	a.mu.Lock()
	w := a.writer
	a.mu.Unlock()
	if w != nil {
		_ = w.Write(int(a.eLogType), formatted)
	}
}

func (a *CYLoggerSystemAppender) formatMessage(msg *Common.CYBaseMessage) string {
	if a.layout == nil {
		return msg.StrMsg
	}
	t := msg.Time
	return a.layout.GetFormatMessage(
		a.szChannel,
		Core.ELogType(msg.EMsgType),
		msg.NSeverCode,
		msg.StrMsg,
		msg.StrFile,
		msg.StrFunc,
		msg.NLine,
		msg.NProcessId,
		msg.NThreadId,
		t.Year(), int(t.Month()), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/int(time.Millisecond),
		a.filter != nil,
	)
}

func (a *CYLoggerSystemAppender) GetLogType() Core.ELogType               { return a.eLogType }
func (a *CYLoggerSystemAppender) GetLogPath() string                      { return a.szLogPath }
func (a *CYLoggerSystemAppender) SetLogPath(szLogPath string)             { a.szLogPath = szLogPath }
func (a *CYLoggerSystemAppender) GetChannel() string                     { return a.szChannel }
func (a *CYLoggerSystemAppender) GetFile() string                        { return "" }
func (a *CYLoggerSystemAppender) GetFileMode() Core.ELogFileMode         { return Core.LogFileModeAppend }
func (a *CYLoggerSystemAppender) IsEnable() bool                         { return a.bEnable.Load() }
func (a *CYLoggerSystemAppender) SetEnable(b bool)                       { a.bEnable.Store(b) }
func (a *CYLoggerSystemAppender) GetLayout() Layout.ICYLoggerTemplateLayout { return a.layout }
func (a *CYLoggerSystemAppender) SetLayout(l Layout.ICYLoggerTemplateLayout)  { a.layout = l }
func (a *CYLoggerSystemAppender) GetFilter() *Filter.ICYLoggerPatternFilter { return a.filter }
func (a *CYLoggerSystemAppender) SetFilter(f *Filter.ICYLoggerPatternFilter) { a.filter = f }
func (a *CYLoggerSystemAppender) GetFPSCounter() *Common.CYFPSCounter    { return a.fpsCounter }
func (a *CYLoggerSystemAppender) GetQueueSize() int                      { return len(a.PublicQueue) }
func (a *CYLoggerSystemAppender) Start(run func())                      { go run() }

// GetId returns the system log type id.
func (a *CYLoggerSystemAppender) GetId() Core.ELogType { return a.eLogType }

// GetLogName returns an empty string: the system log has no local file.
func (a *CYLoggerSystemAppender) GetLogName() string { return "" }

// ForceNewFile is a no-op for the system event log.
func (a *CYLoggerSystemAppender) ForceNewFile() {}

// GetSize returns 0: the system event log has no local file size.
func (a *CYLoggerSystemAppender) GetSize() int64 { return 0 }

// Copy is a no-op for the system event log.
func (a *CYLoggerSystemAppender) Copy(target string) {}

// ClearContents is a no-op for the system event log.
func (a *CYLoggerSystemAppender) ClearContents() {}

// CYLoggerAppenderFactory creates appender instances by type.
type CYLoggerAppenderFactory struct {
	Common.CYNoCopy
	mu   sync.RWMutex
	apps map[Core.ELogType][]IAppender
}

var g_CYLoggerAppenderFactoryInstance *CYLoggerAppenderFactory
var g_CYLoggerAppenderFactoryOnce sync.Once

func GetCYLoggerAppenderFactoryInstance() *CYLoggerAppenderFactory {
	g_CYLoggerAppenderFactoryOnce.Do(func() {
		g_CYLoggerAppenderFactoryInstance = &CYLoggerAppenderFactory{
			apps: make(map[Core.ELogType][]IAppender),
		}
	})
	return g_CYLoggerAppenderFactoryInstance
}

func (f *CYLoggerAppenderFactory) CreateAppender(eLogType Core.ELogType, szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode) IAppender {
	switch eLogType {
	case Core.LogTypeNone:
		// LOG_TYPE_NONE is the "console" appender: C++ CYLoggerAppenderFactory::
		// CreateFileAppender returns a CYLoggerConsoleAppender("CryLogger",
		// GetShowConsoleWindow()) for this type. The console appender embeds
		// CYLoggerBaseAppender, so its public/private queue lengths are already
		// reported to Statistics on every swap tick (see swapLoop).
		return newConsoleAppender(eLogType, szChannel, szFile, szLogPath, eFileMode)
	case Core.LogTypeMain:
		return newMainAppender(szChannel, szFile, szLogPath, eFileMode)
	case LOG_TYPE_BUFFER:
		return newBufferAppender(eLogType, szChannel, szFile, szLogPath, eFileMode)
	case Core.LogTypeRemote:
		return newRemoteAppender(eLogType, szChannel, szFile, szLogPath, eFileMode, Core.GetCYLoggerConfigInstance().GetRemoteProto())
	case Core.LogTypeSys:
		return newSystemAppender(eLogType, szChannel, szFile, szLogPath, eFileMode)
	default:
		return newFileAppender(eLogType, szChannel, szFile, szLogPath, eFileMode)
	}
}

func (f *CYLoggerAppenderFactory) CreateConsoleAppender(eLogType Core.ELogType, szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode) *CYLoggerConsoleAppender {
	return newConsoleAppender(eLogType, szChannel, szFile, szLogPath, eFileMode)
}

func (f *CYLoggerAppenderFactory) RegisterAppender(app IAppender) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.apps[app.GetLogType()] = append(f.apps[app.GetLogType()], app)
}

func (f *CYLoggerAppenderFactory) UnregisterAppender(app IAppender) {
	f.mu.Lock()
	defer f.mu.Unlock()
	apps := f.apps[app.GetLogType()]
	for i, a := range apps {
		if a == app {
			f.apps[app.GetLogType()] = append(apps[:i], apps[i+1:]...)
			return
		}
	}
}

func (f *CYLoggerAppenderFactory) GetAppenders(eLogType Core.ELogType) []IAppender {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make([]IAppender, len(f.apps[eLogType]))
	copy(result, f.apps[eLogType])
	return result
}

func (f *CYLoggerAppenderFactory) GetAllAppenders() map[Core.ELogType][]IAppender {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make(map[Core.ELogType][]IAppender)
	for k, v := range f.apps {
		result[k] = v
	}
	return result
}

func (f *CYLoggerAppenderFactory) CloseAll() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, apps := range f.apps {
		for _, app := range apps {
			app.UnInit()
		}
	}
	f.apps = make(map[Core.ELogType][]IAppender)
}
