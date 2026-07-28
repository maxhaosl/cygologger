// Command mount_main_test verifies the WithMountMain switch:
//
//   - WithMountMain(false) keeps a STRICT per-level file set with NO Main.log,
//     recreating the historical Debug (4 files: Trace/Info/Warn/Error) and
//     Release (1 file: Error) strict sets without an aggregate Main file.
//   - WithMountMain(true) (the default, backward compatible) still mounts
//     Main.log and aggregates every enabled level's messages into it.
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

var (
	msgs = map[string]string{
		"Trace": "trace-line", "Debug": "debug-line", "Info": "info-line",
		"Warn": "warn-line", "Error": "error-line", "Fatal": "fatal-line",
	}
	letters = map[string]string{
		"Trace": "T", "Debug": "D", "Info": "I", "Warn": "W", "Error": "E", "Fatal": "F",
	}
	allLevels = []string{"Trace", "Debug", "Info", "Warn", "Error", "Fatal"}
)

// runCase initialises with the given level filter and mountMain switch, writes
// one line per level, then asserts which files exist and that Main is gated by
// the switch.
func runCase(name string, filter Core.ELogLevelFilter, mountMain bool, enabled, disabled []string, expectMain bool) {
	fmt.Printf("\n=== Case: %s (filter=0x%X, mountMain=%v) ===\n", name, int(filter), mountMain)

	dir, err := os.MkdirTemp("", "cymm_")
	if err != nil {
		check(false, "MkdirTemp: %v", err)
		return
	}
	defer os.RemoveAll(dir)

	// Pass the full baseline every time: GetCYLoggerConfigInstance is a sync.Once
	// singleton, so unset options are NOT reset between Inits.
	ok := gologger.InitDefaultWithOpts(dir,
		gologger.WithConsole(false),
		gologger.WithFileMode(Core.LogFileModeAppend),
		gologger.WithLayoutType(Core.LogLayoutTypeBuildin1),
		gologger.WithLogLevel(filter),
		gologger.WithMountMain(mountMain),
	)
	if !ok {
		check(false, "InitDefaultWithOpts failed")
		return
	}

	gologger.Trace(msgs["Trace"])
	gologger.Debug(msgs["Debug"])
	gologger.Info(msgs["Info"])
	gologger.Warn(msgs["Warn"])
	gologger.Error(msgs["Error"])
	gologger.Fatal(msgs["Fatal"])
	gologger.Flush()
	time.Sleep(50 * time.Millisecond)

	// 1) Per-level files follow the filter (identical to level_filter_test).
	for _, t := range enabled {
		check(fileExists(dir, t), "file %s.log should EXIST (enabled by filter)", t)
	}
	for _, t := range disabled {
		check(!fileExists(dir, t), "file %s.log should NOT exist (filtered out)", t)
	}

	// 2) Main.log is gated by the mountMain switch.
	if expectMain {
		check(fileExists(dir, "Main"), "Main.log SHOULD exist (mountMain=true)")
		c := readFile(dir, "Main")
		for _, e := range enabled {
			check(strings.Contains(c, msgs[e]),
				"Main.log contains enabled %s message", e)
		}
		for _, d := range disabled {
			check(!strings.Contains(c, msgs[d]),
				"Main.log must NOT contain filtered %s message", d)
		}
	} else {
		check(!fileExists(dir, "Main"), "Main.log must NOT exist (mountMain=false) — strict file set")
	}

	// 3) Content correctness (markers + no cross-type leak) for enabled files.
	for _, t := range enabled {
		c := readFile(dir, t)
		check(strings.Contains(c, msgs[t]), "%s.log contains its own message", t)
		check(strings.Contains(c, "|"+letters[t]+"|"),
			"%s.log has correct level marker |%s|", t, letters[t])
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
	fmt.Println("=== CYGoLogger WithMountMain verification ===")

	// Strict Debug set: Trace/Info/Warn/Error (4 files), no Main.log.
	runCase("MountMain(false)+DebugStrict",
		Core.ELogLevelFilter(Core.LogLevelTrace|Core.LogLevelInfo|Core.LogLevelWarn|Core.LogLevelError),
		false,
		[]string{"Trace", "Info", "Warn", "Error"},
		[]string{"Debug", "Fatal"},
		false)

	// Strict Release set: Error only (1 file), no Main.log.
	runCase("MountMain(false)+ReleaseStrict",
		Core.ELogLevelFilter(Core.LogLevelError),
		false,
		[]string{"Error"},
		[]string{"Trace", "Debug", "Info", "Warn", "Fatal"},
		false)

	// All levels enabled but Main suppressed: 6 level files, no Main.log.
	runCase("MountMain(false)+LogFilterAll",
		gologger.LogFilterAll,
		false,
		allLevels,
		nil,
		false)

	// Default (mountMain=true): Main.log is still mounted (backward compatible).
	runCase("MountMain(true)+LogFilterWarnsAndErrors",
		gologger.LogFilterWarnsAndErrors,
		true,
		[]string{"Info", "Warn", "Error", "Fatal"},
		[]string{"Trace", "Debug"},
		true)

	fmt.Println()
	if failures == 0 {
		fmt.Println("=== ALL CHECKS PASSED ===")
		os.Exit(0)
	}
	fmt.Printf("=== %d CHECK(S) FAILED ===\n", failures)
	os.Exit(1)
}
