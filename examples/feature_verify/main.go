// Command feature_verify is the single entry point that proves every
// cygologger capability works end-to-end through the public Go API.
//
// It is a full-feature matrix: each capability is exercised and asserted,
// printing one PASS/FAIL line per feature and a final tally. The process
// exits non-zero if any feature fails, so it can be wired into CI / the
// top-level Build/verify.sh "one-click" gate.
//
// Mode- and daemon-dependent options that require a fresh process (console
// window, remote TCP/UDP, system/syslog, file-name mode, layout selector)
// are verified in isolation by examples/config_verify (run via -opt). This
// file covers everything testable inside one initialized logger instance.
package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gologger "github.com/maxhaosl/cygologger/ICYLogger"
	Appender "github.com/maxhaosl/cygologger/ICYLogger/Appender"
	Common "github.com/maxhaosl/cygologger/ICYLogger/Common"
	Core "github.com/maxhaosl/cygologger/ICYLogger/Core"
	Schedule "github.com/maxhaosl/cygologger/ICYLogger/Schedule"
	UpLoad "github.com/maxhaosl/cygologger/ICYLogger/UpLoad"
)

var (
	passCount, failCount int
	runID                = fmt.Sprintf("%d", time.Now().UnixNano())
)

func check(name string, ok bool) {
	if ok {
		passCount++
		fmt.Printf("  [PASS] %s\n", name)
	} else {
		failCount++
		fmt.Printf("  [FAIL] %s\n", name)
	}
}

func mk(s string) string { return s + "-" + runID }

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func writeFile(path, content string) {
	_ = os.WriteFile(path, []byte(content), 0644)
}

func touchPast(path string, dur time.Duration) {
	t := time.Now().Add(-dur)
	_ = os.Chtimes(path, t, t)
}

func listLogs(dir string) []string {
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ".log") {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func readAllLogs(dir string) string {
	var sb strings.Builder
	for _, f := range listLogs(dir) {
		if b, err := os.ReadFile(f); err == nil {
			sb.Write(b)
		}
	}
	return sb.String()
}

func sumSizes(paths []string) int64 {
	var total int64
	for _, f := range paths {
		if fi, err := os.Stat(f); err == nil {
			total += fi.Size()
		}
	}
	return total
}

// inProcessFTP survives just long enough to receive one STOR, recording the
// remote path and payload so we can assert the upload feature end-to-end.
type inProcessFTP struct {
	ln       net.Listener
	mu       sync.Mutex
	storPath string
	data     []byte
	err      error
	done     chan struct{}
}

func startFTP() *inProcessFTP {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return &inProcessFTP{err: err, done: make(chan struct{})}
	}
	s := &inProcessFTP{ln: ln, done: make(chan struct{})}
	go s.serve()
	return s
}

func (s *inProcessFTP) addr() (string, int) {
	tcp := s.ln.Addr().(*net.TCPAddr)
	return tcp.IP.String(), tcp.Port
}

func (s *inProcessFTP) serve() {
	defer close(s.done)
	conn, err := s.ln.Accept()
	if err != nil {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
		return
	}
	defer conn.Close()
	w := func(format string, args ...any) { fmt.Fprintf(conn, format+"\r\n", args...) }
	w("220 fake-ftp ready")
	r := bufio.NewReader(conn)
	var dataLn net.Listener
	defer func() {
		if dataLn != nil {
			_ = dataLn.Close()
		}
	}()
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		arg := ""
		if idx := strings.Index(line, " "); idx >= 0 {
			arg = line[idx+1:]
		}
		switch cmd {
		case "USER":
			w("331 need password")
		case "PASS":
			w("230 logged in")
		case "TYPE":
			w("200 type set")
		case "PASV":
			dl, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				w("425 cannot open data port")
				continue
			}
			dataLn = dl
			p := dl.Addr().(*net.TCPAddr).Port
			w("227 Entering Passive Mode (127,0,0,1,%d,%d)", p/256, p%256)
		case "STOR":
			s.mu.Lock()
			s.storPath = arg
			s.mu.Unlock()
			w("150 ok, send data")
			dc, e := dataLn.Accept()
			if e != nil {
				return
			}
			payload, _ := io.ReadAll(dc)
			_ = dc.Close()
			s.mu.Lock()
			s.data = payload
			s.mu.Unlock()
			w("226 transfer complete")
		case "QUIT":
			w("221 bye")
			return
		default:
			w("502 not implemented")
		}
	}
}

