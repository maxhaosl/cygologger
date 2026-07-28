// Command feature_verify verifies that every C++ ICYLoggerDefine.hpp control
// knob is present and behaves correctly in the Go port (CYGoLogger):
//
//	C++ constant                          Go feature exercised
//	LOG_LIMIT_ENABLE                      master detection switch
//	LOG_LIMIT_CLEAR_UNLOGFILE             non-log file purge
//	LOG_TIME_CLEAR_LOG                    cleanup/rotation check period (s)
//	LOG_TIME_EXPIRED_FILE                 expired-file retention window (h)
//	LOG_CHECK_FILE_SIZE_TIME              size-policy check interval (s)
//	LOG_CHECK_FILE_COUNT_TIME             count-policy check interval (s)
//	LOG_CHECK_FILE_SIZE                   per-file size threshold -> rotation
//	LOG_COUNT_PER_TYPE                    max files kept per log type
//	LOG_CHECK_FILE_TYPE_SIZE              per-type total size cap
//	LOG_CHECK_FILE_ALL_SIZE               global total size cap
//
// Each feature is asserted two ways: (1) its default value matches the C++
// header, and (2) its runtime behaviour is exercised directly. A final
// summary prints PASS/FAIL counts and exits non-zero if anything failed.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"
	Appender "github.com/maxhaosl/CYGoLogger/ICYLogger/Appender"
	Common "github.com/maxhaosl/CYGoLogger/ICYLogger/Common"
	Core "github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
	Schedule "github.com/maxhaosl/CYGoLogger/ICYLogger/Schedule"
)

var (
	passCount, failCount int
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
		if err == nil && !d.IsDir() && filepath.Ext(p) == ".log" {
			out = append(out, p)
		}
		return nil
	})
	return out
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

func main() {
	fmt.Println("=== CYGoLogger feature verification (mirrors C++ ICYLoggerDefine) ===")

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
	writeFile(small, "hello")                            // 5 bytes  < 5MB
	writeFile(big, string(make([]byte, 6*1024*1024)))    // 6MB     > 5MB
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

	// master switch off -> nothing removed
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
	// 24h expiry window so freshly-created (seconds-old) files are NOT expired;
	// only the per-type count limit should act here.
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
	clG := Schedule.NewCYLoggerClearLogFile(dirG, 24) // 24h expiry -> fresh files not expired
	clG.SetRestriction(1000, 500, 1<<40)              // per-type cap 500 bytes
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
	clH := Schedule.NewCYLoggerClearLogFile(dirH, 24) // 24h expiry -> fresh files not expired
	clH.SetRestriction(1000, 1<<30, 500)              // global cap 500, huge per-type
	clH.DoClear()
	check("global total <= 500 bytes", sumSizes(listLogs(dirH)) <= 500)
	check("global cap actually deleted files", len(listLogs(dirH)) < 5)

	// ------------------------------------------------------------------
	// [I] LOG_LIMIT_CLEAR_UNLOGFILE.
	// ------------------------------------------------------------------
	fmt.Println("\n[I] LOG_LIMIT_CLEAR_UNLOGFILE: purge non-log files")
	dirI, _ := os.MkdirTemp("", "cygo-nl")
	defer os.RemoveAll(dirI)
	zip := filepath.Join(dirI, "old_package.zip")
	txt := filepath.Join(dirI, "stray.txt")
	writeFile(zip, "junk")
	writeFile(txt, "junk")
	clI := Schedule.NewCYLoggerClearLogFile(dirI, 24)
	clI.SetRestriction(1000, 1<<30, 1<<40)
	clI.SetClearUnLogFile(true)
	clI.DoClear()
	check("non-log .zip removed", !fileExists(zip))
	check("non-log .txt removed", !fileExists(txt))

	// flag off -> non-log files kept
	dirI2, _ := os.MkdirTemp("", "cygo-nl-off")
	defer os.RemoveAll(dirI2)
	zip2 := filepath.Join(dirI2, "old_package.zip")
	writeFile(zip2, "junk")
	clI2 := Schedule.NewCYLoggerClearLogFile(dirI2, 24)
	clI2.SetRestriction(1000, 1<<30, 1<<40)
	clI2.SetClearUnLogFile(false)
	clI2.DoClear()
	check("non-log file kept when flag=false", fileExists(zip2))

	// ------------------------------------------------------------------
	// [J] Live logger integration: restriction wiring + size-based rotation.
	// ------------------------------------------------------------------
	fmt.Println("\n[J] Live logger: restriction wiring + size-based rotation")
	// Use a small per-file threshold (1024 bytes) via WithRestriction only; the
	// rotation must then fire through the control->appender propagation path
	// (Bug B fix), i.e. without any manual fa.SetRestriction call.
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

	// WithRestriction's per-file threshold must reach the live appender via the
	// shared restriction object (Bug B fix) — no manual SetRestriction here.
	ent := gologger.GetLoggerEntity(gologger.LogTypeInfo)
	if ent != nil && ent.GetAppenderCount() > 0 {
		if fa, ok := ent.GetAppender(0).(*Appender.CYLoggerFileAppender); ok {
			check("WithRestriction nCheckFileSize propagated to appender", fa.GetRestriction().GetCheckFileSize() == 1024)
			before := len(listLogs("./logs"))
			for i := 0; i < 60; i++ {
				gologger.Info("rotation probe line %04d padding-padding-padding", i)
			}
			gologger.Flush()
			check("size-based rotation created a new log file", len(listLogs("./logs")) > before)
		} else {
			check("Info appender is *CYLoggerFileAppender", false)
		}
	} else {
		check("Info entity/appender available", false)
	}

	// ForceNewFile rotates every file appender to a fresh file (C++ ForceEntityNewFile).
	base := len(listLogs("./logs"))
	gologger.ForceNewFile()
	gologger.Info("line written after ForceNewFile")
	gologger.Flush()
	check("ForceNewFile produced an additional file", len(listLogs("./logs")) > base)

	// ------------------------------------------------------------------
	fmt.Printf("\n=== RESULT: %d passed, %d failed ===\n", passCount, failCount)
	if failCount > 0 {
		os.Exit(1)
	}
}
