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
	"log/syslog"
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
}

// LOG_TYPE_BUFFER is a virtual type for buffer appenders.
const LOG_TYPE_BUFFER Core.ELogType = 100

// CYLoggerBaseAppender is the base appender with double-buffering.
type CYLoggerBaseAppender struct {
	Common.CYNamedThread
	Common.CYNoCopy
	mu              sync.Mutex
	eLogType        Core.ELogType
	szChannel       string
	szFile          string
	szLogPath       string
	eFileMode       Core.ELogFileMode
	bEnable         atomic.Bool
	nBufferCapacity int

	PublicQueue  chan *Common.CYBaseMessage
	PrivateQueue chan *Common.CYBaseMessage
	flushCond    *sync.Cond
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
	}
	app.flushCond = sync.NewCond(&app.mu)
	app.fpsCounter = Common.NewCYFPSCounter(1000)
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
func (a *CYLoggerBaseAppender) GetQueueSize() int                     { return len(a.PublicQueue) }

func (a *CYLoggerBaseAppender) Init() bool { return true }

func (a *CYLoggerBaseAppender) UnInit() {
	// With synchronous Write, the queue is bypassed for file appenders.
	// Just stop the goroutine and close the channel.
	a.Stop()
	if a.PublicQueue != nil {
		close(a.PublicQueue)
		a.PublicQueue = nil
	}
}

func (a *CYLoggerBaseAppender) Flush() {
	a.mu.Lock()
	for len(a.PublicQueue) > 0 || len(a.PrivateQueue) > 0 {
		a.flushCond.Wait()
	}
	a.mu.Unlock()
}

func (a *CYLoggerBaseAppender) Write(pMsg *Common.CYBaseMessage) bool {
	if !a.bEnable.Load() || pMsg == nil {
		return false
	}
	select {
	case a.PublicQueue <- pMsg:
		return true
	default:
		return false
	}
}

func (a *CYLoggerBaseAppender) Run() {
	flushTick := time.NewTicker(time.Second)
	defer flushTick.Stop()
	for {
		select {
		case <-flushTick.C:
			a.processMessages()
		case msg, ok := <-a.PublicQueue:
			if !ok {
				a.processMessages()
				return
			}
			a.handleMessage(msg)
		}
	}
}

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
	a.mu.Lock()
	defer a.mu.Unlock()
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
	close(a.stopCh)
	if a.timeRotator != nil {
		a.timeRotator.Stop()
	}
	a.Flush()
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
		fileName = pc_.GetLogFileName(a.szChannel, int(a.eLogType))
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
	if info.Size() >= int64(a.restriction.GetCheckFileSize()) {
		a.rotateFileLocked()
	}
}

func (a *CYLoggerFileAppender) rotateFileLocked() {
	if a.file != nil {
		a.file.Close()
		a.file = nil
	}
	a.openFile()
}

func (a *CYLoggerFileAppender) Write(pMsg *Common.CYBaseMessage) bool {
	if !a.bEnable.Load() || pMsg == nil {
		return false
	}
	defer Common.ReleaseBaseMessage(pMsg)
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
	defer Common.ReleaseBaseMessage(pMsg)
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
	select {
	case q <- pMsg:
		return true
	default:
		return false
	}
}

func (a *CYLoggerBufferAppender) Run() {
	a.flushTicks = time.NewTicker(time.Second)
	defer a.flushTicks.Stop()

	var buffer []*Common.CYBaseMessage
	var bufMu sync.Mutex

	for a.IsRunning() {
		select {
		case <-a.flushTicks.C:
			a.drainAndSort(&buffer, &bufMu)
		case msg := <-a.PublicQueue:
			a.handleMessage(msg)
		case msg := <-a.PrivateQueue:
			a.handleMessage(msg)
		default:
			time.Sleep(time.Millisecond * 10)
		}
	}
	a.drainAndSort(&buffer, &bufMu)
	a.processMessages()
}

