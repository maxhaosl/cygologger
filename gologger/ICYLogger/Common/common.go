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
	LogDir             = "Log"
	LogSeparator       = " "
	LogEscapeChar      = '\\'
	LogFieldNameEnd    = ']'
	LogFieldValueEnd   = ','
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

func (e *CYTimeElapsed) ElapsedMilliseconds() int64 {
	return time.Since(e.startTime).Milliseconds()
}

func (e *CYTimeElapsed) ElapsedSeconds() float64 {
	return time.Since(e.startTime).Seconds()
}

// ============================================================================
// CYTimeStamps - timestamp formatting
// ============================================================================

type CYTimeStamps struct {
	mu sync.Mutex
}

var g_CYTimeStampsInstance *CYTimeStamps
var g_CYTimeStampsOnce sync.Once

func GetCYTimeStampsInstance() *CYTimeStamps {
	g_CYTimeStampsOnce.Do(func() {
		g_CYTimeStampsInstance = &CYTimeStamps{}
	})
	return g_CYTimeStampsInstance
}

func (ts *CYTimeStamps) GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d.%03d", nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
}

func (ts *CYTimeStamps) GetTimeStampsFile(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return fmt.Sprintf("%04d%02d%02d%02d%02d%02d%03d", nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
}

func (ts *CYTimeStamps) GetTimeStampsShort(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
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

func NewCYNamedCondition(name string) *CYNamedCondition {
	return &CYNamedCondition{name: name, cond: sync.NewCond(&sync.Mutex{})}
}

func (c *CYNamedCondition) GetName() string { return c.name }
func (c *CYNamedCondition) Wait()           { c.cond.Wait() }
func (c *CYNamedCondition) Signal()        { c.cond.Signal() }
func (c *CYNamedCondition) Broadcast()      { c.cond.Broadcast() }
func (c *CYNamedCondition) Lock()           { c.cond.L.Lock() }
func (c *CYNamedCondition) Unlock()        { c.cond.L.Unlock() }

// ============================================================================
// CYBaseMessage - pooled message type
// ============================================================================

type CYBaseMessage struct {
	EMsgType   int
	NSeverCode int
	StrMsg     string
	StrFile    string
	StrFunc    string
	NLine      int
	Time       time.Time
	NProcessId uint64
	NThreadId  uint64
}

func AcquireBaseMessage() *CYBaseMessage {
	return &CYBaseMessage{}
}

func (m *CYBaseMessage) Clone() *CYBaseMessage {
	msg := &CYBaseMessage{}
	*msg = *m
	return msg
}

func ReleaseBaseMessage(m *CYBaseMessage) {
	if m == nil {
		return
	}
}

type CYNormalMessage struct {
	CYBaseMessage
}

func AcquireNormalMessage() *CYNormalMessage {
	return &CYNormalMessage{
		CYBaseMessage: *AcquireBaseMessage(),
	}
}

func ReleaseNormalMessage(m *CYNormalMessage) {
	if m != nil {
		ReleaseBaseMessage(&m.CYBaseMessage)
	}
}

type CYEscapeMessage struct {
	CYBaseMessage
}

func AcquireEscapeMessage() *CYEscapeMessage {
	return &CYEscapeMessage{
		CYBaseMessage: *AcquireBaseMessage(),
	}
}

func ReleaseEscapeMessage(m *CYEscapeMessage) {
	if m != nil {
		ReleaseBaseMessage(&m.CYBaseMessage)
	}
}

type CYStrMessage struct {
	CYBaseMessage
}

func AcquireStrMessage() *CYStrMessage {
	return &CYStrMessage{
		CYBaseMessage: *AcquireBaseMessage(),
	}
}

func ReleaseStrMessage(m *CYStrMessage) {
	if m != nil {
		ReleaseBaseMessage(&m.CYBaseMessage)
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
}

func NewCYFPSCounter(nMsPerUpdate int) *CYFPSCounter {
	c := &CYFPSCounter{nMsPerUpdate: int64(nMsPerUpdate)}
	c.lastTick.Store(time.Now().UnixNano())
	return c
}

func (c *CYFPSCounter) Start() {
	c.bRunning.Store(true)
	c.lastTick.Store(time.Now().UnixNano())
	c.nFrames.Store(0)
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
	c.update()
}

func (c *CYFPSCounter) GetFPS() float64 {
	v := c.fpsValue.Load()
	if v == nil {
		return 0
	}
	return v.(float64)
}

func (c *CYFPSCounter) GetFrames() int64 {
	return c.nFrames.Load()
}

func (c *CYFPSCounter) Reset() {
	c.nFrames.Store(0)
	c.lastTick.Store(time.Now().UnixNano())
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

func (pc *CYPathConvert) GetLogFileName(szChannel string, eLogType int) string {
	now := time.Now()
	prefix := ""
	switch eLogType {
	case 1:
		prefix = "Trace"
	case 2:
		prefix = "Debug"
	case 3:
		prefix = "Info"
	case 4:
		prefix = "Warn"
	case 5:
		prefix = "Error"
	case 6:
		prefix = "Fatal"
	case 7:
		prefix = "Main"
	case 8:
		prefix = "Remote"
	case 9:
		prefix = "Sys"
	default:
		prefix = "Log"
	}
	if szChannel != "" {
		return fmt.Sprintf("%s_%s_%s.log", prefix, szChannel, now.Format("20060102_150405"))
	}
	return fmt.Sprintf("%s_%s.log", prefix, now.Format("20060102_150405"))
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
