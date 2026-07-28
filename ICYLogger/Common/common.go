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

// Package Common provides core utilities, threading primitives, and message types
// used throughout the logging library.
package Common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// CYNoCopy - non-copyable marker
// ============================================================================

// CYNoCopy is a non-copyable marker embedded to prevent accidental copies.
type CYNoCopy struct{}

func (c *CYNoCopy) Lock()   {}
func (c *CYNoCopy) Unlock() {}

// ============================================================================
// CYPrivateDefine - private constants
// ============================================================================

const (
	LogDir                       = "Log"
	LogSeparator                 = " "
	LogEscapeChar                = '\\'
	LogHeaderStart               = '['
	LogHeaderEnd                 = ']'
	LogFieldNameEnd              = '='
	LogFieldValueEnd             = '|'
	LogExtensionFieldValueEnd    = '#'
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
	ColorGray   = "\033[37m"
	ColorWhite  = "\033[97m"

	ColorTrace = "\033[36m"
	ColorDebug = "\033[34m"
	ColorInfo  = "\033[32m"
	ColorWarn  = "\033[33m"
	ColorError = "\033[31m"
	ColorFatal = "\033[35m"
	ColorMain  = "\033[36m"
	ColorSys   = "\033[35m"
)

func GetLogColor(eLogType int) string {
	switch eLogType {
	case 1:
		return ColorTrace
	case 2:
		return ColorDebug
	case 3:
		return ColorInfo
	case 4:
		return ColorWarn
	case 5:
		return ColorError
	case 6:
		return ColorFatal
	case 7:
		return ColorMain
	case 9:
		return ColorSys
	default:
		return ColorReset
	}
}

const (
	DefaultMaxFileSize  = 5 * 1024 * 1024
	DefaultMaxFileCount = 20
	DefaultMaxTotalSize = 1024 * 1024 * 1024
)

// ============================================================================
// CYTimeElapsed - elapsed time measurement
// ============================================================================

type CYTimeElapsed struct {
	startTime time.Time
}

func NewCYTimeElapsed() *CYTimeElapsed {
	return &CYTimeElapsed{startTime: time.Now()}
}

func (e *CYTimeElapsed) Reset() {
	e.startTime = time.Now()
}

func (e *CYTimeElapsed) Elapsed() time.Duration {
	return time.Since(e.startTime)
}

func (e *CYTimeElapsed) ElapsedNanoseconds() int64 {
	return time.Since(e.startTime).Nanoseconds()
}

func (e *CYTimeElapsed) ElapsedMicroseconds() int64 {
	return time.Since(e.startTime).Microseconds()
}

func (e *CYTimeElapsed) ElapsedMilliseconds() int64 {
	return time.Since(e.startTime).Milliseconds()
}

func (e *CYTimeElapsed) ElapsedSeconds() float64 {
	return time.Since(e.startTime).Seconds()
}

func (e *CYTimeElapsed) ElapsedMinutes() float64 {
	return time.Since(e.startTime).Minutes()
}

func (e *CYTimeElapsed) ElapsedHours() float64 {
	return time.Since(e.startTime).Hours()
}

// ============================================================================
// CYTimeStamps - timestamp formatting
// ============================================================================

// CYTimeStamps formats timestamps. fmt.Sprintf is a pure function with no
// shared state, so no locking is required; the previous global sync.Mutex
// serialised every log line's timestamp formatting across all goroutines.
type CYTimeStamps struct{}

var g_CYTimeStampsInstance *CYTimeStamps
var g_CYTimeStampsOnce sync.Once

func GetCYTimeStampsInstance() *CYTimeStamps {
	g_CYTimeStampsOnce.Do(func() {
		g_CYTimeStampsInstance = &CYTimeStamps{}
	})
	return g_CYTimeStampsInstance
}

func (ts *CYTimeStamps) GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d.%03d", nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
}

