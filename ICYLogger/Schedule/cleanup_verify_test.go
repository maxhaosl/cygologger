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

package Schedule

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	Core "github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
)

// writeFile creates a file with the given content (and optional mtime).
func writeFile(t *testing.T, path string, content []byte, modTime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	if !modTime.IsZero() {
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}
}

// countGroup returns the number of .log files whose base name starts with prefix.
func countGroup(dir, prefix string) int {
	n := 0
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(p) == ".log" && strings.HasPrefix(filepath.Base(p), prefix) {
			n++
		}
		return nil
	})
	return n
}

func pad2(i int) string { return fmt.Sprintf("%02d", i) }

// TestCleanupExpiredFiles verifies LOG_TIME_EXPIRED_FILE: a file older than the
// retention window is removed while a fresh file is kept.
func TestCleanupExpiredFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Old_20240101.log"), []byte("old"), time.Now().Add(-25*time.Hour))
	writeFile(t, filepath.Join(dir, "Fresh_20240101.log"), []byte("new"), time.Now())
	writeFile(t, filepath.Join(dir, "Other_20240101.log"), []byte("x"), time.Now())

	cl := NewCYLoggerClearLogFile(dir, Core.DefaultLogTimeExpiredFile) // 24h
	cl.SetRestriction(20, 500*1024*1024, 1024*1024*1024)
	cl.SetClearUnLogFile(false)
	cl.DoClear()

	if _, err := os.Stat(filepath.Join(dir, "Old_20240101.log")); !os.IsNotExist(err) {
		t.Errorf("expired file should have been removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "Fresh_20240101.log")); err != nil {
		t.Errorf("fresh file should be kept: %v", err)
	}
}

// TestCleanupCountPerType verifies LOG_COUNT_PER_TYPE: files per type are capped
// to the limit, deleting the oldest when exceeded.
func TestCleanupCountPerType(t *testing.T) {
	dir := t.TempDir()
	base := time.Now()
	for i := 0; i < 25; i++ { // real C++ default is 20
		p := filepath.Join(dir, "Info_"+pad2(i)+".log")
		writeFile(t, p, []byte("x"), base.Add(time.Duration(i)*time.Minute))
	}
	cl := NewCYLoggerClearLogFile(dir, Core.DefaultLogTimeExpiredFile)
	cl.SetRestriction(Core.DefaultLogCountPerType, 500*1024*1024, 1024*1024*1024)
	cl.SetClearUnLogFile(false)
	cl.DoClear()

	if got := countGroup(dir, "Info_"); got != 20 {
		t.Fatalf("expected 20 Info files after count limit, got %d", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "Info_"+pad2(0)+".log")); !os.IsNotExist(err) {
		t.Errorf("oldest Info file should have been removed")
	}
}

// TestCleanupPerTypeSize verifies LOG_CHECK_FILE_TYPE_SIZE: total size per type is
// capped (scaled-down threshold here for speed; the real constant is asserted).
func TestCleanupPerTypeSize(t *testing.T) {
	dir := t.TempDir()
	base := time.Now()
	for i := 0; i < 6; i++ { // 6 * 1000 bytes = 6000 > threshold 5000
		p := filepath.Join(dir, "Info_"+pad2(i)+".log")
		writeFile(t, p, bytes.Repeat([]byte("x"), 1000), base.Add(time.Duration(i)*time.Minute))
	}
	cl := NewCYLoggerClearLogFile(dir, Core.DefaultLogTimeExpiredFile)
	cl.SetRestriction(100, 5000, 1024*1024*1024) // count high, per-type size 5000
	cl.SetClearUnLogFile(false)
	cl.DoClear()

	var total int64
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasPrefix(filepath.Base(p), "Info_") {
			total += info.Size()
		}
		return nil
	})
	if total > 5000 {
		t.Fatalf("per-type size not enforced: total=%d > 5000", total)
	}
	if total == 0 {
		t.Fatalf("per-type size cleanup removed everything")
	}
	// Constant fidelity: the real C++ default must be 500MB.
	if Core.DefaultLogCheckFileTypeSize != 1024*1024*500 {
		t.Errorf("LOG_CHECK_FILE_TYPE_SIZE default mismatch")
	}
}

// TestCleanupAllSize verifies LOG_CHECK_FILE_ALL_SIZE: global total size is capped.
func TestCleanupAllSize(t *testing.T) {
	dir := t.TempDir()
	base := time.Now()
	for i := 0; i < 3; i++ {
		writeFile(t, filepath.Join(dir, "Info_"+pad2(i)+".log"), bytes.Repeat([]byte("x"), 1000), base.Add(time.Duration(i)*time.Minute))
		writeFile(t, filepath.Join(dir, "Debug_"+pad2(i)+".log"), bytes.Repeat([]byte("x"), 1000), base.Add(time.Duration(i+10)*time.Minute))
	}
	cl := NewCYLoggerClearLogFile(dir, Core.DefaultLogTimeExpiredFile)
	cl.SetRestriction(100, 1024*1024*500, 3000) // global limit 3000
	cl.SetClearUnLogFile(false)
	cl.DoClear()

	total := int64(0)
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(p) == ".log" {
			total += info.Size()
		}
		return nil
	})
	if total > 3000 {
		t.Fatalf("global size not enforced: total=%d > 3000", total)
	}
}

