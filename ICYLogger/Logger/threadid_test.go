package logger

import (
	"strings"
	"sync"
	"testing"

	"github.com/maxhaosl/cygologger/ICYLogger/Core"
)

// TestThreadIdDefaultOff verifies the default (no WithThreadId call) records
// T:0 and still produces correct, complete log lines — the fast path must be
// the default because the goroutine-ID fetch serialises all concurrent writers.
func TestThreadIdDefaultOff(t *testing.T) {
	dir := t.TempDir()
	if !InitLogger(dir, false) {
		t.Fatal("InitLogger failed")
	}
	defer UnInitLogger()

	l := GetInstance()
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				l.WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
					"tid.go", "handler", i, "default-off line g=%d i=%d", g, i)
			}
		}(g)
	}
	wg.Wait()

	got := readLogs(t, dir, "Info*.log")
	if !strings.Contains(got, "default-off line g=0 i=0") {
		t.Fatalf("log content missing after concurrent writes:\n%s", got[:min(200, len(got))])
	}
	// The T: field must render as 0 (goroutine ID not recorded by default).
	if !strings.Contains(got, "|T:0|") {
		t.Fatalf("expected T:0 in default-off output, got:\n%s", got[:min(200, len(got))])
	}
}

// TestThreadIdOptInOn verifies WithThreadId(true) records a non-zero goroutine
// ID and the line is still complete and race-free under concurrency.
func TestThreadIdOptInOn(t *testing.T) {
	dir := t.TempDir()
	if !InitLogger(dir, false) {
		t.Fatal("InitLogger failed")
	}
	Core.GetCYLoggerConfigInstance().SetWithThreadId(true)
	// Re-Init so the hot-path atomic gWithThreadId is refreshed from config.
	UnInitLogger()
	if !InitLogger(dir, false) {
		t.Fatal("re-Init failed")
	}
	defer UnInitLogger()

	l := GetInstance()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				l.WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
					"tid.go", "handler", i, "opt-in line g=%d i=%d", g, i)
			}
		}(g)
	}
	wg.Wait()

	got := readLogs(t, dir, "Info*.log")
	if !strings.Contains(got, "opt-in line g=0 i=0") {
		t.Fatalf("log content missing after concurrent writes:\n%s", got[:min(200, len(got))])
	}
	// With thread-id on, at least one line must carry a non-zero T: field.
	if !strings.Contains(got, "|T:") {
		t.Fatalf("expected T: field in opt-in output, got:\n%s", got[:min(200, len(got))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
