// Command level_filter_test verifies that the LOG_LEVEL_FILTER (ELogLevelFilter)
// controls BOTH whether a log-type file is generated AND whether the correct
// content is written:
//
//  1. A level disabled by the filter must NOT produce its dedicated .log file.
//  2. An enabled level must produce its file, with the correct level marker
//     (|T|/|D|/|I|/|W|/|E|/|F|) and its own message, and must NOT leak any other
//     level's message (no cross-type contamination, no filtered content).
//  3. The Main aggregate log must contain exactly the enabled levels' messages.
//
// Run: go run .   (exits non-zero on any failure so it can gate CI).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
)

var failures int

func check(cond bool, format string, args ...any) {
	if cond {
		fmt.Printf("  PASS: "+format+"\n", args...)
		return
	}
	fmt.Printf("  FAIL: "+format+"\n", args...)
	failures++
}

func fileExists(dir, name string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, name+".log"))
	return len(matches) > 0
}

func readFile(dir, name string) string {
	data, err := os.ReadFile(filepath.Join(dir, name+".log"))
	if err != nil {
		return ""
	}
	return string(data)
}

// Per-level message text and layout type marker.
var (
	msgs = map[string]string{
		"Trace": "trace-line", "Debug": "debug-line", "Info": "info-line",
		"Warn": "warn-line", "Error": "error-line", "Fatal": "fatal-line",
	}
	letters = map[string]string{
		"Trace": "T", "Debug": "D", "Info": "I", "Warn": "W", "Error": "E", "Fatal": "F",
	}
)

// runCase initialises the logger with the given filter, writes one line of every
// level, then asserts file existence and content correctness.
func runCase(name string, filter Core.ELogLevelFilter, enabled, disabled []string) {
	fmt.Printf("\n=== Case: %s (filter=0x%X) ===\n", name, int(filter))

	dir, err := os.MkdirTemp("", "cyllf_")
	if err != nil {
		check(false, "MkdirTemp: %v", err)
		return
	}
	defer os.RemoveAll(dir)

	ok := gologger.InitDefaultWithOpts(dir,
		gologger.WithConsole(false),
		gologger.WithFileMode(Core.LogFileModeAppend),
		gologger.WithLayoutType(Core.LogLayoutTypeBuildin1),
		gologger.WithLogLevel(filter),
	)
	if !ok {
		check(false, "InitDefaultWithOpts failed")
		return
	}

	// Emit exactly one line of every level. Fatal does NOT os.Exit here; it is a
	// normal level write, so the program continues.
	gologger.Trace(msgs["Trace"])
	gologger.Debug(msgs["Debug"])
	gologger.Info(msgs["Info"])
	gologger.Warn(msgs["Warn"])
	gologger.Error(msgs["Error"])
	gologger.Fatal(msgs["Fatal"])
	gologger.Flush()
	// Main/file appenders are synchronous, but give the console/buffer swap
	// loops a beat to settle (also matters under -race).
	time.Sleep(50 * time.Millisecond)

	// 1) File existence must follow the filter.
	for _, t := range enabled {
		check(fileExists(dir, t), "file %s.log should EXIST (enabled by filter)", t)
	}
	for _, t := range disabled {
		check(!fileExists(dir, t), "file %s.log should NOT exist (filtered out)", t)
	}

	// 2) Content correctness.
	for _, t := range enabled {
		if t == "Main" {
			c := readFile(dir, "Main")
			for _, e := range enabled {
				if e == "Main" {
					continue
				}
				check(strings.Contains(c, msgs[e]),
					"Main.log contains enabled %s message", e)
			}
			for _, d := range disabled {
				check(!strings.Contains(c, msgs[d]),
					"Main.log must NOT contain filtered %s message", d)
			}
			continue
		}
		c := readFile(dir, t)
		check(strings.Contains(c, msgs[t]), "%s.log contains its own message", t)
		check(strings.Contains(c, "|"+letters[t]+"|"),
			"%s.log has correct level marker |%s|", t, letters[t])
		// Must not leak any other level (enabled or disabled) into this file.
		for other, msg := range msgs {
			if other == t {
				continue
			}
			check(!strings.Contains(c, msg),
				"%s.log must NOT contain %s message (no cross-type leak)", t, other)
		}
	}

	gologger.UnInitLogger()
}

func main() {
	fmt.Println("=== CYGoLogger LOG_LEVEL_FILTER verification ===")

	// Only Error + Fatal (+Main) should appear.
	runCase("LogFilterErrors", gologger.LogFilterErrors,
		[]string{"Error", "Fatal", "Main"},
		[]string{"Trace", "Debug", "Info", "Warn"})

	// Info/Warn/Error/Fatal (+Main).
	runCase("LogFilterWarnsAndErrors", gologger.LogFilterWarnsAndErrors,
		[]string{"Info", "Warn", "Error", "Fatal", "Main"},
		[]string{"Trace", "Debug"})

	// Everything enabled — regression guard that the gating is a no-op for ALL.
	runCase("LogFilterAll", gologger.LogFilterAll,
		[]string{"Trace", "Debug", "Info", "Warn", "Error", "Fatal", "Main"},
		nil)

	fmt.Println()
	if failures == 0 {
		fmt.Println("=== ALL CHECKS PASSED ===")
		os.Exit(0)
	}
	fmt.Printf("=== %d CHECK(S) FAILED ===\n", failures)
	os.Exit(1)
}
