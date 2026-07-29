/*
 * cygologger License
 * -----------
 *
 * cygologger is licensed under the terms of the MIT license reproduced below.
 * This means that cygologger is free software and can be used for both academic
 * and commercial purposes at absolutely no cost.
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

package Appender

import (
	"testing"

	Core "github.com/maxhaosl/cygologger/ICYLogger/Core"
)

// TestCreateAppenderNoneReturnsConsole verifies that LOG_TYPE_NONE maps to a
// console appender (mirroring C++ CreateFileAppender's LOG_TYPE_NONE branch),
// and that the resulting appender is runnable and reports its type as NONE.
func TestCreateAppenderNoneReturnsConsole(t *testing.T) {
	f := GetCYLoggerAppenderFactoryInstance()
	app := f.CreateAppender(Core.LogTypeNone, "test", "test", t.TempDir(), Core.LogFileModeAppend)
	if app == nil {
		t.Fatal("CreateAppender(LogTypeNone) returned nil")
	}
	defer app.UnInit()

	ca, ok := app.(*CYLoggerConsoleAppender)
	if !ok {
		t.Fatalf("CreateAppender(LogTypeNone) returned %T, want *CYLoggerConsoleAppender", app)
	}
	if ca.GetLogType() != Core.LogTypeNone {
		t.Errorf("console appender log type = %v, want LogTypeNone", ca.GetLogType())
	}
	if !ca.IsRunning() {
		t.Errorf("console appender should be running after creation")
	}
}

// TestConfigGetExePath verifies GetExePath returns a non-empty directory on the
// current platform (Windows: executable dir; others: working dir fallback).
func TestConfigGetExePath(t *testing.T) {
	c := Core.GetCYLoggerConfigInstance()
	path := c.GetExePath()
	if path == "" {
		t.Errorf("GetExePath returned empty string")
	}
}
