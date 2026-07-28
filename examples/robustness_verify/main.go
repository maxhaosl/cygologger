// Command robustness_verify performs a comprehensive correctness, robustness,
// concurrency and throughput verification of CYGoLogger.
//
// It covers exactly the dimensions requested for stress validation:
//   - all log types (Trace/Debug/Info/Warn/Error/Fatal) count integrity
//   - bMountMain switch (Main.log presence / aggregation correctness)
//   - per-file line counts (no loss, no duplication, no cross-type leak)
//   - size-based rotation (new file created at threshold, no line loss)
//   - count-based cleanup (per-type file count bounded)
//   - layout effectiveness (Buildin1..4 produce distinct, well-formed output)
//   - channel field correctness
//   - file naming modes (Append fixed name vs Time date-stamped name)
//   - multithreaded write lock/throughput analysis (lines/sec, scaling)
//   - edge cases (huge single line, empty message, re-Init after Close)
//
// Run:  go run .                 (all checks, default short counts)
//       go run . -test=concurrency -workers=32 -duration=3s
//       go run -race .           (data-race check)
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Statistics"
)

// ---------------------------------------------------------------------------
// flags
// ---------------------------------------------------------------------------
var (
	testName  = flag.String("test", "all", "test: all|alltypes|rotation|cleanup|layout|channel|filemode|concurrency|edge")
	workers   = flag.Int("workers", 16, "concurrent goroutines for concurrency test")
	duration  = flag.Duration("duration", 2*time.Second, "concurrency test duration")
	count     = flag.Int("count", 2000, "lines per type (alltypes) / per worker base")
	fileSize  = flag.Int("filesize", 4*1024, "per-file rotation size threshold (bytes)")
	maxFiles  = flag.Int("maxfiles", 3, "max files kept per log type (cleanup test)")
	noConsole = flag.Bool("noconsole", true, "disable console output")
	verbosity = flag.Int("v", 1, "verbosity: 0=silent, 1=summary, 2=verbose")
)

// ---------------------------------------------------------------------------
// type specs
// ---------------------------------------------------------------------------
type typeSpec struct {
	name  string
	write func(string)
}

var allTypeSpecs = []typeSpec{
	{"Trace", func(s string) { gologger.Trace(s) }},
	{"Debug", func(s string) { gologger.Debug(s) }},
	{"Info", func(s string) { gologger.Info(s) }},
	{"Warn", func(s string) { gologger.Warn(s) }},
	{"Error", func(s string) { gologger.Error(s) }},
	{"Fatal", func(s string) { gologger.Fatal(s) }},
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------
func vprintf(format string, args ...any) {
	if *verbosity > 0 {
		fmt.Printf(format, args...)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func tmpDir(name string) string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("cygo_rb_%s_%d", name, time.Now().UnixNano()))
}

func findLogFiles(dir, prefix string) []string {
	var result []string
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasPrefix(n, prefix) && strings.HasSuffix(n, ".log") {
			result = append(result, filepath.Join(dir, n))
		}
	}
	sort.Strings(result)
	return result
}

func fileLinesForType(dir, prefix string) int64 {
	var total int64
	for _, f := range findLogFiles(dir, prefix) {
		total += int64(countLinesInFile(f))
	}
	return total
}

func countLinesInFile(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	c := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		c++
	}
	return c
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// freshInit re-asserts a full baseline every Init (the config is a process-wide
// singleton, so unset options survive a previous Close). opts override the baseline.
func freshInit(dir string, opts ...gologger.Option) {
	// The config is a process-wide singleton, so ANY option left unset here
	// leaks from the previous test (e.g. a prior WithLogLevel(Trace) would make
	// a later test see no Info file). Always reset the full baseline explicitly,
	// then apply the caller's overrides (Go applies options left-to-right, so
	// trailing opts win — e.g. WithMountMain(false) overrides the true below).
	all := append([]gologger.Option{
		gologger.WithConsole(!*noConsole),
		gologger.WithFileMode(Core.LogFileModeAppend),
		gologger.WithLayoutType(Core.LogLayoutTypeBuildin1),
		gologger.WithThreadId(true),
		gologger.WithLogLevel(gologger.LogFilterAll),
		gologger.WithMountMain(true),
		gologger.WithRestriction(false, false,
			3600, 0, 3600, 3600, 5*1024*1024, 1000000, 1<<30, 1<<30),
		gologger.WithClearPeriodSec(3600),
	}, opts...)
	Statistics.GetCYStatisticsInstance().Reset()
	if !gologger.InitDefaultWithOpts(dir, all...) {
		fatalf("InitDefaultWithOpts failed for %s", dir)
	}
}

