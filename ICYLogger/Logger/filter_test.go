package logger

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
)

// TestLogLevelFilterSuppressesFileCreation verifies that a level disabled by
// LOG_LEVEL_FILTER (the effective ELogLevelFilter) produces neither output nor a
// dedicated log file, while enabled levels and the Main aggregate still appear.
// This mirrors the C++ LOG_LEVEL_FILTER semantics: a suppressed level is fully
// turned off (no file, no writes).
func TestLogLevelFilterSuppressesFileCreation(t *testing.T) {
	dir := t.TempDir()
	config := Core.GetCYLoggerConfigInstance()
	config.SetFileMode(Core.LogFileModeAppend) // deterministic file names
	config.SetLogLevelFilter(Core.LogFilterErrors)

	if !InitLogger(dir, false) {
		t.Fatalf("InitLogger failed")
	}
	t.Cleanup(func() {
		UnInitLogger()
		// Restore defaults so other tests (sharing the config singleton) are unaffected.
		config.SetLogLevelFilter(Core.DefaultLogLevelFilter)
			config.SetFileMode(Core.LogFileModeTime)
	})

	// Emit exactly one line per level.
	LOG_TRACE("trace")
	LOG_DEBUG("debug")
	LOG_INFO("info")
	LOG_WARN("warn")
	LOG_ERROR("error")
	LOG_FATAL("fatal")
	FlushLogger()
	time.Sleep(50 * time.Millisecond)

	mustExist := func(name string) {
		t.Helper()
		matches, _ := filepath.Glob(filepath.Join(dir, name+".log"))
		if len(matches) == 0 {
			t.Errorf("expected %s.log to exist (level enabled), but it was not created", name)
		}
	}
	mustNotExist := func(name string) {
		t.Helper()
		matches, _ := filepath.Glob(filepath.Join(dir, name+".log"))
		if len(matches) != 0 {
			t.Errorf("expected %s.log NOT to exist (level filtered out), but found: %v", name, matches)
		}
	}

	// Enabled by LogFilterErrors.
	mustExist("Error")
	mustExist("Fatal")
	mustExist("Main") // aggregate of enabled levels
	// Suppressed by LOG_LEVEL_FILTER — must not generate a file.
	mustNotExist("Trace")
	mustNotExist("Debug")
	mustNotExist("Info")
	mustNotExist("Warn")

	// The aggregate Main log must contain only the two enabled lines.
	mainContent := readLogs(t, dir, "Main.log")
	for _, lvl := range []string{"error", "fatal"} {
		if !strings.Contains(strings.ToLower(mainContent), lvl) {
			t.Errorf("Main.log missing enabled level %q: %q", lvl, mainContent)
		}
	}
	for _, lvl := range []string{"trace", "debug", "info", "warn"} {
		if strings.Contains(strings.ToLower(mainContent), lvl) {
			t.Errorf("Main.log unexpectedly contains filtered level %q: %q", lvl, mainContent)
		}
	}
}

// TestLogLevelFilterAllKeepsAllFiles verifies the default filter (LogFilterAll)
// still mounts every per-type file plus Main — i.e. the new gating is a no-op for
// the all-enabled default and does not regress existing behaviour.
func TestLogLevelFilterAllKeepsAllFiles(t *testing.T) {
	dir := t.TempDir()
	config := Core.GetCYLoggerConfigInstance()
	config.SetFileMode(Core.LogFileModeAppend)
	config.SetLogLevelFilter(Core.LogFilterAll)

	if !InitLogger(dir, false) {
		t.Fatalf("InitLogger failed")
	}
	t.Cleanup(func() {
		UnInitLogger()
		config.SetLogLevelFilter(Core.DefaultLogLevelFilter)
		config.SetFileMode(Core.LogFileModeTime)
	})

	LOG_TRACE("t")
	LOG_DEBUG("d")
	LOG_INFO("i")
	LOG_WARN("w")
	LOG_ERROR("e")
	LOG_FATAL("f")
	FlushLogger()
	time.Sleep(50 * time.Millisecond)

	for _, name := range []string{"Trace", "Debug", "Info", "Warn", "Error", "Fatal", "Main"} {
		matches, _ := filepath.Glob(filepath.Join(dir, name+".log"))
		if len(matches) == 0 {
			t.Errorf("expected %s.log to exist with LogFilterAll, but it was not created", name)
		}
	}
}

// TestLogLevelFilterWarnsAndErrors verifies the intermediate preset only mounts
// Info/Warn/Error/Fatal (+Main) and suppresses Trace/Debug files.
func TestLogLevelFilterWarnsAndErrors(t *testing.T) {
	dir := t.TempDir()
	config := Core.GetCYLoggerConfigInstance()
	config.SetFileMode(Core.LogFileModeAppend)
	config.SetLogLevelFilter(Core.LogFilterWarnsAndErrors)

	if !InitLogger(dir, false) {
		t.Fatalf("InitLogger failed")
	}
	t.Cleanup(func() {
		UnInitLogger()
		config.SetLogLevelFilter(Core.DefaultLogLevelFilter)
			config.SetFileMode(Core.LogFileModeTime)
	})

	LOG_TRACE("t")
	LOG_DEBUG("d")
	LOG_INFO("i")
	LOG_WARN("w")
	LOG_ERROR("e")
	LOG_FATAL("f")
	FlushLogger()
	time.Sleep(50 * time.Millisecond)

	mustExist := func(name string) {
		t.Helper()
		matches, _ := filepath.Glob(filepath.Join(dir, name+".log"))
		if len(matches) == 0 {
			t.Errorf("expected %s.log to exist, but it was not created", name)
		}
	}
	mustNotExist := func(name string) {
		t.Helper()
		matches, _ := filepath.Glob(filepath.Join(dir, name+".log"))
		if len(matches) != 0 {
			t.Errorf("expected %s.log NOT to exist, but found: %v", name, matches)
		}
	}

	mustExist("Info")
	mustExist("Warn")
	mustExist("Error")
	mustExist("Fatal")
	mustExist("Main")
	mustNotExist("Trace")
	mustNotExist("Debug")
}
