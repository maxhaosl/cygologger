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
	"runtime"
	"sync"
	"time"
)

// ============================================================================
// CYExceptionLogFile - crash / exception logging (Go equivalent of C++
// CYExceptionLogFile + EXCEPTION_BEGIN/EXCEPTION_END macros).
//
// In C++ the exception system catches thrown exceptions. In Go the idiomatic
// equivalent is panic/recover. This module:
//   - Provides a singleton exception log file (synchronous, via CYSimpleLogFile).
//   - Offers Recover() / RecoverGoroutine() defer helpers that capture a panic,
//     write it (with full stack trace) to the exception log, and optionally
//     invoke a user-supplied panic handler.
// ============================================================================

// PanicHandler is invoked when a panic is captured by Recover/RecoverGoroutine.
// recovered is the value passed to panic(); stack is the full goroutine stack.
type PanicHandler func(recovered any, stack string)

// CYExceptionLogFile is a singleton synchronous exception logger.
type CYExceptionLogFile struct {
	mu           sync.Mutex
	logFile      *CYSimpleLogFile
	panicHandler PanicHandler
	rethrow      bool // if true, Recover re-panics after logging
	inited       bool
}

var (
	g_CYExceptionLogFileInstance *CYExceptionLogFile
	g_CYExceptionLogFileOnce     sync.Once
)

// GetCYExceptionLogFileInstance returns the singleton exception logger.
func GetCYExceptionLogFileInstance() *CYExceptionLogFile {
	g_CYExceptionLogFileOnce.Do(func() {
		g_CYExceptionLogFileInstance = &CYExceptionLogFile{
			logFile: NewCYSimpleLogFile(),
		}
	})
	return g_CYExceptionLogFileInstance
}

// InitLog initializes the exception log file.
// logPath: base directory; the exception log is written to <logPath>/Exception.log.
func (e *CYExceptionLogFile) InitLog(logPath string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.inited {
		return true
	}
	if logPath == "" {
		logPath = GetCYPublicFunctionInstance().GetCurrentDir()
	}
	// Write time + line count prefixes; store under logPath (no extra sub-dir).
	ok := e.logFile.InitLog(true, true, logPath, ".", "Exception.log")
	e.inited = ok
	return ok
}

// SetPanicHandler registers a callback invoked whenever a panic is captured.
func (e *CYExceptionLogFile) SetPanicHandler(h PanicHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.panicHandler = h
}

// SetRethrow controls whether Recover re-panics after logging (default false).
func (e *CYExceptionLogFile) SetRethrow(b bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rethrow = b
}

// WriteLog writes an exception message with source location to the exception log.
func (e *CYExceptionLogFile) WriteLog(msg, file, location string, line int) bool {
	e.mu.Lock()
	handler := e.panicHandler
	inited := e.inited
	e.mu.Unlock()

	full := fmt.Sprintf("[EXCEPTION] %s (%s:%d %s)", msg, file, line, location)
	if inited {
		e.logFile.WriteString(full)
	}
	if handler != nil {
		handler(msg, "")
	}
	return inited
}

// handlePanic is the shared logic for Recover / RecoverGoroutine.
func (e *CYExceptionLogFile) handlePanic(recovered any) {
	if recovered == nil {
		return
	}
	stack := captureStack(3)

	e.mu.Lock()
	handler := e.panicHandler
	inited := e.inited
	rethrow := e.rethrow
	e.mu.Unlock()

	msg := fmt.Sprintf("[PANIC] %v\nTime: %s\nStack:\n%s",
		recovered, time.Now().Format("2006-01-02 15:04:05.000"), stack)

	if inited {
		e.logFile.WriteString(msg)
	}
	if handler != nil {
		handler(recovered, stack)
	}
	if rethrow {
		panic(recovered)
	}
}

// CloseLog closes the exception log file.
func (e *CYExceptionLogFile) CloseLog() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.logFile != nil {
		e.logFile.CloseLog()
	}
	e.inited = false
}

// ReportPanic logs an already-recovered panic value. It exists so that
// higher-level packages can implement their own `defer Xxx()` recovery helper
// (which must call the builtin recover() DIRECTLY inside the deferred function)
// and then delegate the logging/handler logic here.
func ReportPanic(recovered any) {
	if recovered == nil {
		return
	}
	GetCYExceptionLogFileInstance().handlePanic(recovered)
}

// captureStack returns the current goroutine stack trace, skipping `skip` frames.
func captureStack(skip int) string {
	buf := make([]byte, 8192)
	n := runtime.Stack(buf, false)
	_ = skip
	return string(buf[:n])
}

// ============================================================================
// Defer helpers (idiomatic Go panic recovery)
// ============================================================================

// Recover is a defer helper that captures a panic in the current goroutine,
// logs it (with stack trace) to the exception log, and invokes the registered
// panic handler. By default it swallows the panic (does not re-panic); use
// GetCYExceptionLogFileInstance().SetRethrow(true) to re-panic after logging.
//
// Usage:
//
//	func risky() {
//	    defer Common.Recover()
//	    // ... code that may panic ...
//	}
func Recover() {
	if r := recover(); r != nil {
		GetCYExceptionLogFileInstance().handlePanic(r)
	}
}

// RecoverGoroutine is identical to Recover but intended to be deferred at the
// top of a goroutine, ensuring background panics are logged rather than crashing
// the whole process silently.
func RecoverGoroutine() {
	if r := recover(); r != nil {
		GetCYExceptionLogFileInstance().handlePanic(r)
	}
}

// SafeGo runs fn in a new goroutine protected by RecoverGoroutine, so any panic
// inside fn is captured and logged.
func SafeGo(fn func()) {
	go func() {
		defer RecoverGoroutine()
		fn()
	}()
}