func (a *CYLoggerBufferAppender) drainAndSort(buffer *[]*Common.CYBaseMessage, bufMu sync.Locker) {
	bufMu.Lock()
	defer bufMu.Unlock()

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
	for {
		select {
		case msg := <-a.PublicQueue:
			a.writeMessage(msg)
		default:
			break
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

// CYLoggerRemoteAppender sends logs over TCP to a remote host.
type CYLoggerRemoteAppender struct {
	*CYLoggerBaseAppender
	mu          struct{}
	sync.RWMutex
	remoteAddr  string
	conn        net.Conn
	reconnectCh chan struct{}
	stopCh      chan struct{}
}

func newRemoteAppender(eLogType Core.ELogType, szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode) *CYLoggerRemoteAppender {
	app := &CYLoggerRemoteAppender{
		CYLoggerBaseAppender: newBaseAppender(eLogType, szChannel, szFile, szLogPath, eFileMode),
		remoteAddr:  szFile,
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
	conn, err := net.DialTimeout("tcp", a.remoteAddr, 5*time.Second)
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
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))
	a.conn.Write(header)
	a.conn.Write(data)
}

// CYLoggerSystemAppender writes to system event log (syslog on Unix).
type CYLoggerSystemAppender struct {
	Common.CYNoCopy
	eLogType    Core.ELogType
	szLogPath   string
	mu          sync.Mutex
	w           *syslog.Writer
	bEnable     atomic.Bool
	PublicQueue chan *Common.CYBaseMessage
}

func newSystemAppender(eLogType Core.ELogType, szChannel, szFile, szLogPath string, eFileMode Core.ELogFileMode) *CYLoggerSystemAppender {
	_ = szChannel
	_ = szFile
	_ = eFileMode
	return &CYLoggerSystemAppender{
		eLogType:    eLogType,
		szLogPath:   szLogPath,
		PublicQueue: make(chan *Common.CYBaseMessage, 4096),
	}
}

func (a *CYLoggerSystemAppender) Init() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	var err error
	a.w, err = syslog.New(syslog.LOG_INFO|syslog.LOG_USER, "CYGoLogger")
	if err != nil {
		a.w = nil
		return false
	}
	return true
}

func (a *CYLoggerSystemAppender) UnInit() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.w != nil {
		a.w.Close()
		a.w = nil
	}
}

func (a *CYLoggerSystemAppender) Flush() {}

func (a *CYLoggerSystemAppender) Write(pMsg *Common.CYBaseMessage) bool {
	if !a.bEnable.Load() || pMsg == nil {
		return false
	}
	select {
	case a.PublicQueue <- pMsg:
		return true
	default:
		return false
	}
}

func (a *CYLoggerSystemAppender) GetLogType() Core.ELogType { return a.eLogType }
func (a *CYLoggerSystemAppender) GetLogPath() string         { return a.szLogPath }
func (a *CYLoggerSystemAppender) SetLogPath(szLogPath string) { a.szLogPath = szLogPath }
func (a *CYLoggerSystemAppender) GetChannel() string          { return "" }
func (a *CYLoggerSystemAppender) GetFile() string            { return "" }
func (a *CYLoggerSystemAppender) GetFileMode() Core.ELogFileMode { return Core.LogFileModeAppend }
func (a *CYLoggerSystemAppender) IsEnable() bool             { return a.bEnable.Load() }
func (a *CYLoggerSystemAppender) SetEnable(b bool)          { a.bEnable.Store(b) }
func (a *CYLoggerSystemAppender) GetLayout() Layout.ICYLoggerTemplateLayout { return nil }
func (a *CYLoggerSystemAppender) SetLayout(Layout.ICYLoggerTemplateLayout) {}
func (a *CYLoggerSystemAppender) GetFilter() *Filter.ICYLoggerPatternFilter { return nil }
func (a *CYLoggerSystemAppender) SetFilter(*Filter.ICYLoggerPatternFilter) {}
func (a *CYLoggerSystemAppender) GetFPSCounter() *Common.CYFPSCounter { return nil }
func (a *CYLoggerSystemAppender) GetQueueSize() int             { return len(a.PublicQueue) }
func (a *CYLoggerSystemAppender) Run() {
	for msg := range a.PublicQueue {
		if msg != nil {
			Common.ReleaseBaseMessage(msg)
		}
	}
}
func (a *CYLoggerSystemAppender) Start(run func()) {
	go run()
}

func (a *CYLoggerSystemAppender) doWrite(msg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.w == nil {
		return
	}
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}
	switch {
	case a.eLogType == Core.LogTypeError || a.eLogType == Core.LogTypeFatal:
		a.w.Err(msg)
	case a.eLogType == Core.LogTypeWarn:
		a.w.Warning(msg)
	case a.eLogType == Core.LogTypeDebug:
		a.w.Debug(msg)
	default:
		a.w.Info(msg)
	}
}

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
	case Core.LogTypeMain:
		return newMainAppender(szChannel, szFile, szLogPath, eFileMode)
	case LOG_TYPE_BUFFER:
		return newBufferAppender(eLogType, szChannel, szFile, szLogPath, eFileMode)
	case Core.LogTypeRemote:
		return newRemoteAppender(eLogType, szChannel, szFile, szLogPath, eFileMode)
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