func (ts *CYTimeStamps) GetTimeStampsFile(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	return fmt.Sprintf("%04d%02d%02d%02d%02d%02d%03d", nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
}

func (ts *CYTimeStamps) GetTimeStampsShort(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	return fmt.Sprintf("%02d:%02d:%02d.%03d", nHR, nMN, nSC, nMMN)
}

// ============================================================================
// CYNamedThread - goroutine wrapper
// ============================================================================

type CYNamedThread struct {
	CYNoCopy
	name     string
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	bRunning atomic.Bool
}

func NewCYNamedThread(name string) *CYNamedThread {
	return &CYNamedThread{name: name}
}

func (t *CYNamedThread) GetName() string     { return t.name }
func (t *CYNamedThread) SetThreadName(name string) { t.name = name }
func (t *CYNamedThread) GetThreadId() uint64 { return GetGID() }
func (t *CYNamedThread) IsRunning() bool     { return t.bRunning.Load() }

func (t *CYNamedThread) Start(run func()) {
	t.mu.Lock()
	if t.bRunning.Load() {
		t.mu.Unlock()
		return
	}
	t.ctx, t.cancel = context.WithCancel(context.Background())
	t.bRunning.Store(true)
	t.mu.Unlock()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Route background-goroutine panics into the exception log
				// channel so they are captured with a full stack trace
				// instead of being silently swallowed.
				GetCYExceptionLogFileInstance().handlePanic(r)
			}
			t.bRunning.Store(false)
		}()
		run()
	}()
}

func (t *CYNamedThread) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	t.bRunning.Store(false)
}

// ============================================================================
// CYNamedLocker - named mutex
// ============================================================================

type CYNamedLocker struct {
	CYNoCopy
	name  string
	mutex sync.Mutex
}

func NewCYNamedLocker(name string) *CYNamedLocker {
	return &CYNamedLocker{name: name}
}

func (l *CYNamedLocker) Lock()          { l.mutex.Lock() }
func (l *CYNamedLocker) Unlock()       { l.mutex.Unlock() }
func (l *CYNamedLocker) GetName() string { return l.name }

// ============================================================================
// CYNamedCondition - named condition variable
// ============================================================================

type CYNamedCondition struct {
	CYNoCopy
	name string
	cond *sync.Cond
}

// ERetCode mirrors the C++ RetCode returned by WaitForEvent. It is defined here
// (rather than in Core) to avoid an import cycle with the Core package.
type ERetCode int

const (
	RetCodeOK          ERetCode = 0
	RetCodeCondTimeout ERetCode = 1
	RetCodeError       ERetCode = -1
)

func NewCYNamedCondition(name string) *CYNamedCondition {
	return &CYNamedCondition{name: name, cond: sync.NewCond(&sync.Mutex{})}
}

func (c *CYNamedCondition) GetName() string { return c.name }
func (c *CYNamedCondition) Wait()           { c.cond.Wait() }
func (c *CYNamedCondition) Signal()        { c.cond.Signal() }
func (c *CYNamedCondition) Broadcast()      { c.cond.Broadcast() }
func (c *CYNamedCondition) Lock()           { c.cond.L.Lock() }
func (c *CYNamedCondition) Unlock()        { c.cond.L.Unlock() }

// Reset re-initialises the condition variable (mirrors C++ Reset).
func (c *CYNamedCondition) Reset() {
	c.cond = sync.NewCond(&sync.Mutex{})
}

// WaitForEvent waits until Signal/Broadcast, or until timeout elapses. On timeout
// it returns RetCodeCondTimeout, otherwise RetCodeOK (mirrors C++ WaitForEvent).
func (c *CYNamedCondition) WaitForEvent(timeout time.Duration) ERetCode {
	if timeout <= 0 {
		c.Lock()
		c.cond.Wait()
		c.Unlock()
		return RetCodeOK
	}
	timedOut := make(chan struct{})
	timer := time.NewTimer(timeout)
	go func() {
		select {
		case <-timer.C:
			close(timedOut)
			c.Signal()
		case <-timedOut:
		}
	}()
	c.Lock()
	c.cond.Wait()
	c.Unlock()
	timer.Stop()
	select {
	case <-timedOut:
		return RetCodeCondTimeout
	default:
		return RetCodeOK
	}
}

// ============================================================================
// CYBaseMessage - pooled message type with sync.Pool for zero-allocation reuse
// ============================================================================

type CYBaseMessage struct {
	EMsgType   int
	NSeverCode int
	StrChannel string // per-message channel, mirrors C++ WriteLog szChannel; rendered by layouts
	StrMsg     string
	StrFile    string
	StrFunc    string
	NLine      int
	Time       time.Time
	NProcessId uint64
	NThreadId  uint64
}