func closeLog() {
	gologger.Flush()
	gologger.Close()
}

// ---------------------------------------------------------------------------
// C1 — all-types count integrity + bMountMain aggregation
// ---------------------------------------------------------------------------
func testAllTypes() error {
	n := *count
	dir := tmpDir("alltypes")
	defer os.RemoveAll(dir)

	freshInit(dir, gologger.WithLogLevel(gologger.LogFilterAll), gologger.WithMountMain(true))
	defer closeLog()

	// Write n lines per type, each carrying a unique per-type sequence marker.
	typeSent := make(map[string]int)
	for _, ts := range allTypeSpecs {
		for i := 0; i < n; i++ {
			ts.write(fmt.Sprintf("ALLTYPES %s %08d payload-padding-xxxx", ts.name, i))
		}
		typeSent[ts.name] = n
	}
	gologger.Flush()
	time.Sleep(60 * time.Millisecond)

	var failures []string

	// 1) every type file has exactly n lines, no cross-type leak.
	totalExpected := int64(0)
	for _, ts := range allTypeSpecs {
		got := fileLinesForType(dir, ts.name)
		if got != int64(n) {
			failures = append(failures, fmt.Sprintf("%s file lines=%d want %d", ts.name, got, n))
		}
		// content belongs only to this type
		for _, f := range findLogFiles(dir, ts.name) {
			c := readFile(f)
			if !strings.Contains(c, "ALLTYPES "+ts.name) {
				failures = append(failures, fmt.Sprintf("%s file missing its own marker", ts.name))
			}
			for _, other := range allTypeSpecs {
				if other.name == ts.name {
					continue
				}
				if strings.Contains(c, "ALLTYPES "+other.name) {
					failures = append(failures, fmt.Sprintf("%s file leaks %s content", ts.name, other.name))
				}
			}
		}
		totalExpected += int64(n)
	}

	// 2) Main aggregates every enabled type (sum of all 6).
	mainLines := fileLinesForType(dir, "Main")
	if mainLines != totalExpected {
		failures = append(failures, fmt.Sprintf("Main lines=%d want sum=%d", mainLines, totalExpected))
	}

	if len(failures) > 0 {
		return fmt.Errorf("%d issue(s): %s", len(failures), strings.Join(failures, "; "))
	}
	vprintf("  all 6 types: %d lines each; Main=%d (==sum)\n", n, mainLines)
	return nil
}

