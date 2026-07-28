// Correctness-oriented stress tests: all log levels, layout formats,
// channel field rendering, and file-naming modes.
//
// These complement the throughput/rotation tests in main.go: they verify not
// just "how many lines" but "are the lines exactly right".
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
)

// ---------------------------------------------------------------------------
// 9. alllevels — every log level: correct file, correct count, correct format
// ---------------------------------------------------------------------------

func testAllLevels() error {
	perLevel := *count / 6
	if perLevel < 1000 {
		perLevel = 1000
	}
	dir := tmpDir("alllevels")
	defer cleanupDir(dir)

	type level struct {
		name   string // file prefix
		code   string // layout-1 type code
		marker string // message marker
		fn     func(format string, args ...any)
	}
	levels := []level{
		{"Trace", "T", "lvl=TRACE", gologger.Trace},
		{"Debug", "D", "lvl=DEBUG", gologger.Debug},
		{"Info", "I", "lvl=INFO", gologger.Info},
		{"Warn", "W", "lvl=WARN", gologger.Warn},
		{"Error", "E", "lvl=ERROR", gologger.Error},
		{"Fatal", "F", "lvl=FATAL", gologger.Fatal},
	}

	vprintf("  %d lines per level x %d levels (concurrent)\n", perLevel, len(levels))

	initLog(dir)
	defer closeLog()

	var wg sync.WaitGroup
	for _, lv := range levels {
		wg.Add(1)
		go func(lv level) {
			defer wg.Done()
			for i := 0; i < perLevel; i++ {
				lv.fn("%s seq=%08d all-level correctness probe", lv.marker, i)
			}
		}(lv)
	}
	wg.Wait()
	gologger.Flush()

	var stats Core.STStatistics
	if !gologger.GetInstance().GetStats(&stats) {
		return fmt.Errorf("GetStats failed")
	}
	statByLevel := map[string]uint64{
		"Trace": stats.NTraceLine, "Debug": stats.NDebugLine,
		"Info": stats.NInfoLine, "Warn": stats.NWarnLine,
		"Error": stats.NErrorLine, "Fatal": stats.NFatalLine,
	}

	total := int64(perLevel * len(levels))
	for _, lv := range levels {
		// 1) statistics counter
		if got := statByLevel[lv.name]; got != uint64(perLevel) {
			return fmt.Errorf("%s: Stats counter=%d, want %d", lv.name, got, perLevel)
		}
		// 2) file line count
		lines := fileLinesForType(dir, lv.name)
		if lines != int64(perLevel) {
			return fmt.Errorf("%s: file lines=%d, want %d", lv.name, lines, perLevel)
		}
		// 3) every line has the right type code and marker; no cross-type mixing
		codeTag := "|" + lv.code + "|P:"
		for _, f := range findLogFiles(dir, lv.name) {
			bad, badLine := scanEveryLine(f, func(line string) bool {
				return strings.Contains(line, codeTag) && strings.Contains(line, lv.marker)
			})
			if bad {
				return fmt.Errorf("%s: malformed/mixed line in %s: %q",
					lv.name, filepath.Base(f), badLine)
			}
		}
		vprintf("  [OK] %-5s count=%d, format+marker verified\n", lv.name, perLevel)
	}

	// 4) Main aggregates every level exactly once
	mainLines := fileLinesForType(dir, "Main")
	if mainLines != total {
		return fmt.Errorf("Main aggregate: got %d lines, want %d", mainLines, total)
	}
	vprintf("  [OK] Main aggregate=%d (= 6 x %d)\n", mainLines, perLevel)

	// 5) no line ever carries T:0 (goroutine id must be real, regression for GetGID)
	for _, f := range findLogFiles(dir, "Info") {
		bad, badLine := scanEveryLine(f, func(line string) bool {
			return !strings.Contains(line, "|T:0|")
		})
		if bad {
			return fmt.Errorf("goroutine id is 0 (GetGID regression): %q", badLine)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 10. layout — built-in layouts 1-4 produce exactly the documented format
// ---------------------------------------------------------------------------

func testLayout() error {
	const msg = "layout probe alpha 12345"
	const chMsg = "layout channel probe"
	const chName = "ChanX"

	ts := `\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3}`
	loc := `[^|\]]+\(\d+\)` // func(line)

	cases := []struct {
		layout  Core.ELogLayoutType
		name    string
		plainRe *regexp.Regexp // full-line match for the plain Info line
		chRe    *regexp.Regexp // full-line match for the channel Info line
	}{
		{ // [Time]|I|P:pid|T:tid|file::func(line)] msg
			Core.LogLayoutTypeBuildin1, "Buildin1",
			regexp.MustCompile(`^\[` + ts + `\]\|I\|P:\d+\|T:[1-9]\d*\|[^|]+::[^|]+\(\d+\)\] ` + regexp.QuoteMeta(msg) + `$`),
			regexp.MustCompile(`^\[` + ts + `\]\|I\|P:\d+\|T:[1-9]\d*\|[^|]+::[^|]+\(\d+\)\] \[Channel:` + chName + `\]` + regexp.QuoteMeta(chMsg) + `$`),
		},
		{ // [Time][I]|P:pid|T:tid][func(line)] msg
			Core.LogLayoutTypeBuildin2, "Buildin2",
			regexp.MustCompile(`^\[` + ts + `\]\[I\]\|P:\d+\|T:[1-9]\d*\]\[` + loc + `\] ` + regexp.QuoteMeta(msg) + `$`),
			regexp.MustCompile(`^\[` + ts + `\]\[I\]\|P:\d+\|T:[1-9]\d*\]\[` + loc + `\] \[Channel:` + chName + `\]` + regexp.QuoteMeta(chMsg) + `$`),
		},
		{ // [HH:MM:SS]|I|P:pid|T:tid|func(line)]msg - [file(line)]
			Core.LogLayoutTypeBuildin3, "Buildin3",
			regexp.MustCompile(`^\[\d{2}:\d{2}:\d{2}\]\|I\|P:\d+\|T:[1-9]\d*\|` + loc + `\]` + regexp.QuoteMeta(msg) + ` - \[[^\]]+\(\d+\)\]$`),
			regexp.MustCompile(`^\[\d{2}:\d{2}:\d{2}\]\|I\|P:\d+\|T:[1-9]\d*\|` + loc + `\]\[Channel:` + chName + `\]` + regexp.QuoteMeta(chMsg) + ` - \[[^\]]+\(\d+\)\]$`),
		},
		{ // [Time][I|P:pid|T:tid][Chan][func(line)] msg
			Core.LogLayoutTypeBuildin4, "Buildin4",
			regexp.MustCompile(`^\[` + ts + `\]\[I\|P:\d+\|T:[1-9]\d*\]\[` + loc + `\] ` + regexp.QuoteMeta(msg) + `$`),
			regexp.MustCompile(`^\[` + ts + `\]\[I\|P:\d+\|T:[1-9]\d*\]\[` + chName + `\]\[` + loc + `\] ` + regexp.QuoteMeta(chMsg) + `$`),
		},
	}

	for _, tc := range cases {
		dir := tmpDir("layout_" + tc.name)

		initLog(dir, gologger.WithLayoutType(tc.layout))
		gologger.Info("%s", msg)
		gologger.InfoCh(chName, "%s", chMsg)
		gologger.Flush()
		closeLog()

		files := findLogFiles(dir, "Info")
		if len(files) != 1 {
			cleanupDir(dir)
			return fmt.Errorf("%s: expected 1 Info file, got %d", tc.name, len(files))
		}
		lines := readAllLines(files[0])
		if len(lines) != 2 {
			cleanupDir(dir)
			return fmt.Errorf("%s: expected 2 lines, got %d", tc.name, len(lines))
		}
		if !tc.plainRe.MatchString(lines[0]) {
			cleanupDir(dir)
			return fmt.Errorf("%s: plain line format mismatch:\n  got:  %q\n  want: %s",
				tc.name, lines[0], tc.plainRe)
		}
		if !tc.chRe.MatchString(lines[1]) {
			cleanupDir(dir)
			return fmt.Errorf("%s: channel line format mismatch:\n  got:  %q\n  want: %s",
				tc.name, lines[1], tc.chRe)
		}
		vprintf("  [OK] %s exact-format match (plain + channel)\n", tc.name)
		cleanupDir(dir)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 11. channel — per-message channel field under concurrency, exact counts
// ---------------------------------------------------------------------------

func testChannel() error {
	perCh := *count / 8
	if perCh < 2000 {
		perCh = 2000
	}
	channels := []string{"ChanA", "ChanB", "ChanC", "ChanD"}
	plainN := perCh // additional lines without a channel
	dir := tmpDir("channel")
	defer cleanupDir(dir)

	vprintf("  %d lines x %d channels + %d plain lines (concurrent)\n",
		perCh, len(channels), plainN)

	initLog(dir)
	defer closeLog()

	var wg sync.WaitGroup
	for _, ch := range channels {
		wg.Add(1)
		go func(ch string) {
			defer wg.Done()
			for i := 0; i < perCh; i++ {
				gologger.InfoCh(ch, "channel payload seq=%08d", i)
			}
		}(ch)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < plainN; i++ {
			gologger.Info("plain payload seq=%08d", i)
		}
	}()
	wg.Wait()
	gologger.Flush()

	wantTotal := int64(perCh*len(channels) + plainN)
	gotTotal := fileLinesForType(dir, "Info")
	if gotTotal != wantTotal {
		return fmt.Errorf("total Info lines: got %d, want %d", gotTotal, wantTotal)
	}

	// Exact per-channel counts + plain lines must have no Channel: field.
	chCount := make(map[string]int64, len(channels))
	var plainCount, chTagged int64
	for _, f := range findLogFiles(dir, "Info") {
		for _, line := range readAllLines(f) {
			tagged := false
			for _, ch := range channels {
				if strings.Contains(line, "[Channel:"+ch+"]") {
					chCount[ch]++
					chTagged++
					tagged = true
					break
				}
			}
			if !tagged {
				if strings.Contains(line, "Channel:") {
					return fmt.Errorf("unexpected Channel field on plain line: %q", line)
				}
				plainCount++
			}
		}
	}
	for _, ch := range channels {
		if chCount[ch] != int64(perCh) {
			return fmt.Errorf("channel %s: got %d lines, want %d", ch, chCount[ch], perCh)
		}
		vprintf("  [OK] %s = %d lines\n", ch, chCount[ch])
	}
	if plainCount != int64(plainN) {
		return fmt.Errorf("plain lines: got %d, want %d", plainCount, plainN)
	}
	vprintf("  [OK] plain(no channel) = %d lines, tagged total = %d\n", plainCount, chTagged)

	// Main double-write must carry channels too.
	mainLines := fileLinesForType(dir, "Main")
	if mainLines != wantTotal {
		return fmt.Errorf("Main lines: got %d, want %d", mainLines, wantTotal)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 12. filemode — append (fixed name) vs time (timestamped, rolls per start)
// ---------------------------------------------------------------------------

func testFileMode() error {
	timeNameRe := regexp.MustCompile(`^Info_\d{8}_\d{6}\.log$`)

	// --- append mode: single stable file name, survives re-init ---
	dirA := tmpDir("filemode_append")
	defer cleanupDir(dirA)

	for round := 0; round < 2; round++ {
		initLog(dirA, gologger.WithFileMode(Core.LogFileModeAppend))
		for i := 0; i < 100; i++ {
			gologger.Info("append-mode round=%d seq=%04d", round, i)
		}
		gologger.Flush()
		closeLog()
	}
	filesA := findLogFiles(dirA, "Info")
	if len(filesA) != 1 || filepath.Base(filesA[0]) != "Info.log" {
		names := baseNames(filesA)
		return fmt.Errorf("append mode: want exactly [Info.log], got %v", names)
	}
	if got := countLinesInFile(filesA[0]); got != 200 {
		return fmt.Errorf("append mode: Info.log should accumulate 200 lines, got %d", got)
	}
	vprintf("  [OK] append mode: fixed name Info.log, 200 lines across 2 inits\n")

	// --- time mode: timestamped name, a NEW file per start ---
	dirT := tmpDir("filemode_time")
	defer cleanupDir(dirT)

	for round := 0; round < 2; round++ {
		initLog(dirT, gologger.WithFileMode(Core.LogFileModeTime))
		for i := 0; i < 100; i++ {
			gologger.Info("time-mode round=%d seq=%04d", round, i)
		}
		gologger.Flush()
		closeLog()
		if round == 0 {
			time.Sleep(1100 * time.Millisecond) // ensure a distinct timestamp
		}
	}
	filesT := findLogFiles(dirT, "Info")
	if len(filesT) != 2 {
		return fmt.Errorf("time mode: want 2 files (one per start), got %v", baseNames(filesT))
	}
	var totalT int
	for _, f := range filesT {
		base := filepath.Base(f)
		if !timeNameRe.MatchString(base) {
			return fmt.Errorf("time mode: bad file name %q (want Info_YYYYMMDD_HHMMSS.log)", base)
		}
		totalT += countLinesInFile(f)
	}
	if totalT != 200 {
		return fmt.Errorf("time mode: total lines got %d, want 200", totalT)
	}
	vprintf("  [OK] time mode: 2 timestamped files %v, 100 lines each\n", baseNames(filesT))
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// scanEveryLine returns (true, line) for the first line failing predicate ok.
func scanEveryLine(path string, ok func(string) bool) (bad bool, badLine string) {
	f, err := os.Open(path)
	if err != nil {
		return true, "<open failed: " + err.Error() + ">"
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		if line := sc.Text(); !ok(line) {
			return true, line
		}
	}
	return false, ""
}

func readAllLines(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

func baseNames(paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.Base(p)
	}
	return out
}
