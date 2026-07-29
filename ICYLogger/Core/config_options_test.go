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
 * furnished to do other dealings in the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

package Core

import (
	"testing"
)

// TestCPPConfigOptionDefaultsMatch verifies that the five C++ CYLogger config
// options are mirrored exactly by the Go default constants:
//
//	LOG_SHOW_CONSOLE_WINDOW = false
//	LOG_WRITE_REMOTE        = false
//	LOG_WRITE_SYS           = false
//	LOG_FILE_MODE           = LOG_MODE_FILE_TIME   (1)
//	LOG_LAYOUT_TYPE         = LOG_LAYOUT_TYPE_BUILDIN_1 (1)
func TestCPPConfigOptionDefaultsMatch(t *testing.T) {
	cases := []struct {
		name string
		got  any
		want any
	}{
		{"LOG_SHOW_CONSOLE_WINDOW", DefaultLogShowConsoleWindow, false},
		{"LOG_WRITE_REMOTE", DefaultLogWriteRemote, false},
		{"LOG_WRITE_SYS", DefaultLogWriteSys, false},
		{"LOG_FILE_MODE", int(DefaultLogFileMode), int(LogFileModeTime)},
		{"LOG_LAYOUT_TYPE", int(DefaultLogLayoutType), int(LogLayoutTypeBuildin1)},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

// TestConfigOptionSettersGetters verifies each option's setter actually changes
// the value observable through its getter — i.e. the config plumbing works.
func TestConfigOptionSettersGetters(t *testing.T) {
	c := &CYLoggerConfig{}

	c.SetShowConsole(true)
	if !c.IsShowConsole() {
		t.Error("SetShowConsole(true) not reflected by IsShowConsole()")
	}
	c.SetShowConsole(false)
	if c.IsShowConsole() {
		t.Error("SetShowConsole(false) not reflected by IsShowConsole()")
	}

	c.SetWriteRemote(true)
	if !c.IsWriteRemote() {
		t.Error("SetWriteRemote(true) not reflected by IsWriteRemote()")
	}

	c.SetWriteSys(true)
	if !c.IsWriteSys() {
		t.Error("SetWriteSys(true) not reflected by IsWriteSys()")
	}

	c.SetFileMode(LogFileModeAppend)
	if c.GetFileMode() != LogFileModeAppend {
		t.Error("SetFileMode(Append) not reflected by GetFileMode()")
	}
	c.SetFileMode(LogFileModeTime)
	if c.GetFileMode() != LogFileModeTime {
		t.Error("SetFileMode(Time) not reflected by GetFileMode()")
	}

	c.SetLayoutType(LogLayoutTypeBuildin4)
	if c.GetLayoutType() != LogLayoutTypeBuildin4 {
		t.Error("SetLayoutType(Buildin4) not reflected by GetLayoutType()")
	}
}