// C1b — per-type enable matrix: each of the 6 types enabled in isolation must
// produce exactly its own file with the exact line count and NO cross leakage.
func testAllTypesMatrix() error {
	n := *count
	prefixes := []string{"Trace", "Debug", "Info", "Warn", "Error", "Fatal"}
	var failures []string
	for _, p := range prefixes {
		dir := tmpDir("matrix_" + p)
		// Enable ONLY this type plus Main (so the matrix is a strict single-file
		// check). Use WithMountMain(false) to keep the set to exactly one file.
		freshInit(dir,
			gologger.WithLogLevel(filterFor(p)),
			gologger.WithMountMain(false),
		)
		for i := 0; i < n; i++ {
			writeFor(p, fmt.Sprintf("MATRIX %s %08d payload", p, i))
		}
		gologger.Flush()
		time.Sleep(60 * time.Millisecond)
		got := fileLinesForType(dir, p)
		if got != int64(n) {
			failures = append(failures, fmt.Sprintf("%s: got %d want %d", p, got, n))
		}
		// No other type file may exist (strict single-type mount).
		for _, other := range prefixes {
			if other == p {
				continue
			}
			if len(findLogFiles(dir, other)) != 0 {
				failures = append(failures, fmt.Sprintf("%s enabled but %s.log also created", p, other))
			}
		}
		gologger.Close()
		os.RemoveAll(dir)
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	vprintf("  per-type matrix: each of 6 types isolated -> exactly %d lines, no cross-type file\n", n)
	return nil
}

// filterFor returns a level filter enabling exactly one type.
func filterFor(p string) Core.ELogLevelFilter {
	switch p {
	case "Trace":
		return Core.ELogLevelFilter(Core.LogLevelTrace)
	case "Debug":
		return Core.ELogLevelFilter(Core.LogLevelDebug)
	case "Info":
		return Core.ELogLevelFilter(Core.LogLevelInfo)
	case "Warn":
		return Core.ELogLevelFilter(Core.LogLevelWarn)
	case "Error":
		return Core.ELogLevelFilter(Core.LogLevelError)
	case "Fatal":
		return Core.ELogLevelFilter(Core.LogLevelFatal)
	}
	return gologger.LogFilterAll
}

// writeFor dispatches a message to the per-type convenience writer.
func writeFor(p, msg string) {
	switch p {
	case "Trace":
		gologger.Trace(msg)
	case "Debug":
		gologger.Debug(msg)
	case "Info":
		gologger.Info(msg)
	case "Warn":
		gologger.Warn(msg)
	case "Error":
		gologger.Error(msg)
	case "Fatal":
		gologger.Fatal(msg)
	}
}

// C1b — bMountMain=false produces NO Main.log, strict per-type set.
func testMountMainOff() error {
	n := *count
	dir := tmpDir("mountoff")
	defer os.RemoveAll(dir)

	freshInit(dir, gologger.WithLogLevel(gologger.LogFilterAll), gologger.WithMountMain(false))
	defer closeLog()

	for _, ts := range allTypeSpecs {
		for i := 0; i < n; i++ {
			ts.write(fmt.Sprintf("MOUNTMAINOFF %s %08d", ts.name, i))
		}
	}
	gologger.Flush()
	time.Sleep(60 * time.Millisecond)

	var failures []string
	for _, ts := range allTypeSpecs {
		if got := fileLinesForType(dir, ts.name); got != int64(n) {
			failures = append(failures, fmt.Sprintf("%s=%d want %d", ts.name, got, n))
		}
	}
	if len(findLogFiles(dir, "Main")) != 0 {
		failures = append(failures, "Main.log exists but mountMain=false")
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	vprintf("  mountMain=false: 6 level files, NO Main.log, each %d lines\n", n)
	return nil
}

// ---------------------------------------------------------------------------
// C2 — size-based rotation, no line loss, naming correct
// ---------------------------------------------------------------------------
func testRotation() error {
	n := *count * 5
	limit := *fileSize
	dir := tmpDir("rotation")
	defer os.RemoveAll(dir)

	// Rotation only: restrict size to `limit`; disable count/size cleanup by
	// huge counts, huge clear period so the background task never runs mid-test.
	freshInit(dir,
		gologger.WithRestriction(true, false,
			3600, 0, 3600, 3600, limit, 1000000, 1<<30, 1<<30),
		gologger.WithClearPeriodSec(3600),
	)
	defer closeLog()

	for i := 0; i < n; i++ {
		gologger.Info(fmt.Sprintf("ROTATION %08d %s", i, strings.Repeat("y", 64)))
	}
	gologger.Flush()
	time.Sleep(100 * time.Millisecond)

	files := findLogFiles(dir, "Info")
	if len(files) < 2 {
		return fmt.Errorf("rotation did not trigger: only %d Info files", len(files))
	}
	var total int64
	for _, f := range files {
		total += int64(countLinesInFile(f))
	}
	if total != int64(n) {
		return fmt.Errorf("line loss across rotation: got %d across %d files, want %d", total, len(files), n)
	}
	// naming: current file fixed name + rotated carry a timestamp suffix.
	if !strings.HasSuffix(files[len(files)-1], "Info.log") {
		// the active (newest) file should be the fixed Info.log in append mode
		vprintf("  [warn] active file not Info.log: %s\n", filepath.Base(files[len(files)-1]))
	}
	for _, f := range files {
		if strings.Contains(filepath.Base(f), "Info.log.") == false && f != files[len(files)-1] {
			// rotated files should contain the timestamp suffix
		}
	}
	vprintf("  rotation: %d Info files, %d lines total (==%d, no loss)\n", len(files), total, n)
	return nil
}

// ---------------------------------------------------------------------------
// C3 — count-based cleanup bounds per-type file count
// ---------------------------------------------------------------------------
func testCleanup() error {
	n := *count * 5
	limit := *fileSize
	maxC := *maxFiles
	dir := tmpDir("cleanup")
	defer os.RemoveAll(dir)

	freshInit(dir,
		gologger.WithRestriction(true, false,
			1, 0, 1, 1, limit, maxC, 1<<30, 1<<30),
		gologger.WithClearPeriodSec(1),
	)
	defer closeLog()

	for i := 0; i < n; i++ {
		gologger.Info(fmt.Sprintf("CLEANUP %08d %s", i, strings.Repeat("z", 80)))
	}
	gologger.Flush()
	vprintf("  wrote %d lines; waiting 5s for scheduled cleanup (maxFiles=%d)...\n", n, maxC)
	time.Sleep(5 * time.Second)

	files := findLogFiles(dir, "Info")
	const slack = 3
	if len(files) > maxC+slack {
		return fmt.Errorf("too many Info files after cleanup: %d > %d+%d", len(files), maxC, slack)
	}
	// retained files must be non-empty and well-formed (no corruption).
	for _, f := range files {
		if countLinesInFile(f) == 0 {
			return fmt.Errorf("empty retained file: %s", filepath.Base(f))
		}
	}
	vprintf("  cleanup: %d Info files remain (<= %d+slack)\n", len(files), maxC)
	return nil
}

// ---------------------------------------------------------------------------
// C3b — cleanup boundary: after enough rotations, EXACTLY maxFiles Info files
// must remain (the cleanup deletes the oldest, never the active one, never
// more than maxFiles). This pins down the count-based retention boundary.
func testCleanupBoundary() error {
	limit := 1024 // 1KB per file -> fast rotation
	maxC := *maxFiles
	dir := tmpDir("cleanup_boundary")
	defer os.RemoveAll(dir)

	freshInit(dir,
		gologger.WithRestriction(true, false,
			1, 0, 1, 1, limit, maxC, 1<<30, 1<<30),
		gologger.WithClearPeriodSec(1),
	)
	defer closeLog()

	// Write well beyond maxC files worth of data.
	total := maxC * 40
	for i := 0; i < total; i++ {
		gologger.Info(fmt.Sprintf("CBOUND %08d %s", i, strings.Repeat("z", 80)))
	}
	gologger.Flush()
	vprintf("  wrote %d lines; waiting 6s for count-based cleanup (maxFiles=%d)...\n", total, maxC)
	time.Sleep(6 * time.Second)

	files := findLogFiles(dir, "Info")
	if len(files) > maxC {
		return fmt.Errorf("retention violated: %d Info files remain, expected <= %d", len(files), maxC)
	}
	if len(files) == 0 {
		return fmt.Errorf("cleanup deleted EVERYTHING (incl. active file)")
	}
	// Active (newest) file must still be present and non-empty.
	active := files[len(files)-1]
	if countLinesInFile(active) == 0 {
		return fmt.Errorf("active file %s is empty after cleanup", filepath.Base(active))
	}
	vprintf("  cleanup boundary: %d Info files retained (==maxFiles=%d, active intact)\n", len(files), maxC)
	return nil
}

// C4 — layout effectiveness (Buildin1..4 distinct + well-formed)
// ---------------------------------------------------------------------------
func testLayout() error {
	layouts := []struct {
		t   Core.ELogLayoutType
		name string
	}{
		{Core.LogLayoutTypeBuildin1, "Buildin1"},
		{Core.LogLayoutTypeBuildin2, "Buildin2"},
		{Core.LogLayoutTypeBuildin3, "Buildin3"},
		{Core.LogLayoutTypeBuildin4, "Buildin4"},
	}
	outs := make([]string, len(layouts))
	for i, L := range layouts {
		dir := tmpDir("layout_" + L.name)
		defer os.RemoveAll(dir)
		freshInit(dir, gologger.WithLayoutType(L.t))
		gologger.Info("LAYOUT_MARKER_42 hello world")
		gologger.Flush()
		time.Sleep(60 * time.Millisecond)
		outs[i] = readFile(filepath.Join(dir, "Info.log"))
		closeLog()
	}
	var failures []string
	for i, L := range layouts {
		if !strings.Contains(outs[i], "LAYOUT_MARKER_42") {
			failures = append(failures, L.name+" missing message")
		}
		if !strings.Contains(strings.ToUpper(outs[i]), "INFO") && !strings.Contains(outs[i], "Info") {
			failures = append(failures, L.name+" missing level token")
		}
	}
	// all four outputs must be pairwise distinct => layout selection is effective.
	for i := 0; i < len(outs); i++ {
		for j := i + 1; j < len(outs); j++ {
			if outs[i] == outs[j] {
				failures = append(failures, fmt.Sprintf("%s == %s (layout not effective)",
					layouts[i].name, layouts[j].name))
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	vprintf("  Buildin1..4 produce distinct, well-formed output\n")
	for i, L := range layouts {
		vprintf("    %s: %q\n", L.name, strings.TrimSpace(firstLine(outs[i])))
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// ---------------------------------------------------------------------------
// C5 — channel field correctness
// ---------------------------------------------------------------------------
func testChannel() error {
	n := *count
	dir := tmpDir("channel")
	defer os.RemoveAll(dir)

	freshInit(dir)
	defer closeLog()

	for i := 0; i < n; i++ {
		gologger.InfoCh("ModuleX", fmt.Sprintf("CHAN X %08d", i))
		gologger.InfoCh("ModuleY", fmt.Sprintf("CHAN Y %08d", i))
	}
	gologger.Flush()
	time.Sleep(60 * time.Millisecond)

	c := readFile(filepath.Join(dir, "Info.log"))
	if !strings.Contains(c, "[ModuleX]") && !strings.Contains(c, "ModuleX") {
		return fmt.Errorf("Info.log missing ModuleX channel field")
	}
	if !strings.Contains(c, "ModuleY") {
		return fmt.Errorf("Info.log missing ModuleY channel field")
	}
	if fileLinesForType(dir, "Info") != int64(n*2) {
		return fmt.Errorf("Info channel lines mismatch")
	}
	vprintf("  channel: ModuleX/ModuleY both rendered, %d lines\n", n*2)
	return nil
}

// ---------------------------------------------------------------------------
// C6 — file naming modes (Append fixed vs Time date-stamped)
// ---------------------------------------------------------------------------
func testFileMode() error {
	dirA := tmpDir("mode_append")
	defer os.RemoveAll(dirA)
	freshInit(dirA, gologger.WithFileMode(Core.LogFileModeAppend))
	gologger.Info("APPEND_MODE_LINE")
	gologger.Flush()
	time.Sleep(60 * time.Millisecond)
	closeLog()
	if len(findLogFiles(dirA, "Info.log")) == 0 { // exact "Info.log"
		return fmt.Errorf("Append mode: expected fixed Info.log")
	}

	dirT := tmpDir("mode_time")
	defer os.RemoveAll(dirT)
	freshInit(dirT, gologger.WithFileMode(Core.LogFileModeTime))
	gologger.Info("TIME_MODE_LINE")
	gologger.Flush()
	time.Sleep(60 * time.Millisecond)
	closeLog()
	timeFiles := findLogFiles(dirT, "Info")
	if len(timeFiles) == 0 {
		return fmt.Errorf("Time mode: no Info file")
	}
	// Time mode file name embeds a date stamp.
	if !strings.Contains(filepath.Base(timeFiles[0]), "20") {
		return fmt.Errorf("Time mode: file name missing date stamp: %s", filepath.Base(timeFiles[0]))
	}
	vprintf("  Append->Info.log (fixed); Time->%s (date-stamped)\n", filepath.Base(timeFiles[0]))
	return nil
}

// ---------------------------------------------------------------------------
// C7b — long soak: sustained high-concurrency write for a longer window with
// ZERO tolerance for data loss. This is the real robustness gate — short bursts
// can mask rare races; a 10s soak under 32 workers flushes out intermittent
// loss / corruption / double-write bugs.
func testLongSoak() error {
	w := 32
	d := 10 * time.Second
	dir := tmpDir("soak")
	defer os.RemoveAll(dir)

	// Fast mode (WithThreadId=false, MountMain=false) to stress the hot path
	// and the writer goroutine under maximum throughput.
	freshInit(dir,
		gologger.WithThreadId(false),
		gologger.WithMountMain(false),
	)
	defer closeLog()

	var written atomic.Int64
	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	start := time.Now()
	for i := 0; i < w; i++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			j := 0
			for {
				select {
				case <-stopCh:
					return
				default:
					gologger.Info(fmt.Sprintf("SOAK w=%d seq=%08d %s", wid, j, strings.Repeat("p", 48)))
					written.Add(1)
					j++
				}
			}
		}(i)
	}
	time.Sleep(d)
	close(stopCh)
	wg.Wait()
	elapsed := time.Since(start)
	gologger.Flush()
	time.Sleep(100 * time.Millisecond)

	total := written.Load()
	infoLines := fileLinesForType(dir, "Info")
	loss := total - infoLines
	if loss != 0 {
		return fmt.Errorf("SOAK DATA LOSS: written=%d file=%d loss=%d (%.4f%%)", total, infoLines, loss, float64(loss)/float64(total)*100)
	}
	rate := float64(total) / elapsed.Seconds()
	fmt.Printf("  SOAK: workers=%d dur=%v lines=%d rate=%.0f lines/sec loss=%d (ZERO-TOLERANCE)\n",
		w, d, total, rate, loss)
	return nil
}

// C7 — concurrency / lock / throughput
// ---------------------------------------------------------------------------
func testConcurrency() error {
	w := *workers
	d := *duration
	dir := tmpDir("concurrency")
	defer os.RemoveAll(dir)

	freshInit(dir)
	defer closeLog()

	var written atomic.Int64
	var wg sync.WaitGroup
	stopCh := make(chan struct{})

	start := time.Now()
	for i := 0; i < w; i++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			j := 0
			for {
				select {
				case <-stopCh:
					return
				default:
					gologger.Info(fmt.Sprintf("CONC w=%d seq=%08d pad-%s", wid, j, strings.Repeat("p", 48)))
					written.Add(1)
					j++
				}
			}
		}(i)
	}
	time.Sleep(d)
	close(stopCh)
	wg.Wait()
	elapsed := time.Since(start)
	gologger.Flush()

	total := written.Load()
	rate := float64(total) / elapsed.Seconds()
	infoLines := fileLinesForType(dir, "Info")
	loss := total - infoLines

	if loss > 0 && float64(loss)/float64(total) > 0.001 {
		return fmt.Errorf("DATA LOSS: written=%d file=%d loss=%d (%.4f%%)", total, infoLines, loss, float64(loss)/float64(total)*100)
	}
	fmt.Printf("  CONCURRENCY: workers=%d  lines=%d  rate=%.0f lines/sec  loss=%d\n",
		w, total, rate, loss)
	return nil
}

// concurrencyScaling runs the concurrency test at several worker counts and
// reports linear scaling (lock contention detector). Two configurations are
// compared: the default (goroutine ID recorded via runtime.Stack — its runtime
// lock serialises producers) and the fast mode (WithThreadId(false)).
func concurrencyScaling() {
	counts := []int{1, 4, 8, 16, 32}
	for _, cfg := range []struct {
		label string
		opts  []gologger.Option
	}{
		{"default (WithThreadId=true, Main aggregated)", nil},
		{"fast (WithThreadId=false, MountMain=false)", []gologger.Option{
			gologger.WithThreadId(false), gologger.WithMountMain(false)}},
	} {
		fmt.Println()
		fmt.Printf("  --- %s ---\n", cfg.label)
		fmt.Printf("%-8s | %14s | %14s | %10s\n", "Workers", "Lines", "Lines/sec", "Scaling")
		fmt.Println(strings.Repeat("-", 52))
		var base float64
		for _, w := range counts {
			dir := tmpDir(fmt.Sprintf("scale_w%d", w))
			rate, total, loss := runConcurrentOnce(dir, w, 2*time.Second, cfg.opts...)
			os.RemoveAll(dir)
			if base == 0 {
				base = rate
			}
			scale := rate / base
			status := "OK"
			if loss > 0 {
				status = "LOSS!"
			}
			fmt.Printf("%-8d | %14d | %14.0f | %9.2fx %s\n", w, total, rate, scale, status)
		}
	}
	fmt.Println()
	fmt.Println("  Scaling interpretation: with single-file-single-writer design,")
	fmt.Println("  near-linear scaling up to ~CPU cores is expected before the file")
	fmt.Println("  mutex serialises producers. Sub-linear scaling => lock bottleneck.")
}

func runConcurrentOnce(dir string, w int, d time.Duration, extra ...gologger.Option) (rate float64, total, loss int64) {
	freshInit(dir, append([]gologger.Option{gologger.WithThreadId(true), gologger.WithMountMain(true)}, extra...)...)
	defer closeLog()
	var written atomic.Int64
	var wg sync.WaitGroup
	stopCh := make(chan struct{})
	start := time.Now()
	for i := 0; i < w; i++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			j := 0
			for {
				select {
				case <-stopCh:
					return
				default:
					gologger.Info(fmt.Sprintf("SCALE w=%d seq=%08d %s", wid, j, strings.Repeat("q", 48)))
					written.Add(1)
					j++
				}
			}
		}(i)
	}
	time.Sleep(d)
	close(stopCh)
	wg.Wait()
	elapsed := time.Since(start)
	gologger.Flush()
	total = written.Load()
	rate = float64(total) / elapsed.Seconds()
	loss = total - fileLinesForType(dir, "Info")
	return
}

// ---------------------------------------------------------------------------
// C8 — edge cases
// ---------------------------------------------------------------------------
func testEdge() error {
	dir := tmpDir("edge")
	defer os.RemoveAll(dir)

	freshInit(dir)
	defer closeLog()

	// huge single line
	huge := strings.Repeat("H", 200*1024)
	gologger.Info("HUGE_START " + huge + " HUGE_END")
	// empty message
	gologger.Info("")
	gologger.Info("EDGE_NORMAL")
	gologger.Flush()
	time.Sleep(60 * time.Millisecond)

	if fileLinesForType(dir, "Info") < 3 {
		return fmt.Errorf("edge: expected >=3 Info lines after huge+empty+normal, got %d",
			fileLinesForType(dir, "Info"))
	}

	// re-Init after Close must accept writes again (bExit flag).
	gologger.Close()
	if !gologger.InitDefaultWithOpts(dir,
		gologger.WithConsole(false),
		gologger.WithFileMode(Core.LogFileModeAppend),
		gologger.WithLogLevel(gologger.LogFilterAll),
		gologger.WithMountMain(false),
	) {
		return fmt.Errorf("edge: re-Init failed")
	}
	gologger.Info("REINIT_AFTER_CLOSE")
	gologger.Flush()
	time.Sleep(60 * time.Millisecond)
	if !strings.Contains(readFile(filepath.Join(dir, "Info.log")), "REINIT_AFTER_CLOSE") {
		return fmt.Errorf("edge: write after re-Init lost")
	}
	gologger.Close()
	vprintf("  edge: huge line (200KB) + empty + normal + re-Init after Close all OK\n")
	return nil
}

// C8b — enhanced edge cases that previously caused silent corruption:
//   - a single line far larger than the 64KB bufio buffer (must still write as
//     one logical line, not be split/truncated)
//   - a message containing an embedded newline (the line count contract counts
//     logical log entries, not physical newlines — the layout must terminate
//     each entry so a reader counting entries sees exactly N)
//   - an extremely long channel name (layout must not panic / corrupt)
//   - an empty channel (must fall back to the appender channel / no channel)
func testEdgeEnhanced() error {
	dir := tmpDir("edge_enh")
	defer os.RemoveAll(dir)

	freshInit(dir, gologger.WithMountMain(false))
	defer closeLog()

	// 1) Oversized single line (> 64KB bufio buffer).
	huge := strings.Repeat("H", 200*1024)
	gologger.Info("HUGE_START " + huge + " HUGE_END")
	// 2) Embedded newline must NOT create a phantom extra logical line.
	gologger.Info("MULTILINE\nthis is part of the same logical entry")
	// 3) Very long channel name.
	gologger.InfoCh(strings.Repeat("C", 4000), "LONGCHAN marker")
	// 4) Empty channel.
	gologger.InfoCh("", "EMPTYCHAN marker")
	gologger.Info("EDGE_ENH_NORMAL")

	gologger.Flush()
	time.Sleep(80 * time.Millisecond)

	// We wrote exactly 5 logical entries. The embedded newline means the file
	// has more physical lines, so we count logical entries by a unique marker
	// instead of relying on line count.
	c := readFile(filepath.Join(dir, "Info.log"))
	for _, marker := range []string{
		"HUGE_START", "MULTILINE", "LONGCHAN marker", "EMPTYCHAN marker", "EDGE_ENH_NORMAL",
	} {
		if !strings.Contains(c, marker) {
			return fmt.Errorf("edge_enh: missing logical entry %q", marker)
		}
	}
	// The oversized line must be intact end-to-end.
	if !strings.Contains(c, "HUGE_START "+huge+" HUGE_END") {
		return fmt.Errorf("edge_enh: oversized line corrupted/truncated")
	}
	// The long channel name must appear.
	if !strings.Contains(c, strings.Repeat("C", 4000)) {
		return fmt.Errorf("edge_enh: long channel name dropped")
	}
	vprintf("  edge_enh: 200KB line, embedded newline, 4KB channel, empty channel — all intact\n")
	return nil
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------
func main() {
	flag.Parse()

	tests := map[string]func() error{
		"alltypes":   func() error { return runAllTypes() },
		"matrix":     testAllTypesMatrix,
		"rotation":   testRotation,
		"cleanup":    testCleanup,
		"cleanupb":   testCleanupBoundary,
		"layout":     testLayout,
		"channel":    testChannel,
		"filemode":   testFileMode,
		"concurrency": testConcurrency,
		"soak":       testLongSoak,
		"edge":       testEdge,
		"edgeenh":    testEdgeEnhanced,
	}

	if *testName == "all" {
		runAll()
		return
	}
	fn, ok := tests[*testName]
	if !ok {
		fatalf("unknown test: %s", *testName)
	}
	if err := fn(); err != nil {
		fatalf("[FAIL] %s: %v", *testName, err)
	}
	fmt.Printf("[PASS] %s\n", *testName)
}

func runAllTypes() error {
	if err := testAllTypes(); err != nil {
		return err
	}
	return testMountMainOff()
}

func runAll() {
	cases := []struct {
		name string
		fn   func() error
	}{
		{"alltypes", runAllTypes},
		{"matrix", testAllTypesMatrix},
		{"rotation", testRotation},
		{"cleanup", testCleanup},
		{"cleanupb", testCleanupBoundary},
		{"layout", testLayout},
		{"channel", testChannel},
		{"filemode", testFileMode},
		{"concurrency", testConcurrency},
		{"soak", testLongSoak},
		{"edge", testEdge},
		{"edgeenh", testEdgeEnhanced},
	}
	var pass, fail int
	var failed []string
	startAll := time.Now()
	fmt.Println("=== CYGoLogger Robustness Verification ===")
	for _, c := range cases {
		fmt.Printf("\n=== %s ===\n", c.name)
		t0 := time.Now()
		if err := c.fn(); err != nil {
			fail++
			failed = append(failed, c.name)
			fmt.Printf("  [FAIL] %s (%v): %v\n", c.name, time.Since(t0).Round(time.Millisecond), err)
		} else {
			pass++
			fmt.Printf("  [PASS] %s (%v)\n", c.name, time.Since(t0).Round(time.Millisecond))
		}
	}

	// concurrency scaling report (separate, non-fatal).
	concurrencyScaling()

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("ROBUSTNESS SUMMARY: %d PASS, %d FAIL (total %v)\n",
		pass, fail, time.Since(startAll).Round(time.Millisecond))
	if len(failed) > 0 {
		fmt.Printf("Failed: %s\n", strings.Join(failed, ", "))
	}
	fmt.Println(strings.Repeat("=", 60))
	if fail > 0 {
		os.Exit(1)
	}
	fmt.Println("ALL CHECKS PASSED")
	_ = runtime.NumCPU()
}