// TestCleanupNonLogFileEnabled verifies LOG_LIMIT_CLEAR_UNLOGFILE: stray non-log
// files (e.g. package zips) are purged on the first pass.
func TestCleanupNonLogFileEnabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "stray.zip"), []byte("pkg"), time.Now())
	writeFile(t, filepath.Join(dir, "notes.txt"), []byte("x"), time.Now())
	cl := NewCYLoggerClearLogFile(dir, Core.DefaultLogTimeExpiredFile)
	cl.SetRestriction(20, 500*1024*1024, 1024*1024*1024)
	cl.SetClearUnLogFile(true)
	cl.DoClear()
	if _, err := os.Stat(filepath.Join(dir, "stray.zip")); !os.IsNotExist(err) {
		t.Errorf("non-log zip should be removed when clear-unlog enabled")
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); !os.IsNotExist(err) {
		t.Errorf("non-log txt should be removed when clear-unlog enabled")
	}
}

// TestCleanupNonLogFileDisabled verifies the toggle actually protects non-log files.
func TestCleanupNonLogFileDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "stray.zip"), []byte("pkg"), time.Now())
	cl := NewCYLoggerClearLogFile(dir, Core.DefaultLogTimeExpiredFile)
	cl.SetRestriction(20, 500*1024*1024, 1024*1024*1024)
	cl.SetClearUnLogFile(false)
	cl.DoClear()
	if _, err := os.Stat(filepath.Join(dir, "stray.zip")); os.IsNotExist(err) {
		t.Errorf("non-log zip must be KEPT when clear-unlog disabled")
	}
}

// TestCleanupMasterSwitchDisabled verifies LOG_LIMIT_ENABLE: with the master
// switch off, NO cleanup (expired/count/size/non-log) runs at all.
func TestCleanupMasterSwitchDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Old_20240101.log"), []byte("old"), time.Now().Add(-25*time.Hour))
	for i := 0; i < 25; i++ {
		writeFile(t, filepath.Join(dir, "Info_"+pad2(i)+".log"), []byte("x"), time.Now().Add(time.Duration(i)*time.Minute))
	}
	writeFile(t, filepath.Join(dir, "stray.zip"), []byte("pkg"), time.Now())

	cl := NewCYLoggerClearLogFile(dir, Core.DefaultLogTimeExpiredFile)
	cl.SetRestriction(20, 500*1024*1024, 1024*1024*1024)
	cl.SetClearUnLogFile(true)
	cl.SetEnable(false)
	cl.DoClear()

	total := 0
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(p) == ".log" {
			total++
		}
		return nil
	})
	if total != 26 { // 25 Info + 1 Old, untouched
		t.Fatalf("with master switch off, nothing should be cleaned; got %d logs", total)
	}
	if _, err := os.Stat(filepath.Join(dir, "stray.zip")); os.IsNotExist(err) {
		t.Errorf("non-log file must be kept when master switch off")
	}
}

// TestCleanupIntervalGating verifies LOG_CHECK_FILE_SIZE_TIME and
// LOG_CHECK_FILE_COUNT_TIME: the size and count passes are gated by their
// intervals and only run unconditionally on the very first pass.
func TestCleanupIntervalGating(t *testing.T) {
	dir := t.TempDir()
	base := time.Now()
	for i := 0; i < 4; i++ { // 4 * 50 bytes: count 4 > 2, total 200 > 100
		p := filepath.Join(dir, "Info_"+pad2(i)+".log")
		writeFile(t, p, bytes.Repeat([]byte("x"), 50), base.Add(time.Duration(i)*time.Minute))
	}
	cl := NewCYLoggerClearLogFile(dir, Core.DefaultLogTimeExpiredFile)
	cl.SetRestriction(2, 100, 1024*1024*1024) // count limit 2, per-type size 100
	cl.SetCheckFileSizeTime(100)
	cl.SetCheckFileCountTime(100)
	cl.SetClearUnLogFile(false)

	// First pass: both policies run unconditionally.
	cl.DoClear()
	if got := countGroup(dir, "Info_"); got != 2 {
		t.Fatalf("after first pass expected 2 Info files, got %d", got)
	}

	// Add 3 more files (now 5 total, exceeding both limits).
	for i := 4; i < 7; i++ {
		p := filepath.Join(dir, "Info_"+pad2(i)+".log")
		writeFile(t, p, bytes.Repeat([]byte("x"), 50), base.Add(time.Duration(i)*time.Minute))
	}
	// Immediately run again: 100s intervals not elapsed -> no trimming.
	cl.DoClear()
	if got := countGroup(dir, "Info_"); got != 5 {
		t.Fatalf("gated pass must NOT trim: expected 5, got %d", got)
	}

	// Simulate the interval having elapsed, then re-run: both policies run again.
	cl.lastSizeCheck = time.Now().Add(-200 * time.Second)
	cl.lastCountCheck = time.Now().Add(-200 * time.Second)
	cl.DoClear()
	if got := countGroup(dir, "Info_"); got != 2 {
		t.Fatalf("after interval elapsed expected 2, got %d", got)
	}
}
