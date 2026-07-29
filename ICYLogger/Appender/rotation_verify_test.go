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

package Appender

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	Common "github.com/maxhaosl/cygologger/ICYLogger/Common"
	Core "github.com/maxhaosl/cygologger/ICYLogger/Core"
)

func newTestFileAppender(t *testing.T, dir string, fr *Common.CYFileRestriction) *CYLoggerFileAppender {
	t.Helper()
	a := newFileAppender(Core.LogTypeInfo, "", "Info.log", dir, Core.LogFileModeAppend)
	a.SetRestriction(fr)
	if !a.Init() {
		t.Fatal("appender Init failed")
	}
	t.Cleanup(func() { a.UnInit() })
	return a
}

func countDirLogs(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			n++
		}
	}
	return n
}

// TestSizeBasedRotationCreatesNewFile verifies LOG_CHECK_FILE_SIZE: once a single
// file exceeds the per-file limit, a fresh log file is created automatically.
func TestSizeBasedRotationCreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	fr := Common.NewCYFileRestriction()
	fr.SetRestriction(true, true, 60, 24, 300, 60, 100, 20, 500*1024*1024, 1024*1024*1024) // 100-byte limit
	a := newTestFileAppender(t, dir, fr)

	for i := 0; i < 3; i++ {
		a.doWrite(strings.Repeat("x", 60)) // each 60 bytes; after >=100 the file rotates
	}
	a.Flush()

	if logs := countDirLogs(t, dir); logs < 2 {
		t.Fatalf("expected >=2 log files after size-based rotation, got %d", logs)
	}
}

// TestMasterSwitchDisablesRotation verifies LOG_LIMIT_ENABLE: with the master
// detection switch off, no size-based rotation happens even past the limit.
func TestMasterSwitchDisablesRotation(t *testing.T) {
	dir := t.TempDir()
	fr := Common.NewCYFileRestriction()
	fr.SetRestriction(false, true, 60, 24, 300, 60, 1, 20, 500*1024*1024, 1024*1024*1024) // enable=false, 1-byte limit
	a := newTestFileAppender(t, dir, fr)

	for i := 0; i < 50; i++ {
		a.doWrite(strings.Repeat("x", 100))
	}
	a.Flush()

	if logs := countDirLogs(t, dir); logs != 1 {
		t.Fatalf("with master switch off, expected exactly 1 file, got %d", logs)
	}
}

// TestTimeModeRotationNoDoubleTimestamp is a regression test for the
// double-timestamp bug: in LogFileModeTime the active file name already carries
// a start timestamp (e.g. "Info_20060102_150405.log"); rotation must NOT append
// a second full timestamp (which produced names like
// "Info_20060102_150405.20060102_150406.000000.log"). It must instead use a
// short incrementing sequence suffix (".1", ".2", ...).
func TestTimeModeRotationNoDoubleTimestamp(t *testing.T) {
	dir := t.TempDir()
	fr := Common.NewCYFileRestriction()
	fr.SetRestriction(true, true, 60, 24, 300, 60, 100, 20, 500*1024*1024, 1024*1024*1024) // 100-byte limit
	// LogFileModeTime + empty file name -> the appender derives a timestamped name.
	a := newFileAppender(Core.LogTypeInfo, "", "", dir, Core.LogFileModeTime)
	a.SetRestriction(fr)
	if !a.Init() {
		t.Fatal("appender Init failed")
	}
	t.Cleanup(func() { a.UnInit() })

	// Trigger a few size-based rotations.
	for i := 0; i < 5; i++ {
		a.doWrite(strings.Repeat("x", 60))
	}
	a.Flush()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	sawRotated := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		// The rotated (archived) files are the ones with a sequence suffix
		// like "Info_<ts>.1.log". No archived name may contain two timestamp
		// groups. A timestamp group is "_HHMMSS" style; the double-timestamp
		// bug produced a second ".20060102_150405.000000" segment, i.e. a '.'
		// immediately followed by 8 digits then '_'. Assert that never happens.
		trimmed := strings.TrimSuffix(name, ".log")
		// Count occurrences of the "_150405" date-part pattern is brittle;
		// instead assert the buggy shape ".<8digits>_" is absent.
		for i := 0; i+9 < len(trimmed); i++ {
			if trimmed[i] == '.' {
				allDigit := true
				for k := 1; k <= 8; k++ {
					if trimmed[i+k] < '0' || trimmed[i+k] > '9' {
						allDigit = false
						break
					}
				}
				if allDigit && trimmed[i+9] == '_' {
					t.Fatalf("rotated name %q contains a second timestamp segment (double-timestamp bug)", name)
				}
			}
		}
		// Detect the new-style archive suffix ".<n>".
		if idx := strings.LastIndex(trimmed, "."); idx >= 0 {
			suf := trimmed[idx+1:]
			isNum := suf != ""
			for _, c := range suf {
				if c < '0' || c > '9' {
					isNum = false
					break
				}
			}
			if isNum {
				sawRotated = true
			}
		}
	}
	if !sawRotated {
		t.Fatalf("expected at least one sequence-suffixed archive (e.g. Info_<ts>.1.log) after rotation; entries: %v", entries)
	}
}

// TestCheckTimeRotationHonorsEnableCheck is a regression test for Bug E: the
// periodic (per-minute) rotation ticker must not rotate by size when the master
// detection switch (LOG_LIMIT_ENABLE) is disabled.
func TestCheckTimeRotationHonorsEnableCheck(t *testing.T) {
	dir := t.TempDir()
	fr := Common.NewCYFileRestriction()
	fr.SetRestriction(false, true, 60, 24, 300, 60, 1, 20, 500*1024*1024, 1024*1024*1024)
	a := newTestFileAppender(t, dir, fr)

	// Grow the file past the threshold via doWrite: with the master switch OFF
	// doWrite never rotates, and it keeps the in-memory size counter (used by
	// checkTimeRotation since the buffered-write optimisation) in sync.
	a.doWrite(strings.Repeat("y", 5000))
	a.Flush()
	// With the master switch OFF the file count must stay at 1 (no rotation).
	if logs := countDirLogs(t, dir); logs != 1 {
		t.Fatalf("with IsEnableCheck()==false, expected 1 file, got %d", logs)
	}
	a.checkTimeRotation()
	if logs := countDirLogs(t, dir); logs != 1 {
		t.Fatalf("checkTimeRotation must NOT rotate when IsEnableCheck()==false (got %d)", logs)
	}

	// Now enable and re-run: it must rotate, leaving 2 files (rotated + fresh).
	fr.SetRestriction(true, true, 60, 24, 300, 60, 1, 20, 500*1024*1024, 1024*1024*1024)
	a.checkTimeRotation()
	if logs := countDirLogs(t, dir); logs != 2 {
		t.Fatalf("checkTimeRotation must rotate after IsEnableCheck()==true (got %d)", logs)
	}
}
