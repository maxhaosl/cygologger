package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/maxhaosl/cygologger/ICYLogger/Core"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// initLoggerAt (re)initializes the logger singleton into dir with the console
// disabled and registers a cleanup that shuts it down again, so each test gets
// an isolated on-disk state while sharing the process-wide singletons.
func initLoggerAt(t testing.TB, dir string) {
	t.Helper()
	if !InitLogger(dir, false) {
		t.Fatalf("InitLogger(%q) failed", dir)
	}
	t.Cleanup(func() {
		// Restore the default level filter mutated by some tests.
		GetLoggerInstance().SetLogLevel(Core.DefaultLogLevelFilter)
		UnInitLogger()
	})
}

// readLogs concatenates the content of every file matching pattern under dir,
// flushing first so async appenders drain their queues.
func readLogs(t testing.TB, dir, pattern string) string {
	t.Helper()
	FlushLogger()
	// The main/file appenders write synchronously, but give the console/buffer
	// swap loops a beat to settle when running under -race.
	time.Sleep(50 * time.Millisecond)
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	var sb strings.Builder
	for _, m := range matches {
		data, err := os.ReadFile(m)
		if err != nil {
			t.Fatalf("read %s: %v", m, err)
		}
		sb.Write(data)
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Pure unit tests (no logger Init required)
// ---------------------------------------------------------------------------

// TestLevelForType verifies the log-type -> level mapping table.
func TestLevelForType(t *testing.T) {
	c := GetCYLoggerControlInstance()
	cases := []struct {
		in   Core.ELogType
		want Core.ELogLevel
	}{
		{Core.LogTypeConsole, Core.LogLevelConsole},
		{Core.LogTypeTrace, Core.LogLevelTrace},
		{Core.LogTypeDebug, Core.LogLevelDebug},
		{Core.LogTypeInfo, Core.LogLevelInfo},
		{Core.LogTypeWarn, Core.LogLevelWarn},
		{Core.LogTypeError, Core.LogLevelError},
		{Core.LogTypeFatal, Core.LogLevelFatal},
		{Core.LogTypeRemote, Core.LogLevelRemote},
		{Core.LogTypeSys, Core.LogLevelSys},
		{Core.LogTypeMain, Core.LogLevelConsole}, // default branch
	}
	for _, tc := range cases {
		if got := c.levelForType(tc.in); got != tc.want {
			t.Errorf("levelForType(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestPassesFilter verifies the bitmask level filter logic.
func TestPassesFilter(t *testing.T) {
	c := GetCYLoggerControlInstance()
	orig := c.GetLogLevel()
	defer c.SetLogLevel(orig)

	c.SetLogLevel(Core.LogFilterErrors)
	if !c.passesFilter(Core.LogLevelError) || !c.passesFilter(Core.LogLevelFatal) {
		t.Errorf("errors filter must pass Error/Fatal")
	}
	if c.passesFilter(Core.LogLevelInfo) || c.passesFilter(Core.LogLevelDebug) {
		t.Errorf("errors filter must block Info/Debug")
	}

	c.SetLogLevel(Core.LogFilterNone)
	if c.passesFilter(Core.LogLevelError) {
		t.Errorf("none filter must block everything")
	}

	c.SetLogLevel(Core.LogFilterAll)
	for _, lv := range []Core.ELogLevel{
		Core.LogLevelTrace, Core.LogLevelDebug, Core.LogLevelInfo,
		Core.LogLevelWarn, Core.LogLevelError, Core.LogLevelFatal,
	} {
		if !c.passesFilter(lv) {
			t.Errorf("all filter must pass %v", lv)
		}
	}
}

// TestFormatHex verifies the hex+ASCII dump layout.
func TestFormatHex(t *testing.T) {
	got := formatHex([]byte("ABC\x00"))
	if !strings.HasPrefix(got, "0000: ") {
		t.Errorf("hex dump must start with offset prefix, got %q", got)
	}
	if !strings.Contains(got, "41 42 43 00") {
		t.Errorf("hex dump missing byte values, got %q", got)
	}
	if !strings.Contains(got, "|ABC.|") {
		t.Errorf("hex dump missing ASCII column (non-printable as dot), got %q", got)
	}

	// 17 bytes must span two lines with the second offset at 0x0010.
	got2 := formatHex(make([]byte, 17))
	if !strings.Contains(got2, "0010: ") {
		t.Errorf("multi-line hex dump missing second offset, got %q", got2)
	}
	if formatHex(nil) != "" {
		t.Errorf("empty input must produce empty dump")
	}
}

// TestCallerInfo verifies runtime caller capture returns this file and a
// plausible function name/line.
func TestCallerInfo(t *testing.T) {
	file, fn, line := callerInfo(1) // 1 = this frame
	if file != "logger_test.go" {
		t.Errorf("file = %q, want logger_test.go", file)
	}
	if !strings.Contains(fn, "TestCallerInfo") {
		t.Errorf("func = %q, want to contain TestCallerInfo", fn)
	}
	if line <= 0 {
		t.Errorf("line = %d, want > 0", line)
	}
}

// ---------------------------------------------------------------------------
// Integration tests (full Init against a temp dir, console off)
// ---------------------------------------------------------------------------

// TestWriteLogFmtEndToEnd verifies a formatted write lands in the Info log with
// caller info and message body, and is mirrored into the Main log.
func TestWriteLogFmtEndToEnd(t *testing.T) {
	dir := t.TempDir()
	initLoggerAt(t, dir)

	GetInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
		"unit.go", "unitFunc", 42, "hello %s #%d", "world", 7)

	info := readLogs(t, dir, "Info*.log")
	if !strings.Contains(info, "hello world #7") {
		t.Fatalf("Info log missing message, got:\n%s", info)
	}
	main_ := readLogs(t, dir, "Main*.log")
	if !strings.Contains(main_, "hello world #7") {
		t.Fatalf("Main log must aggregate every message, got:\n%s", main_)
	}
}

// TestLevelFilterBlocksAndDirectBypasses verifies SetLogLevel filtering and the
// Direct write path that bypasses it.
func TestLevelFilterBlocksAndDirectBypasses(t *testing.T) {
	dir := t.TempDir()
	initLoggerAt(t, dir)

	GetLoggerInstance().SetLogLevel(Core.LogFilterErrors)

	GetInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
		"unit.go", "unitFunc", 1, "filtered-info-line")
	GetInstance().WriteLogFmtDirect(Core.LogTypeInfo, -1,
		"unit.go", "unitFunc", 2, "direct-info-line")
	GetInstance().WriteLogFmt(int(Core.LogLevelError), Core.LogTypeError, -1,
		"unit.go", "unitFunc", 3, "error-line")

	info := readLogs(t, dir, "Info*.log")
	if strings.Contains(info, "filtered-info-line") {
		t.Errorf("info line must be blocked by LogFilterErrors")
	}
	if !strings.Contains(info, "direct-info-line") {
		t.Errorf("direct write must bypass the level filter")
	}
	errLog := readLogs(t, dir, "Error*.log")
	if !strings.Contains(errLog, "error-line") {
		t.Errorf("error line must pass LogFilterErrors")
	}
}

// TestChannelEscapeHexVariants smoke-tests the Ch / Escape / Hex write paths.
func TestChannelEscapeHexVariants(t *testing.T) {
	dir := t.TempDir()
	initLoggerAt(t, dir)

	GetInstance().WriteLogFmtCh(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
		"UTChan", "unit.go", "unitFunc", 1, "channel-line")
	GetInstance().WriteEscapeLogFmt(int(Core.LogLevelWarn), Core.LogTypeWarn, -1,
		"unit.go", "unitFunc", 2, "escape [x] line")
	GetInstance().WriteHexLog(int(Core.LogLevelDebug), Core.LogTypeDebug, -1,
		"unit.go", "unitFunc", 3, []byte{0xDE, 0xAD})
	GetInstance().WriteLogFmtDirectCh(Core.LogTypeInfo, -1,
		"UTChan2", "unit.go", "unitFunc", 4, "direct-channel-line")

	info := readLogs(t, dir, "Info*.log")
	if !strings.Contains(info, "Channel:UTChan") || !strings.Contains(info, "channel-line") {
		t.Errorf("channel write missing, got:\n%s", info)
	}
	if !strings.Contains(info, "Channel:UTChan2") || !strings.Contains(info, "direct-channel-line") {
		t.Errorf("direct channel write missing, got:\n%s", info)
	}
	warn := readLogs(t, dir, "Warn*.log")
	if !strings.Contains(warn, "escape") {
		t.Errorf("escape write missing, got:\n%s", warn)
	}
	debug := readLogs(t, dir, "Debug*.log")
	if !strings.Contains(debug, "de ad") {
		t.Errorf("hex write missing, got:\n%s", debug)
	}
}

// TestConvenienceMacrosCaptureCaller verifies LOG_* helpers auto-capture this
// test file as the caller.
func TestConvenienceMacrosCaptureCaller(t *testing.T) {
	dir := t.TempDir()
	initLoggerAt(t, dir)

	LOG_INFO("macro info %d", 1)
	LOG_WARN("macro warn")
	LOG_DIRECT_ERROR("macro direct error")
	LOG_INFO_CH("MacroChan", "macro channel info")

	info := readLogs(t, dir, "Info*.log")
	if !strings.Contains(info, "macro info 1") {
		t.Errorf("LOG_INFO missing, got:\n%s", info)
	}
	if !strings.Contains(info, "logger_test.go") {
		t.Errorf("LOG_INFO must capture caller file, got:\n%s", info)
	}
	if !strings.Contains(info, "Channel:MacroChan") {
		t.Errorf("LOG_INFO_CH must render its channel, got:\n%s", info)
	}
	errLog := readLogs(t, dir, "Error*.log")
	if !strings.Contains(errLog, "macro direct error") {
		t.Errorf("LOG_DIRECT_ERROR missing, got:\n%s", errLog)
	}
}

// TestWriteBeforeInitIsSafe verifies writes are silently dropped (no panic)
// when the logger is not initialized.
func TestWriteBeforeInitIsSafe(t *testing.T) {
	l := GetLoggerInstance()
	if l.IsInit() {
		UnInitLogger()
	}
	// Must not panic and must not deadlock.
	l.WriteLog(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, "dropped")
	l.WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, "f.go", "fn", 1, "dropped %d", 1)
	l.WriteHexLog(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, "f.go", "fn", 1, []byte{1})
	l.Flush(Core.LogTypeMax)
}

// TestGetStats verifies statistics are reachable and grow with writes.
func TestGetStats(t *testing.T) {
	dir := t.TempDir()
	initLoggerAt(t, dir)

	var before Core.STStatistics
	if !GetInstance().GetStats(&before) {
		t.Fatalf("GetStats failed")
	}
	for i := 0; i < 10; i++ {
		GetInstance().WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
			"unit.go", "unitFunc", i, "stat line %d", i)
	}
	FlushLogger()
	var after Core.STStatistics
	if !GetInstance().GetStats(&after) {
		t.Fatalf("GetStats failed")
	}
	if after.NTotalLine < before.NTotalLine+10 {
		t.Errorf("total line counter did not grow by 10: before=%d after=%d",
			before.NTotalLine, after.NTotalLine)
	}
}