func main() {
	fmt.Println("=== cygologger full feature matrix ===")

	// ------------------------------------------------------------------
	// [A] Compile-time default constants equal the C++ header values.
	// ------------------------------------------------------------------
	fmt.Println("\n[A] C++ default constants present in Go (Core.*)")
	check("LOG_LIMIT_ENABLE == true", Core.LOG_LIMIT_ENABLE == true)
	check("LOG_LIMIT_CLEAR_UNLOGFILE == true", Core.LOG_LIMIT_CLEAR_UNLOGFILE == true)
	check("LOG_TIME_CLEAR_LOG == 60", Core.LOG_TIME_CLEAR_LOG == 60)
	check("LOG_TIME_EXPIRED_FILE == 24", Core.LOG_TIME_EXPIRED_FILE == 24)
	check("LOG_CHECK_FILE_SIZE_TIME == 300", Core.LOG_CHECK_FILE_SIZE_TIME == 300)
	check("LOG_CHECK_FILE_COUNT_TIME == 60", Core.LOG_CHECK_FILE_COUNT_TIME == 60)
	check("LOG_CHECK_FILE_SIZE == 5MB", Core.LOG_CHECK_FILE_SIZE == 5*1024*1024)
	check("LOG_COUNT_PER_TYPE == 20", Core.LOG_COUNT_PER_TYPE == 20)
	check("LOG_CHECK_FILE_TYPE_SIZE == 500MB", Core.LOG_CHECK_FILE_TYPE_SIZE == 500*1024*1024)
	check("LOG_CHECK_FILE_ALL_SIZE == 1GB", Core.LOG_CHECK_FILE_ALL_SIZE == 1024*1024*1024)

	// ------------------------------------------------------------------
	// [B] Runtime CYFileRestriction defaults match the constants.
	// ------------------------------------------------------------------
	fmt.Println("\n[B] CYFileRestriction defaults match")
	fr := Common.NewCYFileRestriction()
	check("IsEnableCheck() == true", fr.IsEnableCheck())
	check("IsClearUnLogFile() == true", fr.IsClearUnLogFile())
	check("GetTimeClearLog() == 60", fr.GetTimeClearLog() == 60)
	check("GetTimeExpiredFile() == 24", fr.GetTimeExpiredFile() == 24)
	check("GetCheckFileSizeTime() == 300", fr.GetCheckFileSizeTime() == 300)
	check("GetCheckFileCountTime() == 60", fr.GetCheckFileCountTime() == 60)
	check("GetCheckFileSize() == 5MB", fr.GetCheckFileSize() == 5*1024*1024)
	check("GetFileCountPerType() == 20", fr.GetFileCountPerType() == 20)
	check("GetCheckFileTypeSize() == 500MB", fr.GetCheckFileTypeSize() == 500*1024*1024)
	check("GetCheckAllFileSize() == 1GB", fr.GetCheckAllFileSize() == 1024*1024*1024)

	// ------------------------------------------------------------------
	// [C] Core config defaults match.
	// ------------------------------------------------------------------
	fmt.Println("\n[C] CYLoggerConfig defaults match")
	cfg := Core.DefaultConfig()
	check("IsLimitEnable() == true", cfg.IsLimitEnable())
	check("IsClearUnLogFile() == true", cfg.IsClearUnLogFile())
	check("GetTimeClearLog() == 60", cfg.GetTimeClearLog() == 60)
	check("GetTimeExpiredFile() == 24", cfg.GetTimeExpiredFile() == 24)
	check("GetCheckFileSizeTime() == 300", cfg.GetCheckFileSizeTime() == 300)
	check("GetCheckFileCountTime() == 60", cfg.GetCheckFileCountTime() == 60)
	check("GetCheckFileSize() == 5MB", cfg.GetCheckFileSize() == 5*1024*1024)
	check("GetCountPerType() == 20", cfg.GetCountPerType() == 20)
	check("GetCheckFileTypeSize() == 500MB", cfg.GetCheckFileTypeSize() == 500*1024*1024)
	check("GetCheckAllFileSize() == 1GB", cfg.GetCheckAllFileSize() == 1024*1024*1024)
	check("GetClearPeriodSec() == 60", cfg.GetClearPeriodSec() == 60)

	// ------------------------------------------------------------------
	// [D] LOG_CHECK_FILE_SIZE: per-file size threshold drives rotation.
	// ------------------------------------------------------------------
	fmt.Println("\n[D] LOG_CHECK_FILE_SIZE: per-file size threshold")
	dirD, _ := os.MkdirTemp("", "cygo-size")
	defer os.RemoveAll(dirD)
	small := filepath.Join(dirD, "Info_small.log")
	big := filepath.Join(dirD, "Info_big.log")
	writeFile(small, "hello")                         // 5 bytes  < 5MB
	writeFile(big, string(make([]byte, 6*1024*1024))) // 6MB     > 5MB
	check("CheckFileSize(< 5MB) == false", !fr.CheckFileSize(small))
	check("CheckFileSize(> 5MB) == true", fr.CheckFileSize(big))
	check("IsCreateNewLog(6MB) == true", fr.IsCreateNewLog(6*1024*1024))
	check("IsCreateNewLog(1MB) == false", !fr.IsCreateNewLog(1*1024*1024))

	// ------------------------------------------------------------------
	// [E] LOG_LIMIT_ENABLE + LOG_TIME_EXPIRED_FILE.
	// ------------------------------------------------------------------
	fmt.Println("\n[E] LOG_LIMIT_ENABLE + LOG_TIME_EXPIRED_FILE: expired cleanup")
	dirE, _ := os.MkdirTemp("", "cygo-exp")
	defer os.RemoveAll(dirE)
	expiredFile := filepath.Join(dirE, "Info_old.log")
	writeFile(expiredFile, "stale")
	touchPast(expiredFile, 48*time.Hour) // 48h ago, older than the 24h window
	cl := Schedule.NewCYLoggerClearLogFile(dirE, 24)
	cl.SetRestriction(1000, 1<<30, 1<<40) // huge count/type-size/all-size -> only expiry acts
	cl.SetClearUnLogFile(true)
	cl.DoClear()
	check("expired file removed when enabled", !fileExists(expiredFile))

	dirE2, _ := os.MkdirTemp("", "cygo-exp-off")
	defer os.RemoveAll(dirE2)
	keepFile := filepath.Join(dirE2, "Info_old.log")
	writeFile(keepFile, "stale")
	touchPast(keepFile, 48*time.Hour)
	cl2 := Schedule.NewCYLoggerClearLogFile(dirE2, 24)
	cl2.SetRestriction(1000, 1<<30, 1<<40)
	cl2.SetEnable(false) // LOG_LIMIT_ENABLE = false
	cl2.DoClear()
	check("expired file kept when LOG_LIMIT_ENABLE=false", fileExists(keepFile))

	// ------------------------------------------------------------------
	// [F] LOG_COUNT_PER_TYPE.
	// ------------------------------------------------------------------
	fmt.Println("\n[F] LOG_COUNT_PER_TYPE: max files per log type")
	dirF, _ := os.MkdirTemp("", "cygo-cnt")
	defer os.RemoveAll(dirF)
	for i := 0; i < 25; i++ {
		writeFile(filepath.Join(dirF, fmt.Sprintf("Info_%02d.log", i)), "x")
	}
	clF := Schedule.NewCYLoggerClearLogFile(dirF, 24)
	clF.SetRestriction(20, 1<<30, 1<<40) // keep 20 per type
	clF.DoClear()
	check("files kept == 20 after count limit", len(listLogs(dirF)) == 20)

	// ------------------------------------------------------------------
	// [G] LOG_CHECK_FILE_TYPE_SIZE.
	// ------------------------------------------------------------------
	fmt.Println("\n[G] LOG_CHECK_FILE_TYPE_SIZE: per-type total size cap")
	dirG, _ := os.MkdirTemp("", "cygo-typ")
	defer os.RemoveAll(dirG)
	for i := 0; i < 5; i++ {
		writeFile(filepath.Join(dirG, fmt.Sprintf("Warn_%d.log", i)), string(make([]byte, 200))) // 1000 total
	}
	clG := Schedule.NewCYLoggerClearLogFile(dirG, 24)
	clG.SetRestriction(1000, 500, 1<<40) // per-type cap 500 bytes
	clG.DoClear()
	check("per-type total <= 500 bytes", sumSizes(listLogs(dirG)) <= 500)
	check("per-type cap actually deleted files", len(listLogs(dirG)) < 5)

	// ------------------------------------------------------------------
	// [H] LOG_CHECK_FILE_ALL_SIZE.
	// ------------------------------------------------------------------
	fmt.Println("\n[H] LOG_CHECK_FILE_ALL_SIZE: global total size cap")
	dirH, _ := os.MkdirTemp("", "cygo-all")
	defer os.RemoveAll(dirH)
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("Info_%d.log", i)
		if i%2 == 1 {
			name = fmt.Sprintf("Warn_%d.log", i)
		}
		writeFile(filepath.Join(dirH, name), string(make([]byte, 200))) // 1000 total
	}
	clH := Schedule.NewCYLoggerClearLogFile(dirH, 24)
	clH.SetRestriction(1000, 1<<30, 500) // global cap 500, huge per-type
	clH.DoClear()
	check("global total <= 500 bytes", sumSizes(listLogs(dirH)) <= 500)
	check("global cap actually deleted files", len(listLogs(dirH)) < 5)

	// ------------------------------------------------------------------
	// [I] LOG_LIMIT_CLEAR_UNLOGFILE.
	// ------------------------------------------------------------------
	fmt.Println("\n[I] LOG_LIMIT_CLEAR_UNLOGFILE: purge non-log files")
	dirI, _ := os.MkdirTemp("", "cygo-nl")
	defer os.RemoveAll(dirI)
	zipFile := filepath.Join(dirI, "old_package.zip")
	txt := filepath.Join(dirI, "stray.txt")
	writeFile(zipFile, "junk")
	writeFile(txt, "junk")
	clI := Schedule.NewCYLoggerClearLogFile(dirI, 24)
	clI.SetRestriction(1000, 1<<30, 1<<40)
	clI.SetClearUnLogFile(true)
	clI.DoClear()
	check("non-log .zip removed", !fileExists(zipFile))
	check("non-log .txt removed", !fileExists(txt))

	dirI2, _ := os.MkdirTemp("", "cygo-nl-off")
	defer os.RemoveAll(dirI2)
	zipFile2 := filepath.Join(dirI2, "old_package.zip")
	writeFile(zipFile2, "junk")
	clI2 := Schedule.NewCYLoggerClearLogFile(dirI2, 24)
	clI2.SetRestriction(1000, 1<<30, 1<<40)
	clI2.SetClearUnLogFile(false)
	clI2.DoClear()
	check("non-log file kept when flag=false", fileExists(zipFile2))

	// ------------------------------------------------------------------
	// [J] Live logger: restriction wiring + size-based rotation + ForceNewFile.
	// ------------------------------------------------------------------
	fmt.Println("\n[J] Live logger: init, restriction wiring, rotation, ForceNewFile")
	gologger.InitDefaultWithOpts("./logs",
		gologger.WithConsole(false),
		gologger.WithFileMode(Core.LogFileModeAppend),
		gologger.WithRestriction(true, true, 60, 24, 300, 60, 1024, 20, 500*1024*1024, 1024*1024*1024),
	)
	defer gologger.Close()
	single := Core.GetCYLoggerConfigInstance()
	check("singleton IsLimitEnable() == true", single.IsLimitEnable())
	check("singleton IsClearUnLogFile() == true", single.IsClearUnLogFile())
	check("singleton GetTimeClearLog() == 60", single.GetTimeClearLog() == 60)
	check("singleton GetTimeExpiredFile() == 24", single.GetTimeExpiredFile() == 24)
	check("singleton GetCheckFileSizeTime() == 300", single.GetCheckFileSizeTime() == 300)
	check("singleton GetCheckFileCountTime() == 60", single.GetCheckFileCountTime() == 60)
	check("singleton GetCheckFileSize() == 1024", single.GetCheckFileSize() == 1024)
	check("singleton GetCountPerType() == 20", single.GetCountPerType() == 20)
	check("singleton GetCheckFileTypeSize() == 500MB", single.GetCheckFileTypeSize() == 500*1024*1024)
	check("singleton GetCheckAllFileSize() == 1GB", single.GetCheckAllFileSize() == 1024*1024*1024)

	ent := gologger.GetLoggerEntity(Core.LogTypeInfo)
	if ent != nil && ent.GetAppenderCount() > 0 && ent.GetAppender(0) != nil {
		before := len(listLogs("./logs"))
		for i := 0; i < 60; i++ {
			gologger.Info("rotation probe line %04d padding-padding-padding", i)
		}
		gologger.Flush()
		check("size-based rotation created a new log file", len(listLogs("./logs")) > before)
	} else {
		check("Info entity/appender available", false)
	}

	// Bump the per-file size so the remaining functional checks do not churn
	// through hundreds of rotated files. The restriction object is shared by
	// every appender, so one write is enough. Done AFTER the rotation check
	// above so that check still exercises the small 1024-byte threshold.
	if fa, ok := ent.GetAppender(0).(*Appender.CYLoggerFileAppender); ok {
		fa.GetRestriction().SetRestriction(true, true, 60, 24, 300, 60, 50*1024*1024, 20, 500*1024*1024, 1024*1024*1024)
	}

	base := len(listLogs("./logs"))
	gologger.ForceNewFile()
	gologger.Info("line written after ForceNewFile " + mk("FORCE"))
	gologger.Flush()
	check("ForceNewFile produced an additional file", len(listLogs("./logs")) > base)

	// ------------------------------------------------------------------
	// [K] Log levels & types write to distinct destinations.
	// ------------------------------------------------------------------
	fmt.Println("\n[K] Log levels & types (Trace/Debug/Info/Warn/Error/Fatal/Main)")
	gologger.Trace(mk("LVL-TRACE"))
	gologger.Debug(mk("LVL-DEBUG"))
	gologger.Info(mk("LVL-INFO"))
	gologger.Warn(mk("LVL-WARN"))
	gologger.Error(mk("LVL-ERROR"))
	gologger.Fatal(mk("LVL-FATAL"))
	gologger.Main(mk("LVL-MAIN"))
	gologger.Remote(mk("LVL-REMOTE")) // no remote appender mounted -> safe no-op
	gologger.Sys(mk("LVL-SYS"))       // no sys appender mounted   -> safe no-op
	gologger.Flush()
	logs := readAllLogs("./logs")
	check("Trace level written", strings.Contains(logs, mk("LVL-TRACE")))
	check("Debug level written", strings.Contains(logs, mk("LVL-DEBUG")))
	check("Info level written", strings.Contains(logs, mk("LVL-INFO")))
	check("Warn level written", strings.Contains(logs, mk("LVL-WARN")))
	check("Error level written", strings.Contains(logs, mk("LVL-ERROR")))
	check("Fatal level written (no os.Exit)", strings.Contains(logs, mk("LVL-FATAL")))
	check("Main level written", strings.Contains(logs, mk("LVL-MAIN")))

	// ------------------------------------------------------------------
	// [L] Channel-aware logging renders the channel tag.
	// ------------------------------------------------------------------
	fmt.Println("\n[L] Channel-aware logging")
	gologger.InfoCh("BIZCHANNEL", mk("CHAN-BODY"))
	gologger.Flush()
	clogs := readAllLogs("./logs")
	check("channel name rendered", strings.Contains(clogs, "BIZCHANNEL"))
	check("channel message body written", strings.Contains(clogs, mk("CHAN-BODY")))

	// ------------------------------------------------------------------
	// [M] Direct logging bypasses the level filter.
	// ------------------------------------------------------------------
	fmt.Println("\n[M] Direct logging bypasses level filter")
	gologger.SetLogLevel(gologger.LogFilterWarnsAndErrors) // excludes Trace/Debug
	gologger.Trace(mk("TRACE-FILTERED"))                  // filtered out
	gologger.DirectTrace(mk("DIRECT-TRACE"))              // bypasses filter
	gologger.Flush()
	flogs := readAllLogs("./logs")
	check("filtered Trace level suppressed", !strings.Contains(flogs, mk("TRACE-FILTERED")))
	check("Direct Trace still written", strings.Contains(flogs, mk("DIRECT-TRACE")))
	gologger.SetLogLevel(gologger.LogFilterAll) // restore: all levels

	// ------------------------------------------------------------------
	// [N] Escape-formatted logging.
	// ------------------------------------------------------------------
	fmt.Println("\n[N] Escape-formatted logging")
	gologger.EscapeInfo("%s", mk("ESC-BODY"))
	gologger.Flush()
	elogs := readAllLogs("./logs")
	check("escape message body written", strings.Contains(elogs, mk("ESC-BODY")))

	// ------------------------------------------------------------------
	// [O] Hex dump logging.
	// ------------------------------------------------------------------
	fmt.Println("\n[O] Hex dump logging")
	gologger.HexInfo([]byte("ABC"))
	gologger.Flush()
	hlogs := readAllLogs("./logs")
	// 'A'=0x41 'B'=0x42 'C'=0x43 appear in the canonical hex block.
	check("hex dump renders byte values", strings.Contains(hlogs, "41 42 43"))

	// ------------------------------------------------------------------
	// [P] Scope enter/exit logging via defer.
	// ------------------------------------------------------------------
	fmt.Println("\n[P] Scope enter/exit logging")
	func() {
		defer gologger.Scope()()
		gologger.Debug(mk("INSIDE-SCOPE"))
	}()
	gologger.Flush()
	slogs := readAllLogs("./logs")
	check("Scope ENTER logged", strings.Contains(slogs, "ENTER:"))
	check("Scope EXIT logged", strings.Contains(slogs, "EXIT:"))

	// ------------------------------------------------------------------
	// [Q] Concurrent, async-safe writes all persisted after Flush.
	// ------------------------------------------------------------------
	fmt.Println("\n[Q] Concurrent async-safe writes")
	const writers = 8
	const perWriter = 20
	want := writers * perWriter
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				gologger.Info(mk(fmt.Sprintf("ASYNC-%d-%d", w, i)))
			}
		}(w)
	}
	wg.Wait()
	gologger.Flush()
	qlogs := readAllLogs("./logs")
	got := 0
	for w := 0; w < writers; w++ {
		for i := 0; i < perWriter; i++ {
			if strings.Contains(qlogs, mk(fmt.Sprintf("ASYNC-%d-%d", w, i))) {
				got++
			}
		}
	}
	check(fmt.Sprintf("all %d concurrent messages persisted", want), got == want)

	// ------------------------------------------------------------------
	// [R] Template layouts (Buildin1..4) produce distinct output.
	// ------------------------------------------------------------------
	fmt.Println("\n[R] Template layouts Buildin1..4")
	lm := gologger.GetCYLoggerTemplateLayoutManagerInstance()
	fmtWith := func(t Core.ELogLayoutType) string {
		return lm.GetLayout(t).GetFormatMessage(
			"CH", Core.LogTypeInfo, int(Core.LogLevelInfo),
			mk("LAYOUT-BODY"), "file.go", "myFunc", 42, 1234, 56,
			2026, 7, 28, 12, 0, 0, 0, false,
		)
	}
	l1, l2, l3, l4 := fmtWith(Core.LogLayoutTypeBuildin1), fmtWith(Core.LogLayoutTypeBuildin2),
		fmtWith(Core.LogLayoutTypeBuildin3), fmtWith(Core.LogLayoutTypeBuildin4)
	check("Buildin1 renders the year 2026", strings.Contains(l1, "2026"))
	check("layouts are pairwise distinct",
		l1 != l2 && l1 != l3 && l1 != l4 && l2 != l3 && l2 != l4 && l3 != l4)

	// ------------------------------------------------------------------
	// [S] Compression (ZipLog).
	// ------------------------------------------------------------------
	fmt.Println("\n[S] Compression (ZipLog)")
	dirS, _ := os.MkdirTemp("", "cygo-zip")
	defer os.RemoveAll(dirS)
	src := filepath.Join(dirS, "plain.log")
	dst := filepath.Join(dirS, "plain.zip")
	writeFile(src, "compress-me-"+runID)
	okZip := gologger.ZipLog(src, dst)
	check("ZipLog produced archive", okZip && fileExists(dst))
	if okZip && fileExists(dst) {
		if zr, err := zip.OpenReader(dst); err == nil {
			check("zip archive is readable & non-empty", len(zr.File) > 0)
			_ = zr.Close()
		} else {
			check("zip archive is readable & non-empty", false)
		}
	}

	// ------------------------------------------------------------------
	// [T] AES-256-GCM encryption round trip.
	// ------------------------------------------------------------------
	fmt.Println("\n[T] AES-256-GCM encryption")
	enc, errEnc := gologger.NewAESEncryptor([]byte("32-byte-secret-key-for-aes-256!!"))
	check("NewAESEncryptor succeeds", errEnc == nil && enc != nil)
	if enc != nil {
		plain := []byte("secret-PLAINTEXT-" + runID)
		ct, errC := enc.Encrypt(plain)
		check("Encrypt succeeds", errC == nil && len(ct) > 0)
		check("ciphertext differs from plaintext", !bytes.Equal(ct, plain))
		pt, errD := enc.Decrypt(ct)
		check("Decrypt round-trips", errD == nil && bytes.Equal(pt, plain))
	}

	// ------------------------------------------------------------------
	// [U] Statistics grow with writes.
	// ------------------------------------------------------------------
	fmt.Println("\n[U] Statistics counters")
	if entU := gologger.GetLoggerEntity(Core.LogTypeInfo); entU != nil && entU.GetAppenderCount() > 0 {
		if fa, ok := entU.GetAppender(0).(*Appender.CYLoggerFileAppender); ok {
			before := fa.GetStatsLine()
			for i := 0; i < 25; i++ {
				gologger.Info(mk(fmt.Sprintf("STATS-%d", i)))
			}
			gologger.Flush()
			check("appender line counter grew", fa.GetStatsLine() > before)
		} else {
			check("Info appender is *CYLoggerFileAppender", false)
		}
	} else {
		check("Info entity/appender available for stats", false)
	}

	// ------------------------------------------------------------------
	// [V] Panic / exception capture (SafeGo recovers, logs stack).
	// ------------------------------------------------------------------
	fmt.Println("\n[V] Panic / exception capture")
	gologger.InitException("./logs")
	gologger.SafeGo(func() { panic("EXC-" + runID) })
	time.Sleep(200 * time.Millisecond)
	vlogs := readAllLogs("./logs")
	check("panic captured to exception log", strings.Contains(vlogs, "EXC-"+runID))

	// ------------------------------------------------------------------
	// [W] Entity inspection API.
	// ------------------------------------------------------------------
	fmt.Println("\n[W] Entity inspection")
	ew := gologger.GetLoggerEntity(Core.LogTypeInfo)
	check("GetLoggerEntity(Info) non-nil", ew != nil)
	if ew != nil {
		check("GetAppenderCount() > 0", ew.GetAppenderCount() > 0)
		check("GetLogName() returns a file path", ew.GetLogName() != "")
		check("GetSize() > 0 after writes", ew.GetSize() > 0)
	}

	// ------------------------------------------------------------------
	// [X] FTP upload (in-process server, end-to-end).
	// ------------------------------------------------------------------
	fmt.Println("\n[X] FTP upload")
	ftp := startFTP()
	if ftp.err != nil {
		check("FTP server started", false)
	} else {
		host, port := ftp.addr()
		dirX, _ := os.MkdirTemp("", "cygo-ftp")
		defer os.RemoveAll(dirX)
		local := filepath.Join(dirX, "Info.log")
		content := []byte("ftp-upload-payload-" + runID + "\n\x00tail")
		writeFile(local, string(content))
		cfgFTP := &UpLoad.CYUpLoadConfig{
			Host:       host,
			Port:       port,
			User:       "u",
			Password:   "p",
			RemoteDir:  "/logs",
			TimeoutSec: 5,
			Passive:    true,
		}
		errUp := gologger.UploadLogFTP(cfgFTP, local, "")
		select {
		case <-ftp.done:
		case <-time.After(5 * time.Second):
		}
		ftp.mu.Lock()
		path, data, serr := ftp.storPath, ftp.data, ftp.err
		ftp.mu.Unlock()
		check("UploadLogFTP completed without error", errUp == nil && serr == nil)
		check("uploaded to /logs/Info.log", path == "/logs/Info.log")
		check("uploaded payload byte-exact", bytes.Equal(data, content))
	}

	// ------------------------------------------------------------------
	fmt.Printf("\n=== RESULT: %d passed, %d failed ===\n", passCount, failCount)
	if failCount > 0 {
		os.Exit(1)
	}
}