// Reset clears all fields to their zero values for pool reuse.
func (m *CYBaseMessage) Reset() {
	m.EMsgType = 0
	m.NSeverCode = 0
	m.StrChannel = ""
	m.StrMsg = ""
	m.StrFile = ""
	m.StrFunc = ""
	m.NLine = 0
	m.NProcessId = 0
	m.NThreadId = 0
}

var baseMessagePool = sync.Pool{
	New: func() any { return &CYBaseMessage{} },
}

func AcquireBaseMessage() *CYBaseMessage {
	return baseMessagePool.Get().(*CYBaseMessage)
}

func (m *CYBaseMessage) Clone() *CYBaseMessage {
	msg := AcquireBaseMessage()
	*msg = *m
	return msg
}

func ReleaseBaseMessage(m *CYBaseMessage) {
	if m == nil {
		return
	}
	m.Reset()
	baseMessagePool.Put(m)
}

// ============================================================================
// CYBaseMessage subtypes - these three poolable message types mirror the C++
// CYNormalMessage / CYEscapeMessage / CYStrMessage trio. C++ models them as
// separate classes; Go keeps a single CYBaseMessage payload and exposes the
// three named wrappers (with their own sync.Pools) so callers can select the
// escaping/formatting semantics at acquisition time.
// ============================================================================

type CYNormalMessage struct {
	CYBaseMessage
}

var normalMessagePool = sync.Pool{
	New: func() any { return &CYNormalMessage{} },
}

func AcquireNormalMessage() *CYNormalMessage {
	return normalMessagePool.Get().(*CYNormalMessage)
}

func ReleaseNormalMessage(m *CYNormalMessage) {
	if m != nil {
		m.CYBaseMessage.Reset()
		normalMessagePool.Put(m)
	}
}

type CYEscapeMessage struct {
	CYBaseMessage
}

var escapeMessagePool = sync.Pool{
	New: func() any { return &CYEscapeMessage{} },
}

func AcquireEscapeMessage() *CYEscapeMessage {
	return escapeMessagePool.Get().(*CYEscapeMessage)
}

func ReleaseEscapeMessage(m *CYEscapeMessage) {
	if m != nil {
		m.CYBaseMessage.Reset()
		escapeMessagePool.Put(m)
	}
}

type CYStrMessage struct {
	CYBaseMessage
}

var strMessagePool = sync.Pool{
	New: func() any { return &CYStrMessage{} },
}

func AcquireStrMessage() *CYStrMessage {
	return strMessagePool.Get().(*CYStrMessage)
}

func ReleaseStrMessage(m *CYStrMessage) {
	if m != nil {
		m.CYBaseMessage.Reset()
		strMessagePool.Put(m)
	}
}

// ============================================================================
// CYFPSCounter - FPS measurement
// ============================================================================

type CYFPSCounter struct {
	CYNoCopy
	mu sync.Mutex

	bRunning     atomic.Bool
	nFrames      atomic.Int64
	nMsPerUpdate int64
	lastTick     atomic.Int64
	fpsValue     atomic.Value

	// Average-FPS window (mirrors C++ double-window measurement).
	startTime    atomic.Int64
	nTotalFrames atomic.Int64
}

func NewCYFPSCounter(nMsPerUpdate int) *CYFPSCounter {
	c := &CYFPSCounter{nMsPerUpdate: int64(nMsPerUpdate)}
	c.lastTick.Store(time.Now().UnixNano())
	return c
}

func (c *CYFPSCounter) Start() {
	c.bRunning.Store(true)
	now := time.Now().UnixNano()
	c.lastTick.Store(now)
	c.startTime.Store(now)
	c.nFrames.Store(0)
	c.nTotalFrames.Store(0)
	c.fpsValue.Store(float64(0))
}

func (c *CYFPSCounter) Stop() {
	c.bRunning.Store(false)
}

func (c *CYFPSCounter) IsRunning() bool {
	return c.bRunning.Load()
}

func (c *CYFPSCounter) Increment() {
	c.nFrames.Add(1)
	c.nTotalFrames.Add(1)
	c.update()
}

func (c *CYFPSCounter) GetFPS() float64 {
	v := c.fpsValue.Load()
	if v == nil {
		return 0
	}
	return v.(float64)
}

// GetAverageFPS returns the average frames-per-second since Start, mirroring the
// C++ CYFPSCounter average window.
func (c *CYFPSCounter) GetAverageFPS() float64 {
	start := c.startTime.Load()
	if start == 0 {
		return 0
	}
	elapsed := time.Now().UnixNano() - start
	if elapsed <= 0 {
		return 0
	}
	total := c.nTotalFrames.Load()
	return float64(total) * float64(time.Second) / float64(elapsed)
}

