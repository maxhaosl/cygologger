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

// Package Appender provides all log appender implementations.
package Appender

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maxhaosl/cygologger/ICYLogger/Core"
	"github.com/maxhaosl/cygologger/ICYLogger/Filter"
	"github.com/maxhaosl/cygologger/ICYLogger/Layout"
	Common "github.com/maxhaosl/cygologger/ICYLogger/Common"
	"github.com/maxhaosl/cygologger/ICYLogger/Statistics"
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
	stopOnce        sync.Once // guarantees stopCh is closed exactly once
	wg              sync.WaitGroup
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
	stopCh       chan struct{} // signals the consumer goroutine to exit
	swapTick     *time.Ticker  // periodic swap trigger
	fpsCounter   *Common.CYFPSCounter
	layout       Layout.ICYLoggerTemplateLayout
	filter       *Filter.ICYLoggerPatternFilter
	// bLayoutComparable records (at SetLayout time) whether the layout's
	// dynamic type supports ==, so the render-cache identity check in
	// formatMessage can never panic on an exotic custom layout type.
	bLayoutComparable bool
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
		stopCh:          make(chan struct{}),
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
func (a *CYLoggerBaseAppender) SetLayout(pLayout Layout.ICYLoggerTemplateLayout) {
	a.layout = pLayout
	// Interface == on a non-comparable dynamic type panics. The built-in
	// layouts are pointer types (always comparable); probe custom layouts once
	// here so the per-line cache check in formatMessage never has to.
	a.bLayoutComparable = pLayout != nil && reflect.TypeOf(pLayout).Comparable()
}
func (a *CYLoggerBaseAppender) GetFilter() *Filter.ICYLoggerPatternFilter   { return a.filter }
func (a *CYLoggerBaseAppender) SetFilter(pFilter *Filter.ICYLoggerPatternFilter) { a.filter = pFilter }
func (a *CYLoggerBaseAppender) GetFPSCounter() *Common.CYFPSCounter  { return a.fpsCounter }
func (a *CYLoggerBaseAppender) GetQueueSize() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.PublicQueue) + len(a.PrivateQueue)
}

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

// Start launches the consumer goroutine. The WaitGroup lets UnInit block until
// Run has fully exited, so the data channels are only closed once there are no
// remaining receivers (this is what removes the close-vs-receive data race).
func (a *CYLoggerBaseAppender) Start(run func()) {
	a.wg.Add(1)
	a.CYNamedThread.Start(run)
}

// Stop signals the consumer goroutine to exit by closing stopCh, then delegates
// to the embedded CYNamedThread.Stop for the context/running flag.
func (a *CYLoggerBaseAppender) Stop() {
	a.stopOnce.Do(func() { close(a.stopCh) })
	a.CYNamedThread.Stop()
}

func (a *CYLoggerBaseAppender) Init() bool { return true }

