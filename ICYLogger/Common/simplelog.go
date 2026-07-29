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

package Common

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ============================================================================
// CYSimpleLog - lightweight synchronous logger
//
// Port of C++ CYSimpleLog / CYSimpleLogFile / CYSimpleLogConsole.
//
// Unlike the main asynchronous logging pipeline (Appender + goroutine queues),
// SimpleLog writes SYNCHRONOUSLY. This guarantees that log lines are flushed to
// disk/console immediately, which is essential for crash/exception logging where
// the process may be about to terminate.
// ============================================================================

// ESimpleLogType mirrors C++ CYSimpleLog::ESimpleLogType.
type ESimpleLogType int

const (
	SimpleLogTypeNone    ESimpleLogType = iota // no output
	SimpleLogTypeFile                          // file output
	SimpleLogTypeConsole                       // console output
	SimpleLogTypeAll                           // both file and console
)

// Console color constants (mirror C++ ConsoleColor enum, mapped to ANSI codes).
const (
	SimpleColorBlack     = "\033[0;30m"
	SimpleColorBlue      = "\033[0;34m"
	SimpleColorGreen     = "\033[0;32m"
	SimpleColorCyan      = "\033[0;36m"
	SimpleColorRed       = "\033[0;31m"
	SimpleColorMagenta   = "\033[0;35m"
	SimpleColorYellow    = "\033[0;33m"
	SimpleColorWhite     = "\033[0;37m"
	SimpleColorGray      = "\033[1;30m"
	SimpleColorIntensity = "\033[1m"
	SimpleColorReset     = "\033[0m"
)

// ISimpleLog is the common interface implemented by file and console simple loggers.
type ISimpleLog interface {
	InitLog(bLogTime, bLogLineCount bool, szWorkPath, szLogDir, szFilePath string) bool
	WriteString(str string) bool
	WriteLog(format string, args ...any) bool
	CloseLog()
	GetLogType() ESimpleLogType
}

// simpleLogBase holds shared state for simple loggers.
type simpleLogBase struct {
	eLogType      ESimpleLogType
	bInit         bool
	bLogTime      bool
	bLogLineCount bool
	dwLineCount   uint64
}

// buildLine prepends the optional timestamp and line-count prefixes to str.
func (b *simpleLogBase) buildLine(str string) string {
	prefix := ""
	if b.bLogTime {
		now := time.Now()
		prefix += now.Format("2006-01-02 15:04:05.000") + " "
	}
	if b.bLogLineCount {
		b.dwLineCount++
		prefix += fmt.Sprintf("[%d] ", b.dwLineCount)
	}
	return prefix + str
}

func (b *simpleLogBase) GetLogType() ESimpleLogType { return b.eLogType }

// ============================================================================
// CYSimpleLogFile - synchronous file logger
// ============================================================================

type CYSimpleLogFile struct {
	simpleLogBase
	mu       sync.Mutex
	file     *os.File
	logDir   string
	workPath string
	filePath string
}

// NewCYSimpleLogFile creates a new synchronous file logger.
func NewCYSimpleLogFile() *CYSimpleLogFile {
	f := &CYSimpleLogFile{}
	f.eLogType = SimpleLogTypeFile
	return f
}

// InitLog opens (creating directories as needed) the target log file.
// szWorkPath: base working directory (empty => current directory).
// szLogDir:   sub-directory under work path (empty => "Log").
// szFilePath: log file name (empty => "Simple.log").
func (f *CYSimpleLogFile) InitLog(bLogTime, bLogLineCount bool, szWorkPath, szLogDir, szFilePath string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.bLogTime = bLogTime
	f.bLogLineCount = bLogLineCount

	if szWorkPath == "" {
		szWorkPath = GetCYPublicFunctionInstance().GetCurrentDir()
	}
	if szLogDir == "" {
		szLogDir = LogDir
	}
	if szFilePath == "" {
		szFilePath = "Simple.log"
	}
	f.workPath = szWorkPath
	f.logDir = szLogDir
	f.filePath = szFilePath

	dir := filepath.Join(szWorkPath, szLogDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false
	}

	full := filepath.Join(dir, szFilePath)
	file, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return false
	}
	f.file = file
	f.bInit = true
	return true
}

// WriteString writes a single raw line (with optional prefixes) synchronously.
func (f *CYSimpleLogFile) WriteString(str string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.bInit || f.file == nil {
		return false
	}
	line := f.buildLine(str)
	if len(line) == 0 || line[len(line)-1] != '\n' {
		line += "\n"
	}
	if _, err := f.file.WriteString(line); err != nil {
		return false
	}
	// Synchronous flush to guarantee durability for crash logs.
	_ = f.file.Sync()
	return true
}

// WriteLog formats and writes a line synchronously.
func (f *CYSimpleLogFile) WriteLog(format string, args ...any) bool {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	return f.WriteString(msg)
}

// CloseLog closes the underlying file.
func (f *CYSimpleLogFile) CloseLog() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file != nil {
		_ = f.file.Sync()
		_ = f.file.Close()
		f.file = nil
	}
	f.bInit = false
}

// DeleteAllFile removes every regular file inside the given directory.
// Mirrors C++ CYSimpleLogFile::DeleteAllFile.
func (f *CYSimpleLogFile) DeleteAllFile(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

// ============================================================================
// CYSimpleLogConsole - synchronous console logger with color
// ============================================================================

type CYSimpleLogConsole struct {
	simpleLogBase
	mu           sync.Mutex
	consoleTitle string
	color        string
}

// NewCYSimpleLogConsole creates a new synchronous console logger.
func NewCYSimpleLogConsole() *CYSimpleLogConsole {
	c := &CYSimpleLogConsole{color: SimpleColorReset}
	c.eLogType = SimpleLogTypeConsole
	return c
}

// InitLog initializes the console logger. szFilePath is reused as the console title.
func (c *CYSimpleLogConsole) InitLog(bLogTime, bLogLineCount bool, szWorkPath, szLogDir, szFilePath string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bLogTime = bLogTime
	c.bLogLineCount = bLogLineCount
	if szFilePath == "" {
		szFilePath = "Log Info"
	}
	c.consoleTitle = szFilePath
	c.bInit = true
	return true
}

// Color sets the ANSI color applied to subsequent console lines.
func (c *CYSimpleLogConsole) Color(color string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if color == "" {
		color = SimpleColorReset
	}
	c.color = color
}

// WriteString writes a single raw line to stdout synchronously (with color).
func (c *CYSimpleLogConsole) WriteString(str string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.bInit {
		return false
	}
	line := c.buildLine(str)
	if len(line) == 0 || line[len(line)-1] != '\n' {
		line += "\n"
	}
	if c.color != "" && c.color != SimpleColorReset {
		_, _ = fmt.Fprint(os.Stdout, c.color, line, SimpleColorReset)
	} else {
		_, _ = fmt.Fprint(os.Stdout, line)
	}
	return true
}

// WriteLog formats and writes a line to stdout synchronously.
func (c *CYSimpleLogConsole) WriteLog(format string, args ...any) bool {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	return c.WriteString(msg)
}

// CloseLog is a no-op for the console logger.
func (c *CYSimpleLogConsole) CloseLog() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bInit = false
}

// Compile-time interface checks.
var (
	_ ISimpleLog = (*CYSimpleLogFile)(nil)
	_ ISimpleLog = (*CYSimpleLogConsole)(nil)
)