func (c *CYFPSCounter) GetFrames() int64 {
	return c.nFrames.Load()
}

func (c *CYFPSCounter) Reset() {
	c.nFrames.Store(0)
	c.nTotalFrames.Store(0)
	now := time.Now().UnixNano()
	c.lastTick.Store(now)
	c.startTime.Store(now)
	c.fpsValue.Store(float64(0))
}

func (c *CYFPSCounter) update() {
	now := time.Now().UnixNano()
	last := c.lastTick.Load()
	elapsed := now - last
	if elapsed >= c.nMsPerUpdate*int64(time.Millisecond) {
		count := c.nFrames.Swap(0)
		fps := float64(count) * float64(time.Second) / float64(elapsed)
		c.fpsValue.Store(fps)
		c.lastTick.Store(now)
	}
}

// ============================================================================
// CYPublicFunction - cross-platform utilities
// ============================================================================

type CYPublicFunction struct{}

var g_CYPublicFunctionInstance *CYPublicFunction

func GetCYPublicFunctionInstance() *CYPublicFunction {
	if g_CYPublicFunctionInstance == nil {
		g_CYPublicFunctionInstance = &CYPublicFunction{}
	}
	return g_CYPublicFunctionInstance
}

func (pf *CYPublicFunction) IsFileExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (pf *CYPublicFunction) MakeDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

func (pf *CYPublicFunction) RemoveFile(path string) error {
	return os.Remove(path)
}

func (pf *CYPublicFunction) FileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func (pf *CYPublicFunction) DeleteDirectory(dir string) error {
	return os.RemoveAll(dir)
}

func (pf *CYPublicFunction) GetCurrentPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func (pf *CYPublicFunction) GetCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func (pf *CYPublicFunction) GetAppName() string {
	exe, err := os.Executable()
	if err != nil {
		return "app"
	}
	base := filepath.Base(exe)
	ext := filepath.Ext(base)
	if ext == ".exe" {
		base = base[:len(base)-len(ext)]
	}
	return base
}

func (pf *CYPublicFunction) GetCurrentProcessId() int {
	return os.Getpid()
}

func (pf *CYPublicFunction) GetCurrentThreadId() uint64 {
	return GetGID()
}

func (pf *CYPublicFunction) GetEnvironmentVariable(key string) string {
	return os.Getenv(key)
}

func (pf *CYPublicFunction) SetEnvironmentVariable(key, value string) error {
	return os.Setenv(key, value)
}

func (pf *CYPublicFunction) ToString(args ...any) string {
	if len(args) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, arg := range args {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(fmt.Sprintf("%v", arg))
	}
	return sb.String()
}

func (pf *CYPublicFunction) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (pf *CYPublicFunction) WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (pf *CYPublicFunction) CopyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return pf.WriteFile(dst, data)
}

func (pf *CYPublicFunction) Sleep(durationMs int) {
	if durationMs > 0 {
		time.Sleep(time.Duration(durationMs) * time.Millisecond)
	}
}

func (pf *CYPublicFunction) GetOSName() string {
	return runtime.GOOS
}

func (pf *CYPublicFunction) IsWindows() bool {
	return runtime.GOOS == "windows"
}

func (pf *CYPublicFunction) IsLinux() bool {
	return runtime.GOOS == "linux"
}

func (pf *CYPublicFunction) IsMacOS() bool {
	return runtime.GOOS == "darwin"
}

// GetFileName returns the file name of path stripped of its directory and
// extension, mirroring C++ CYPublicFunction::GetFileName.
func (pf *CYPublicFunction) GetFileName(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = base[:len(base)-len(ext)]
	}
	return base
}

// GetFileExt returns the extension of path without the leading dot, mirroring
// C++ CYPublicFunction::GetFileExt.
func (pf *CYPublicFunction) GetFileExt(path string) string {
	return strings.TrimPrefix(filepath.Ext(path), ".")
}

// GetBaseLogName returns the base log file name of path with everything after
// the last '_' removed, mirroring C++ CYPublicFunction::GetBaseLogName.
func (pf *CYPublicFunction) GetBaseLogName(path string) string {
	name := pf.GetFileName(path)
	if i := strings.LastIndex(name, "_"); i != -1 {
		return name[:i]
	}
	return name
}