func (a *CYLoggerBaseAppender) UnInit() {
	// Stop the consumer goroutine and wait for it to finish. This guarantees no
	// goroutine is still receiving from PublicQueue/PrivateQueue when we close
	// them below, eliminating the close-vs-receive data race caught by -race.
	// (The swap ticker is created inside swapLoop and stopped by its own
	// deferred Stop once the loop observes the goroutine is no longer running.)
	a.Stop()
	a.wg.Wait()
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
	// Snapshot the (consumer-swapped) public queue pointer under a.mu so the
	// send cannot race with swapAndProcess, which swaps the same pointers
	// under that lock. The channel object itself stays valid for the duration
	// of the send because UnInit stops the consumer before closing the queues.
	a.mu.Lock()
	pub := a.PublicQueue
	a.mu.Unlock()
	if pub == nil {
		Common.ReleaseBaseMessage(clone)
		return false
	}
	select {
	case pub <- clone:
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
		case <-a.stopCh:
			return
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
	defer a.wg.Done()
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
		case <-a.stopCh:
			// Stop requested: drain any pending messages, then exit. After this
			// returns Run no longer receives from the data channels, so UnInit
			// can close them without a close-vs-receive race.
			a.processMessages()
			return
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

// processMessages drains the PublicQueue synchronously (used during shutdown
// and as the Flush fallback when the consumer goroutine is busy). The queue
// pointer is snapshotted under a.mu so reading it cannot race with the
// consumer's swapAndProcess, which swaps the same pointers under that lock.
func (a *CYLoggerBaseAppender) processMessages() {
	a.mu.Lock()
	pub := a.PublicQueue
	a.mu.Unlock()
	if pub == nil {
		return
	}
	for {
		select {
		case msg, ok := <-pub:
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
	bEscape := a.filter != nil
	// Main double-write dedup: when the same message reaches a second file
	// appender with the SAME layout, effective channel and escape flag, the
	// rendered line is byte-identical — reuse the cached render instead of
	// paying the layout cost twice per line. The cache lives on the message,
	// which is owned by a single producer goroutine (clones copy the cache and
	// are exclusively owned by their consumer), so this is race-free.
	if a.bLayoutComparable && msg.CachedLine != "" && msg.CachedLayout == any(a.layout) &&
		msg.CachedCh == ch && msg.CachedEscape == bEscape {
		return msg.CachedLine
	}
	t := msg.Time
	out := a.layout.GetFormatMessage(
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
		bEscape,
	)
	msg.CachedLine = out
	msg.CachedLayout = a.layout
	msg.CachedCh = ch
	msg.CachedEscape = bEscape
	return out
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
	// bufw buffers writes to file (guarded by a.mu). Every line no longer costs
	// a write(2) syscall; the buffer is flushed by the periodic flusher (1s),
	// on rotation, on Flush() and on UnInit, bounding data loss on hard crash
	// to at most one flush interval.
	bufw           *bufio.Writer
	szCurrentFile  string
	// nCurrentSize tracks the current file size in memory (guarded by a.mu) so
	// the per-write size-rotation check avoids an os.Stat syscall per line.
	// It includes bytes still sitting in bufw.
	nCurrentSize   int64
	restriction    *Common.CYFileRestriction
	timeRotator    *time.Ticker
	stopCh         chan struct{}
	stopOnce       sync.Once
	statsLine      atomic.Int64
	statsByte      atomic.Int64
	// Producer-side batch buffer (double-buffered). Producers format the
	// message on their own goroutine (layout rendering runs fully in parallel)
	// and append the final string under pmu — a sub-microsecond critical
	// section with NO per-message consumer wakeup. The writer goroutine (Run)
	// swaps the buffer on a short tick and writes the whole batch with the
	// bufio writer. CPU profiling showed a per-message channel send spent ~33%
	// of total CPU in pthread_cond_signal waking the parked consumer; the
	// tick-driven batch swap removes that wakeup entirely, which is what makes
	// producer throughput scale with goroutine count.
	pmu       sync.Mutex
	pending   []string   // producers append; swapped out by writePending
	spare     []string   // recycled batch storage (double buffer)
	spaceCond *sync.Cond // backpressure: producers wait when pending is full
	// bClosedForWrite is set (under pmu) on shutdown so blocked producers wake
	// up and stop enqueuing instead of waiting forever.
	bClosedForWrite bool
	// notifyCh carries at most one "data available" token; a non-blocking send
	// after append wakes the writer only when it is actually parked, so the
	// wakeup cost is amortised over whole batches instead of per line.
	notifyCh chan struct{}
}

// fileAppenderPendingCap bounds the producer-side batch buffer. When full,
// producers block (backpressure) instead of dropping lines.
const fileAppenderPendingCap = 1 << 15

func newFileAppender(eLogType Core.ELogType, szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode) *CYLoggerFileAppender {
	app := &CYLoggerFileAppender{
		CYLoggerBaseAppender: newBaseAppender(eLogType, szChannel, szFile, szLogPath, eFileMode),
		restriction:          Common.NewCYFileRestriction(),
		stopCh:              make(chan struct{}),
		pending:             make([]string, 0, 1024),
		spare:               make([]string, 0, 1024),
	}
	app.spaceCond = sync.NewCond(&app.pmu)
	app.notifyCh = make(chan struct{}, 1)
	return app
}

func (a *CYLoggerFileAppender) Init() bool {
	if err := a.openFile(); err != nil {
		return false
	}
	a.timeRotator = time.NewTicker(time.Minute)
	go func() {
		// Periodic buffer flush keeps buffered lines visible on disk within
		// ~1s even under low traffic, without paying a syscall per line.
		flushTicker := time.NewTicker(time.Second)
		defer flushTicker.Stop()
		for {
			select {
			case <-a.timeRotator.C:
				a.checkTimeRotation()
			case <-flushTicker.C:
				a.flushBuffer()
			case <-a.stopCh:
				return
			}
		}
	}()
	return true
}

// flushBuffer flushes the write buffer to the OS (no fsync).
func (a *CYLoggerFileAppender) flushBuffer() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.bufw != nil {
		a.bufw.Flush()
	}
}

// Flush performs a BLOCKING handshake with the writer goroutine: the writer
// drains writeCh and flushes the bufio buffer before acknowledging, so every
// line enqueued before Flush is visible in the file on return. If the writer
// has already stopped (shutdown path), the drain+flush runs on this goroutine.
// A non-blocking send here would silently skip the drain whenever the writer
// was busy, breaking the "write then read the file" contract tests rely on.
func (a *CYLoggerFileAppender) Flush() {
	if a.IsRunning() {
		done := make(chan struct{})
		select {
		case a.flushCh <- done:
			<-done
			return
		case <-a.stopCh:
			// Writer goroutine is exiting: it drains the pending batch itself;
			// fall through to flush whatever reached the buffer.
		}
	} else {
		// Writer goroutine never started (e.g. appender used stand-alone in
		// tests) or already stopped: drain pending lines on this goroutine
		// so a blocking handshake can never deadlock.
		a.writePending()
	}
	a.flushBuffer()
}

func (a *CYLoggerFileAppender) UnInit() {
	// Stop the time-rotation ticker goroutine and signal the writer goroutine
	// to drain the pending batch and exit.
	a.stopOnce.Do(func() { close(a.stopCh) })
	if a.timeRotator != nil {
		a.timeRotator.Stop()
	}
	// Wake producers blocked on backpressure so they stop enqueuing; lines
	// already appended stay in pending and are drained by Run on exit.
	a.pmu.Lock()
	a.bClosedForWrite = true
	a.spaceCond.Broadcast()
	a.pmu.Unlock()
	// Delegate to the base, which waits (via its WaitGroup) for the writer
	// goroutine (Run) to finish draining the batch and exit before closing the
	// legacy queues — no line enqueued before UnInit is lost.
	a.CYLoggerBaseAppender.UnInit()
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closeFileLocked()
}

// closeFileLocked flushes the buffer and closes the file. Caller holds a.mu.
func (a *CYLoggerFileAppender) closeFileLocked() {
	if a.bufw != nil {
		a.bufw.Flush()
		a.bufw = nil
	}
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
	a.bufw = bufio.NewWriterSize(f, 64*1024)
	// Seed the in-memory size counter once per open; subsequent writes update
	// it incrementally so doWrite never needs an os.Stat per line.
	a.nCurrentSize = 0
	if info, err := f.Stat(); err == nil {
		a.nCurrentSize = info.Size()
	}
	return nil
}

func (a *CYLoggerFileAppender) checkTimeRotation() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.file == nil {
		return
	}
	// Honour the master detection switch (LOG_LIMIT_ENABLE): when disabled, the
	// periodic (per-file-size) rotation must not fire either. nCurrentSize
	// includes buffered bytes, so no Stat syscall is needed.
	if a.restriction.IsEnableCheck() && a.restriction.IsCreateNewLog(a.nCurrentSize) {
		a.rotateFileLocked()
	}
}

func (a *CYLoggerFileAppender) rotateFileLocked() {
	a.closeFileLocked()
	// Rotate the current file to a timestamped archive name so that multiple
	// generations accumulate. The cleanup policy (see Schedule) then enforces
	// per-type file count and total size limits across those generations.
	if a.szCurrentFile != "" {
		if rotated := a.rotatedName(); rotated != a.szCurrentFile {
			if err := os.Rename(a.szCurrentFile, rotated); err != nil {
				Common.GetCYExceptionLogFileInstance().WriteLog(
					fmt.Sprintf("log rotation failed: %s -> %s: %v", a.szCurrentFile, rotated, err),
					"appender", "rotateFileLocked", 0)
			}
		}
	}
	// Reopen: the original name no longer exists, so a fresh empty file is created.
	a.openFile()
}

// rotatedName returns a unique archive name for the current log file.
//
// In LogFileModeAppend the active file has a fixed name (e.g. "Info.log"), so a
// timestamp suffix produces a readable, unique archive: "Info.20060102_150405.000000.log".
//
// In LogFileModeTime the active file name ALREADY carries the start timestamp
// (e.g. "Info_20060102_150405.log", see GetLogFileName). Appending a second full
// timestamp there yielded the double-timestamp bug
// ("Info_20060102_150405.20060102_150406.000000.log"). To keep the name readable
// and unique we instead append a short, monotonically increasing sequence suffix
// (".1", ".2", ...) that never collides with an existing file in the directory.
func (a *CYLoggerFileAppender) rotatedName() string {
	dir := filepath.Dir(a.szCurrentFile)
	base := filepath.Base(a.szCurrentFile)
	ext := filepath.Ext(base)
	name := base
	if ext != "" {
		name = base[:len(base)-len(ext)]
	}

	// LogFileModeTime: the base already ends with a start timestamp; adding
	// another one duplicates it. Use an incrementing sequence suffix instead,
	// probing the directory so we never overwrite an existing archive.
	if a.eFileMode == Core.LogFileModeTime {
		for seq := 1; ; seq++ {
			candidate := filepath.Join(dir, fmt.Sprintf("%s.%d%s", name, seq, ext))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				return candidate
			}
		}
	}

	// LogFileModeAppend (fixed base name): a timestamp suffix is both unique
	// and human-readable.
	return filepath.Join(dir, name+"."+time.Now().Format("20060102_150405.000000")+ext)
}

// Write formats the message on the CALLER's goroutine (layout rendering is
// CPU-bound and embarrassingly parallel) and appends the final string to the
// producer-side batch buffer under a sub-microsecond mutex — no per-message
// consumer wakeup. When the buffer is full it BLOCKS (backpressure): a line is
// never dropped. The writer goroutine (Run) swaps the batch out and performs
// the buffered disk writes, preserving per-file ordering.
func (a *CYLoggerFileAppender) Write(pMsg *Common.CYBaseMessage) bool {
	if !a.bEnable.Load() || pMsg == nil {
		return false
	}
	a.fpsCounter.Increment()
	formatted := a.formatMessage(pMsg)

	a.pmu.Lock()
	for len(a.pending) >= fileAppenderPendingCap && !a.bClosedForWrite {
		a.spaceCond.Wait()
	}
	if a.bClosedForWrite {
		a.pmu.Unlock()
		return false
	}
	a.pending = append(a.pending, formatted)
	a.pmu.Unlock()

	// Non-blocking token: wakes the writer only if it is parked; when it is
	// already busy the token (or a previous one) is still pending and this is
	// a no-op, so the expensive futex wake is paid once per batch, not per line.
	select {
	case a.notifyCh <- struct{}{}:
	default:
	}
	return true
}

// Run is the per-file writer goroutine. It is the only goroutine that touches
// the file handle / bufio writer, so per-file ordering is guaranteed while
// producers run fully concurrently. It also honours the Flush handshake and the
// shutdown signal.
func (a *CYLoggerFileAppender) Run() {
	defer a.wg.Done()
	for {
		select {
		case <-a.notifyCh:
			a.writePending()
		case done := <-a.flushCh:
			a.writePending()
			a.flushBuffer()
			close(done)
		case <-a.stopCh:
			a.writePending()
			return
		}
	}
}

// writePending swaps out the producer batch (double-buffer) and writes every
// line under a single file-mutex acquisition. Only the writer goroutine (or
// Flush when the writer is not running) calls this, so the swap is sequential.
func (a *CYLoggerFileAppender) writePending() {
	a.pmu.Lock()
	if len(a.pending) == 0 {
		a.pmu.Unlock()
		return
	}
	batch := a.pending
	a.pending = a.spare[:0]
	a.spare = batch // storage recycled on the next swap (after this write completes)
	a.spaceCond.Broadcast()
	a.pmu.Unlock()

	a.mu.Lock()
	for _, line := range batch {
		a.writeLineLocked(line)
	}
	a.mu.Unlock()
	// Drop string references so the batch storage does not pin written lines.
	for i := range batch {
		batch[i] = ""
	}
}

// doWrite writes a single line under the file mutex (kept for direct callers
// such as tests; the hot path is writePending, which batches under one lock).
func (a *CYLoggerFileAppender) doWrite(msg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.writeLineLocked(msg)
}

// writeLineLocked appends one line to the bufio writer, rotating first when the
// size policy demands it. Caller holds a.mu. The in-memory size counter avoids
// an os.Stat syscall per line.
func (a *CYLoggerFileAppender) writeLineLocked(msg string) {
	if a.file == nil {
		return
	}
	if a.restriction.IsEnableCheck() && a.restriction.IsCreateNewLog(a.nCurrentSize) {
		a.rotateFileLocked()
	}
	if a.bufw == nil {
		return
	}
	n, err := a.bufw.WriteString(msg)
	if err == nil {
		a.nCurrentSize += int64(n)
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
	// nCurrentSize includes bytes still in the write buffer, so it reflects
	// the logical file size without a Stat syscall.
	return a.nCurrentSize
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
	a.closeFileLocked()
	if a.szCurrentFile != "" {
		pc := Common.GetCYPublicFunctionInstance()
		if err := pc.CopyFile(a.szCurrentFile, target); err != nil {
			Common.GetCYExceptionLogFileInstance().WriteLog(
				fmt.Sprintf("log copy failed: %s -> %s: %v", a.szCurrentFile, target, err),
				"appender", "Copy", 0)
		}
	}
	a.openFile()
}

// ClearContents truncates the current log file to zero bytes (mirrors C++ ClearContents).
func (a *CYLoggerFileAppender) ClearContents() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.bufw != nil {
		// Discard buffered content along with the file contents.
		a.bufw = nil
	}
	if a.file != nil {
		a.file.Close()
		a.file = nil
	}
	if a.szCurrentFile != "" {
		os.Remove(a.szCurrentFile)
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

// Main inherits CYLoggerFileAppender.Write / Run / writePending, so it shares
// the same async batch-writer pipeline (format-on-producer -> batch swap ->
// single writer goroutine).

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
	defer a.wg.Done()
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
		case <-a.stopCh:
			// Stop requested: drain pending messages, then exit. Run no longer
			// receives from the data channels, so UnInit can close them safely.
			a.drainAndSort(&buffer, &bufMu)
			a.processMessages()
			return
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
	sync.RWMutex
	remoteAddr  string
	eRemoteProto Core.ERemoteProto
	conn        net.Conn
	reconnectCh chan struct{}
	stopCh      chan struct{}
	stopOnce    sync.Once
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
	// Stop the reconnect-loop goroutine.
	a.stopOnce.Do(func() { close(a.stopCh) })
	// Delegate data-channel draining / goroutine stop to the base.
	a.CYLoggerBaseAppender.UnInit()
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
	stopOnce    sync.Once
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
	w, err := newSystemLogWriter("cygologger", a.eLogType)
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
	a.stopOnce.Do(func() { close(a.stopCh) })
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

// Reset clears every registered appender. It is called on logger shutdown so a
// subsequent InitDefaultWithOpts in the same process starts from a clean slate
// (otherwise previously-mounted remote/sys/console appenders would leak across
// re-initializations and make config options like LOG_WRITE_REMOTE=false appear
// ineffective). The factory singleton itself is preserved; only its registry is
// emptied.
func (f *CYLoggerAppenderFactory) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.apps = make(map[Core.ELogType][]IAppender)
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
