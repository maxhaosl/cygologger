package logger

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/maxhaosl/cygologger/ICYLogger/Core"
)

// TestConcurrentMixedWrites hammers the logger from many goroutines with mixed
// log types, write variants, and interleaved Flush / SetLogLevel calls. Run
// with -race to validate the locking design; the assertion is that everything
// completes without panic/deadlock and the files contain data.
func TestConcurrentMixedWrites(t *testing.T) {
	dir := t.TempDir()
	initLoggerAt(t, dir)

	goroutines := 16
	perG := 300
	if testing.Short() {
		goroutines = 8
		perG = 50
	}

	types := []Core.ELogType{
		Core.LogTypeTrace, Core.LogTypeDebug, Core.LogTypeInfo,
		Core.LogTypeWarn, Core.LogTypeError, Core.LogTypeFatal, Core.LogTypeMain,
	}
	levels := []Core.ELogLevel{
		Core.LogLevelTrace, Core.LogLevelDebug, Core.LogLevelInfo,
		Core.LogLevelWarn, Core.LogLevelError, Core.LogLevelFatal, Core.LogLevelInfo,
	}

	l := GetInstance()
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				idx := (g + i) % len(types)
				switch i % 5 {
				case 0:
					l.WriteLogFmt(int(levels[idx]), types[idx], -1,
						"stab.go", "stabFunc", i, "g%d mixed line %d", g, i)
				case 1:
					l.WriteLogFmtCh(int(levels[idx]), types[idx], -1,
						fmt.Sprintf("G%d", g), "stab.go", "stabFunc", i, "channel line %d", i)
				case 2:
					l.WriteLogFmtDirect(types[idx], -1,
						"stab.go", "stabFunc", i, "direct line %d", i)
				case 3:
					l.WriteEscapeLogFmt(int(levels[idx]), types[idx], -1,
						"stab.go", "stabFunc", i, "escape [%d] line", i)
				default:
					l.WriteHexLog(int(levels[idx]), types[idx], -1,
						"stab.go", "stabFunc", i, []byte{byte(g), byte(i)})
				}
				if i%97 == 0 {
					l.Flush(Core.LogTypeMax)
				}
			}
		}(g)
	}
	// Concurrent runtime reconfiguration while writers are active.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			GetLoggerInstance().SetLogLevel(Core.LogFilterAll)
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()

	got := readLogs(t, dir, "Main*.log")
	if !strings.Contains(got, "mixed line") {
		t.Fatalf("Main log contains no data after concurrent writes")
	}
}

// TestRepeatedInitClose verifies the logger survives multiple Init/Close cycles
// in one process: every cycle must actually write (regression for the bExit
// flag never being reset) and must not leak appender goroutines.
func TestRepeatedInitClose(t *testing.T) {
	cycles := 5
	if testing.Short() {
		cycles = 2
	}

	// Warm up singletons so the baseline goroutine count is representative.
	warm := t.TempDir()
	if !InitLogger(warm, false) {
		t.Fatalf("warm-up InitLogger failed")
	}
	UnInitLogger()
	time.Sleep(100 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	for c := 0; c < cycles; c++ {
		dir := t.TempDir()
		if !InitLogger(dir, false) {
			t.Fatalf("cycle %d: InitLogger failed", c)
		}
		GetInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
			"stab.go", "stabFunc", c, "cycle-%d-marker", c)
		content := readLogs(t, dir, "Info*.log")
		if !strings.Contains(content, fmt.Sprintf("cycle-%d-marker", c)) {
			t.Fatalf("cycle %d: write did not land after re-Init (bExit regression?)", c)
		}
		UnInitLogger()
	}

	// Allow appender goroutines to drain, then check for monotonic leakage.
	var after int
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		after = runtime.NumGoroutine()
		if after <= baseline+4 {
			break
		}
	}
	if after > baseline+8 {
		t.Errorf("goroutine leak suspected across Init/Close cycles: baseline=%d after=%d", baseline, after)
	}
}

// TestCloseIsIdempotent verifies repeated UnInit / Flush after close are safe.
func TestCloseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if !InitLogger(dir, false) {
		t.Fatalf("InitLogger failed")
	}
	GetInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
		"stab.go", "stabFunc", 1, "idempotent close line")

	UnInitLogger()
	UnInitLogger() // second close must be a no-op
	FlushLogger()  // flush after close must not panic
	GetInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
		"stab.go", "stabFunc", 2, "dropped after close") // silently dropped
}

// TestSustainedLoad is a longer soak test, skipped with -short: a burst of
// sustained writes with periodic flushes must keep the process stable.
func TestSustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping soak test in -short mode")
	}
	dir := t.TempDir()
	initLoggerAt(t, dir)

	l := GetInstance()
	deadline := time.Now().Add(2 * time.Second)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			i := 0
			for time.Now().Before(deadline) {
				l.WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
					"soak.go", "soakFunc", i, "soak g%d line %d", g, i)
				i++
				if i%1000 == 0 {
					l.Flush(Core.LogTypeInfo)
				}
			}
		}(g)
	}
	wg.Wait()

	var stats Core.STStatistics
	if !GetInstance().GetStats(&stats) {
		t.Fatalf("GetStats failed after soak")
	}
	if stats.NTotalLine == 0 {
		t.Errorf("soak produced no recorded lines")
	}
}