// GetBasePath returns the directory joined with the base log name of path,
// mirroring C++ CYPublicFunction::GetBasePath.
func (pf *CYPublicFunction) GetBasePath(path string) string {
	return filepath.Join(filepath.Dir(path), pf.GetBaseLogName(path))
}

// PrintTraceHexLog prints a hex+ASCII dump of data to stdout using the trace
// colour (no file logging), mirroring C++ CYPublicFunction::PrintTraceHexLog.
func (pf *CYPublicFunction) PrintTraceHexLog(data []byte) {
	pf.WriteToConsole(ColorTrace, formatHexDump(data))
}

// WriteToConsole writes msg to stdout wrapped in the given ANSI color (mirrors
// C++ CYPublicFunction::WriteToConsole coloured output).
func (pf *CYPublicFunction) WriteToConsole(color, msg string) {
	if color == "" {
		fmt.Fprint(os.Stdout, msg)
		return
	}
	fmt.Fprintf(os.Stdout, "%s%s%s", color, msg, ColorReset)
}

// PrintLog prints a formatted, info-coloured line to stdout (no file logging).
func (pf *CYPublicFunction) PrintLog(format string, args ...any) {
	pf.WriteToConsole(ColorInfo, fmt.Sprintf(format, args...)+"\n")
}

// PrintTraceLog prints a formatted, trace-coloured line to stdout (no file logging).
func (pf *CYPublicFunction) PrintTraceLog(format string, args ...any) {
	pf.WriteToConsole(ColorTrace, fmt.Sprintf(format, args...)+"\n")
}

// PrintHexLog prints a hex+ASCII dump of data to stdout (no file logging).
func (pf *CYPublicFunction) PrintHexLog(data []byte) {
	pf.WriteToConsole(ColorReset, formatHexDump(data))
}

// TrimString trims leading/trailing whitespace (mirrors C++ TrimString).
func (pf *CYPublicFunction) TrimString(s string) string {
	return strings.TrimSpace(s)
}

// Verify panics with msg when condition is false (mirrors C++ Verify).
func (pf *CYPublicFunction) Verify(condition bool, msg string) {
	if !condition {
		panic(msg)
	}
}

// GetLastWriteTime returns the last modification time of the file at path.
func (pf *CYPublicFunction) GetLastWriteTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// GetLocalUTCOffsetHours returns the local timezone's offset from UTC in hours,
// mirroring C++ GetLocalUTCOffsetHours.
func (pf *CYPublicFunction) GetLocalUTCOffsetHours() float64 {
	_, offsetSec := time.Now().Zone()
	return float64(offsetSec) / 3600.0
}

// formatHexDump emits a hex + ASCII table (re-used by PrintHexLog).
func formatHexDump(data []byte) string {
	const lineWidth = 16
	var sb strings.Builder
	for i := 0; i < len(data); i += lineWidth {
		end := i + lineWidth
		if end > len(data) {
			end = len(data)
		}
		sb.WriteString(fmt.Sprintf("%04x: ", i))
		for j := i; j < end; j++ {
			sb.WriteString(fmt.Sprintf("%02x ", data[j]))
		}
		if end-i < lineWidth {
			for j := end - i; j < lineWidth; j++ {
				sb.WriteString("   ")
			}
		}
		sb.WriteString(" |")
		for j := i; j < end; j++ {
			c := data[j]
			if c >= 32 && c < 127 {
				sb.WriteByte(c)
			} else {
				sb.WriteByte('.')
			}
		}
		sb.WriteString("|\n")
	}
	return sb.String()
}

// GetGID returns the current goroutine ID.
func GetGID() uint64 {
	var id uint64
	b := make([]byte, 32)
	runtime.Stack(b, false)
	fields := strings.Fields(string(b))
	for _, field := range fields {
		if strings.HasPrefix(field, "goroutine") {
			parts := strings.Split(field, "[")
			if len(parts) >= 2 {
				numStr := strings.Split(parts[1], "]")[0]
				for _, c := range numStr {
					if c >= '0' && c <= '9' {
						id = id*10 + uint64(c-'0')
					} else {
						break
					}
				}
				break
			}
		}
	}
	return id
}

// ============================================================================
// CYPathConvert - path conversion and log filename generation
// ============================================================================

type CYPathConvert struct{}

