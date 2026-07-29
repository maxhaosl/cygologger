package ICYLogger

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	Core "github.com/maxhaosl/cygologger/ICYLogger/Core"
)

// TestEarlyLevelFilterDropsDisabledLevels verifies the api.go entry-point early
// filter: a level suppressed by the configured level filter is dropped BEFORE
// any caller capture / formatting work, so no file is created for it, while
// enabled levels still produce their files. This guards against a regression
// where the early filter would either fail to drop a disabled level or, worse,
// drop an enabled one.
func TestEarlyLevelFilterDropsDisabledLevels(t *testing.T) {
	dir, err := os.MkdirTemp("", "cygologger-early-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)

	// LogFilterWarnsAndErrors enables Info|Warn|Error|Fatal, disables Trace|Debug.
	if !InitDefaultWithOpts(dir, WithLogLevel(Core.LogFilterWarnsAndErrors), WithConsole(false)) {
		t.Fatalf("InitDefaultWithOpts failed")
	}
	defer Close()

	Trace("dropped trace")
	Debug("dropped debug")
	Info("keep info")
	Warn("keep warn")
	Error("keep error")

	Flush()
	time.Sleep(50 * time.Millisecond)

	mustNotExist := func(name string) {
		t.Helper()
		matches, _ := filepath.Glob(filepath.Join(dir, name+"_*.log"))
		if len(matches) != 0 {
			t.Errorf("expected %s log NOT to exist (level filtered), but found: %v", name, matches)
		}
	}
	mustExist := func(name string) {
		t.Helper()
		matches, _ := filepath.Glob(filepath.Join(dir, name+"_*.log"))
		if len(matches) == 0 {
			t.Errorf("expected %s log to exist, but it was not created", name)
		}
	}

	// Disabled by the filter — must not produce a file (early filter dropped them).
	mustNotExist("Trace")
	mustNotExist("Debug")
	// Enabled by the filter — must still produce their files.
	mustExist("Info")
	mustExist("Warn")
	mustExist("Error")
}

// TestEarlyLevelFilterAllKeepsEnabled verifies that with the all-enabled filter
// the early filter is a no-op: every level still produces its file, proving the
// guard never drops an enabled level.
func TestEarlyLevelFilterAllKeepsEnabled(t *testing.T) {
	dir, err := os.MkdirTemp("", "cygologger-early-all-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)

	if !InitDefaultWithOpts(dir, WithLogLevel(Core.LogFilterAll), WithConsole(false)) {
		t.Fatalf("InitDefaultWithOpts failed")
	}
	defer Close()

	Trace("t")
	Debug("d")
	Info("i")
	Warn("w")
	Error("e")
	Flush()
	time.Sleep(50 * time.Millisecond)

	for _, name := range []string{"Trace", "Debug", "Info", "Warn", "Error"} {
		matches, _ := filepath.Glob(filepath.Join(dir, name+"_*.log"))
		if len(matches) == 0 {
			t.Errorf("expected %s log to exist with LogFilterAll, but it was not created", name)
		}
	}
}

// BenchmarkEarlyFilteredTrace measures the cost of a Trace call that is suppressed
// by the active level filter (release-style config). It should be negligible
// (nanoseconds) because the early filter returns before runtime.Caller / Sprintf.
func BenchmarkEarlyFilteredTrace(b *testing.B) {
	dir, err := os.MkdirTemp("", "cygologger-bench-*")
	if err != nil {
		b.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)
	if !InitDefaultWithOpts(dir, WithLogLevel(Core.LogFilterErrors), WithConsole(false)) {
		b.Fatalf("InitDefaultWithOpts failed")
	}
	defer Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Trace("suppressed trace %d", i)
	}
}
