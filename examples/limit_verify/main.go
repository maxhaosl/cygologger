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

// Command limit_verify performs a live, end-to-end verification of the ten C++
// ICYLoggerDefine restriction features against the running Go logger:
//   - all 10 knobs are reachable through the public config getters;
//   - a single log file that reaches LOG_CHECK_FILE_SIZE (5MB) automatically
//     rolls over to a new file;
//   - the background schedule automatically cleans expired files, trims
//     per-type file counts, and purges non-log (package) files, while keeping
//     the currently-written file intact.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gologger "github.com/maxhaosl/cygologger/ICYLogger"
	Core "github.com/maxhaosl/cygologger/ICYLogger/Core"
	Schedule "github.com/maxhaosl/cygologger/ICYLogger/Schedule"
)

var pass, fail int

func check(name string, ok bool) {
	if ok {
		pass++
		fmt.Printf("  [PASS] %s\n", name)
	} else {
		fail++
		fmt.Printf("  [FAIL] %s\n", name)
	}
}

func listLogCount(dir, prefix string) int {
	n := 0
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".log") {
			n++
		}
	}
	return n
}

func main() {
	fmt.Println("=== C++ restriction features: live end-to-end verification ===")

	dir, _ := os.MkdirTemp("", "cygo_limit_verify")
	defer os.RemoveAll(dir)

	// Exact C++ default values. LOG_TIME_CLEAR_LOG is set to 2 (instead of the
	// C++ default 60) ONLY so the background cleanup tick is observable quickly;
	// Core.DefaultLogTimeClearLog still asserts the real C++ default of 60 below.
	const (
		cppTimeClearLog      = 2 // C++ default is 60; lowered only to observe a tick fast
		cppTimeExpiredFile   = 24
		cppCheckSizeTime     = 60 * 5
		cppCheckCountTime    = 60
		cppCheckFileSize     = 1024 * 1024 * 5
		cppCountPerType      = 20
		cppCheckFileTypeSize = 1024 * 1024 * 500
		cppCheckAllFileSize  = 1024 * 1024 * 1024
	)

	gologger.InitDefaultWithOpts(dir,
		gologger.WithConsole(false),
		gologger.WithFileMode(Core.LogFileModeAppend),
		gologger.WithRestriction(true, true,
			cppTimeClearLog, cppTimeExpiredFile, cppCheckSizeTime, cppCheckCountTime,
			cppCheckFileSize, cppCountPerType, cppCheckFileTypeSize, cppCheckAllFileSize),
	)
	defer gologger.Close()

	// 1) Config wiring: all ten C++ knobs reachable via getters.
	cfg := gologger.GetCYLoggerConfigInstance()
	check("LOG_LIMIT_ENABLE -> IsLimitEnable()==true", cfg.IsLimitEnable())
	check("LOG_LIMIT_CLEAR_UNLOGFILE -> IsClearUnLogFile()==true", cfg.IsClearUnLogFile())
	check("LOG_TIME_CLEAR_LOG configured == 2 (fast tick)", cfg.GetTimeClearLog() == cppTimeClearLog)
	check("LOG_TIME_EXPIRED_FILE == 24", cfg.GetTimeExpiredFile() == 24)
	check("LOG_CHECK_FILE_SIZE_TIME == 300", cfg.GetCheckFileSizeTime() == 300)
	check("LOG_CHECK_FILE_COUNT_TIME == 60", cfg.GetCheckFileCountTime() == 60)
	check("LOG_CHECK_FILE_SIZE == 5MB", cfg.GetCheckFileSize() == 1024*1024*5)
	check("LOG_COUNT_PER_TYPE == 20", cfg.GetCountPerType() == 20)
	check("LOG_CHECK_FILE_TYPE_SIZE == 500MB", cfg.GetCheckFileTypeSize() == 1024*1024*500)
	check("LOG_CHECK_FILE_ALL_SIZE == 1GB", cfg.GetCheckAllFileSize() == 1024*1024*1024)
	check("Core.DefaultLogTimeClearLog == 60 (real C++ default)", Core.DefaultLogTimeClearLog == 60)

	// 2) Auto-create a new file when a single file reaches the size limit.
	probe := strings.Repeat("P", 200)
	for i := 0; i < 30000; i++ { // ~ (200 + header) * 30000 >> 5MB
		gologger.Info("%s-%05d", probe, i)
	}
	gologger.Flush()
	infoFiles := listLogCount(dir, "Info")
	check("size-based rotation created a new Info file (Info*.log > 1)", infoFiles > 1)

	// 3) Auto cleanup: plant an expired, an over-count, and a non-log file.
	expired := filepath.Join(dir, "Old_20240101_000000.log")
	_ = os.WriteFile(expired, []byte("expired"), 0644)
	_ = os.Chtimes(expired, time.Now().Add(-25*time.Hour), time.Now().Add(-25*time.Hour))
	for i := 0; i < 25; i++ {
		_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("Warn_%02d.log", i)), []byte("w"), 0644)
	}
	stray := filepath.Join(dir, "leftover_package.zip")
	_ = os.WriteFile(stray, []byte("pkg"), 0644)

	// Speed up the schedule tick so cleanup is observable (period already 2s from
	// the LOG_TIME_CLEAR_LOG value above; this is just an explicit reminder).
	Schedule.GetCYLoggerScheduleInstance().SetClearPeriodSec(2)

	// Wait for at least one cleanup tick.
	time.Sleep(3 * time.Second)

	if _, err := os.Stat(expired); !os.IsNotExist(err) {
		check("expired log file auto-removed", false)
	} else {
		check("expired log file auto-removed", true)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		check("non-log file (zip) auto-removed", false)
	} else {
		check("non-log file (zip) auto-removed", true)
	}
	check("per-type count limit enforced (Warn_ files <= 20)", listLogCount(dir, "Warn_") <= 20)
	if _, err := os.Stat(filepath.Join(dir, "Info.log")); err != nil {
		check("current Info.log protected from cleanup", false)
	} else {
		check("current Info.log protected from cleanup", true)
	}

	fmt.Printf("\n=== RESULT: %d passed, %d failed ===\n", pass, fail)
	if fail > 0 {
		os.Exit(1)
	}
}