var g_CYPathConvertInstance *CYPathConvert
var pathConvertOnce sync.Once

func GetCYPathConvertInstance() *CYPathConvert {
	pathConvertOnce.Do(func() {
		g_CYPathConvertInstance = &CYPathConvert{}
	})
	return g_CYPathConvertInstance
}

func (pc *CYPathConvert) ConvertLogPath(szLogPath string) string {
	if szLogPath == "" {
		dir := GetCYPublicFunctionInstance().GetCurrentDir()
		return filepath.Join(dir, LogDir)
	}
	return szLogPath
}

func (pc *CYPathConvert) logPrefix(eLogType int) string {
	switch eLogType {
	case 1:
		return "Trace"
	case 2:
		return "Debug"
	case 3:
		return "Info"
	case 4:
		return "Warn"
	case 5:
		return "Error"
	case 6:
		return "Fatal"
	case 7:
		return "Main"
	case 8:
		return "Remote"
	case 9:
		return "Sys"
	default:
		return "Log"
	}
}

func (pc *CYPathConvert) GetLogFileName(szChannel string, eLogType int) string {
	now := time.Now()
	prefix := pc.logPrefix(eLogType)
	if szChannel != "" {
		return fmt.Sprintf("%s_%s_%s.log", prefix, szChannel, now.Format("20060102_150405"))
	}
	return fmt.Sprintf("%s_%s.log", prefix, now.Format("20060102_150405"))
}

// GetFixedLogFileName returns a stable file name (no timestamp). It is used in
// LogFileModeAppend to keep a single rolling file per log type, mirroring the
// C++ fixed-name append behaviour.
func (pc *CYPathConvert) GetFixedLogFileName(szChannel string, eLogType int) string {
	prefix := pc.logPrefix(eLogType)
	if szChannel != "" {
		return fmt.Sprintf("%s_%s.log", prefix, szChannel)
	}
	return fmt.Sprintf("%s.log", prefix)
}

func (pc *CYPathConvert) GetErrorFileName(szChannel string) string {
	if szChannel != "" {
		return fmt.Sprintf("Error_%s.log", szChannel)
	}
	return "Error.log"
}

func (pc *CYPathConvert) NormalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimRight(path, "/")
	return path
}

func (pc *CYPathConvert) JoinPath(parts ...string) string {
	return pc.NormalizePath(filepath.Join(parts...))
}

func (pc *CYPathConvert) Dir(path string) string {
	return filepath.Dir(path)
}

func (pc *CYPathConvert) Base(path string) string {
	return filepath.Base(path)
}

func (pc *CYPathConvert) Ext(path string) string {
	return filepath.Ext(path)
}

func (pc *CYPathConvert) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

// ============================================================================
// CYFileRestriction - file size tracking and rotation
// ============================================================================

type CYFileRestriction struct {
	mu sync.Mutex

	bEnableCheck          bool
	bClearUnLogFile       bool
	nLimitTimeClearLog    int
	nLimitTimeExpiredFile int
	nCheckFileSizeTime    int
	nCheckFileCountTime   int
	nCheckFileSize        int
	nFileCountPerType     int
	nCheckFileTypeSize    int
	nCheckAllFileSize     int

	bExceed    atomic.Bool
	nFileSize  atomic.Int64
	nCheckTime atomic.Int64
}

func NewCYFileRestriction() *CYFileRestriction {
	return &CYFileRestriction{
		bEnableCheck:          true,
		bClearUnLogFile:       true,
		nLimitTimeClearLog:    60,
		nLimitTimeExpiredFile: 24,
		nCheckFileSizeTime:    300,
		nCheckFileCountTime:   60,
		nCheckFileSize:        5 * 1024 * 1024,
		nFileCountPerType:     20,
		nCheckFileTypeSize:    500 * 1024 * 1024,
		nCheckAllFileSize:     1024 * 1024 * 1024,
	}
}

