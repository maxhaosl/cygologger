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
 * furnished to other dealings in the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

// Command config_verify performs an isolated, per-process verification of the
// five C++ CYLogger config options. Each invocation validates ONE option
// (selected by -opt=...), so every check runs in a fresh process with its own
// singleton state — mirroring a real application that calls InitDefaultWithOpts
// once at startup.
//
// Each option is verified by ENABLING it and asserting the run-time effect, plus
// checking the corresponding config getter. A separate "-opt=defaults" run
// asserts the freshly-started (never-configured) defaults match the C++ literals.
//
// Usage:
//
//	go run . -opt=console        # LOG_SHOW_CONSOLE_WINDOW
//	go run . -opt=remote         # LOG_WRITE_REMOTE
//	go run . -opt=sys            # LOG_WRITE_SYS
//	go run . -opt=filemode       # LOG_FILE_MODE (append vs time)
//	go run . -opt=layout         # LOG_LAYOUT_TYPE
//	go run . -opt=defaults       # all five C++ defaults
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	gologger "github.com/maxhaosl/cygologger/ICYLogger"
	Appender "github.com/maxhaosl/cygologger/ICYLogger/Appender"
	Core "github.com/maxhaosl/cygologger/ICYLogger/Core"
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

func appenderCount(eType Core.ELogType) int {
	return len(Appender.GetCYLoggerAppenderFactoryInstance().GetAppenders(eType))
}

func main() {
	opt := flag.String("opt", "", "which option to verify: console|remote|sys|filemode|layout|defaults")
	flag.Parse()

	dir, _ := os.MkdirTemp("", "cygo_cfg_verify_")
	defer os.RemoveAll(dir)

	switch *opt {
	case "console":
		// LOG_SHOW_CONSOLE_WINDOW=true -> messages are written to stdout.
		// Capture stdout to prove the console appender is actually mounted.
		r, w, _ := os.Pipe()
		old := os.Stdout
		os.Stdout = w
		gologger.InitDefaultWithOpts(dir, gologger.WithConsole(true))
		gologger.Info("console-probe-line")
		gologger.Flush()
		gologger.Close()
		w.Close()
		os.Stdout = old
		out, _ := io.ReadAll(r)
		check("LOG_SHOW_CONSOLE_WINDOW=true -> config IsShowConsole()==true",
			gologger.GetCYLoggerConfigInstance().IsShowConsole())
		check("LOG_SHOW_CONSOLE_WINDOW=true -> message written to stdout",
			strings.Contains(string(out), "console-probe-line"))

	case "remote":
		// LOG_WRITE_REMOTE=true -> remote appender mounted.
		gologger.InitDefaultWithOpts(dir, gologger.WithConsole(false), gologger.WithWriteRemote(true))
		check("LOG_WRITE_REMOTE=true -> config IsWriteRemote()==true",
			gologger.GetCYLoggerConfigInstance().IsWriteRemote())
		check("LOG_WRITE_REMOTE=true -> 1 remote appender mounted", appenderCount(Core.LogTypeRemote) == 1)
		gologger.Close()

	case "sys":
		// LOG_WRITE_SYS=true -> system (syslog on Unix) appender mounted.
		gologger.InitDefaultWithOpts(dir, gologger.WithConsole(false), gologger.WithWriteSys(true))
		check("LOG_WRITE_SYS=true -> config IsWriteSys()==true",
			gologger.GetCYLoggerConfigInstance().IsWriteSys())
		check("LOG_WRITE_SYS=true -> 1 sys appender mounted", appenderCount(Core.LogTypeSys) == 1)
		gologger.Close()

	case "filemode":
		// LOG_FILE_MODE=append -> fixed Info.log (no timestamp).
		gologger.InitDefaultWithOpts(dir, gologger.WithConsole(false), gologger.WithFileMode(Core.LogFileModeAppend))
		gologger.Info("append-mode-line")
		gologger.Flush()
		gologger.Close()
		_, errA := os.Stat(filepath.Join(dir, "Info.log"))
		check("LOG_FILE_MODE=append -> fixed Info.log created", errA == nil)
		check("LOG_FILE_MODE=append -> no timestamped Info file", !hasTimestampedInfo(dir))

		// LOG_FILE_MODE=time -> timestamped Info_YYYYMMDD_HHMMSS.log.
		gologger.InitDefaultWithOpts(dir, gologger.WithConsole(false), gologger.WithFileMode(Core.LogFileModeTime))
		gologger.Info("time-mode-line")
		gologger.Flush()
		gologger.Close()
		check("LOG_FILE_MODE=time -> timestamped Info file created", hasTimestampedInfo(dir))

	case "layout":
		// LOG_LAYOUT_TYPE=Buildin3 -> short "HH:MM:SS" timestamp, no full date.
		gologger.InitDefaultWithOpts(dir, gologger.WithConsole(false), gologger.WithLayoutType(Core.LogLayoutTypeBuildin3))
		check("LOG_LAYOUT_TYPE=Buildin3 -> config GetLayoutType()==Buildin3",
			gologger.GetCYLoggerConfigInstance().GetLayoutType() == Core.LogLayoutTypeBuildin3)
		gologger.Info("layout-probe-line")
		gologger.Flush()
		gologger.Close()

		content := readInfoLog(dir)
		check("LOG_LAYOUT_TYPE=Buildin3 -> log body present", strings.Contains(content, "layout-probe-line"))
		check("LOG_LAYOUT_TYPE=Buildin3 -> applied (no full-date Buildin1 format)",
			!strings.Contains(content, "2026-"))

	case "defaults":
		// Fresh process, no Init yet: defaults must match the C++ literals.
		cfg := gologger.GetCYLoggerConfigInstance()
		check("default LOG_SHOW_CONSOLE_WINDOW == false", cfg.IsShowConsole() == false)
		check("default LOG_WRITE_REMOTE == false", cfg.IsWriteRemote() == false)
		check("default LOG_WRITE_SYS == false", cfg.IsWriteSys() == false)
		check("default LOG_FILE_MODE == LogFileModeTime", cfg.GetFileMode() == Core.LogFileModeTime)
		check("default LOG_LAYOUT_TYPE == LogLayoutTypeBuildin1", cfg.GetLayoutType() == Core.LogLayoutTypeBuildin1)

	default:
		fmt.Fprintf(os.Stderr, "unknown -opt %q (console|remote|sys|filemode|layout|defaults)\n", *opt)
		os.Exit(2)
	}

	fmt.Printf("\n=== RESULT: %d passed, %d failed ===\n", pass, fail)
	if fail > 0 {
		os.Exit(1)
	}
}

func hasTimestampedInfo(dir string) bool {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "Info_") && strings.HasSuffix(e.Name(), ".log") {
			return true
		}
	}
	return false
}

func readInfoLog(dir string) string {
	var sb strings.Builder
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "Info") && strings.HasSuffix(e.Name(), ".log") {
			if b, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
				sb.Write(b)
			}
		}
	}
	return sb.String()
}