func (fr *CYFileRestriction) SetRestriction(bEnableCheck, bClearUnLogFile bool,
	nLimitTimeClearLog, nLimitTimeExpiredFile, nCheckFileSizeTime, nCheckFileCountTime,
	nCheckFileSize, nFileCountPerType, nCheckFileTypeSize, nCheckAllFileSize int) {
	fr.mu.Lock()
	defer fr.mu.Unlock()
	fr.bEnableCheck = bEnableCheck
	fr.bClearUnLogFile = bClearUnLogFile
	fr.nLimitTimeClearLog = nLimitTimeClearLog
	fr.nLimitTimeExpiredFile = nLimitTimeExpiredFile
	fr.nCheckFileSizeTime = nCheckFileSizeTime
	fr.nCheckFileCountTime = nCheckFileCountTime
	fr.nCheckFileSize = nCheckFileSize
	fr.nFileCountPerType = nFileCountPerType
	fr.nCheckFileTypeSize = nCheckFileTypeSize
	fr.nCheckAllFileSize = nCheckAllFileSize
}

func (fr *CYFileRestriction) IsEnableCheck() bool             { return fr.bEnableCheck }
func (fr *CYFileRestriction) IsClearUnLogFile() bool        { return fr.bClearUnLogFile }
func (fr *CYFileRestriction) GetTimeClearLog() int          { return fr.nLimitTimeClearLog }
func (fr *CYFileRestriction) GetTimeExpiredFile() int       { return fr.nLimitTimeExpiredFile }
func (fr *CYFileRestriction) GetCheckFileSizeTime() int     { return fr.nCheckFileSizeTime }
func (fr *CYFileRestriction) GetCheckFileCountTime() int     { return fr.nCheckFileCountTime }
func (fr *CYFileRestriction) GetCheckFileSize() int          { return fr.nCheckFileSize }
func (fr *CYFileRestriction) GetFileCountPerType() int       { return fr.nFileCountPerType }
func (fr *CYFileRestriction) GetCheckFileTypeSize() int      { return fr.nCheckFileTypeSize }
func (fr *CYFileRestriction) GetCheckAllFileSize() int        { return fr.nCheckAllFileSize }
func (fr *CYFileRestriction) SetFileSize(nFileSize int64)   { fr.nFileSize.Store(nFileSize) }
func (fr *CYFileRestriction) GetFileSize() int64            { return fr.nFileSize.Load() }
func (fr *CYFileRestriction) SetCheckTime(nCheckTime int64) { fr.nCheckTime.Store(nCheckTime) }
func (fr *CYFileRestriction) GetCheckTime() int64          { return fr.nCheckTime.Load() }
func (fr *CYFileRestriction) IsExceed() bool                { return fr.bExceed.Load() }
func (fr *CYFileRestriction) SetExceed(bExceed bool)       { fr.bExceed.Store(bExceed) }

func (fr *CYFileRestriction) CheckFileSize(szFileName string) bool {
	if !fr.bEnableCheck {
		return false
	}
	info, err := os.Stat(szFileName)
	if err != nil {
		return false
	}
	return info.Size() >= int64(fr.nCheckFileSize)
}

// IsCreateNewLog reports whether a fresh log file should be started, mirroring
// C++ CYFileRestriction::IsCreateNewLog (size-threshold based decision).
func (fr *CYFileRestriction) IsCreateNewLog(nFileSize int64) bool {
	if !fr.bEnableCheck {
		return false
	}
	return nFileSize >= int64(fr.nCheckFileSize)
}

// GetNewLogName returns a timestamped log file name for a forced rotation,
// mirroring C++ CYFileRestriction::GetNewLogName.
func (fr *CYFileRestriction) GetNewLogName() string {
	now := time.Now()
	return fmt.Sprintf("Log_%s.log", now.Format("20060102_150405.000000"))
}

// ============================================================================
// CYTimeUtils - high-resolution timestamp helper
// ============================================================================

// NowNano returns the current time in nanoseconds. C++ CYTimeUtils::rdtsc() reads
// the CPU time-stamp counter, which has no portable Go equivalent; NowNano() is
// the closest cross-platform approximation and is suitable for elapsed-time and
// ordering checks.
func NowNano() int64 {
	return time.Now().UnixNano()
}

// Rdtsc returns a nanosecond-granularity timestamp - the closest portable Go
// equivalent of C++ CYTimeUtils::rdtsc() (the CPU time-stamp counter). Go cannot
// read the hardware TSC directly (nor runtime.nanotime, which is unexported);
// time.Now().UnixNano() provides a high-resolution wall-clock timestamp that is
// suitable for elapsed-time and ordering measurements, mirroring the TSC's
// intended use. Prefer Rdtsc over NowNano only when a symbolic TSC name aids
// readability at a call site.
func Rdtsc() int64 {
	return time.Now().UnixNano()
}
