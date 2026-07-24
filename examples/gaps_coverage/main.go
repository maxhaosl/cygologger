// Command gaps_coverage demonstrates the capabilities added to CYGoLogger to
// fully align with the C++ CYLogger library:
//   - One-line init that mirrors the C++ CY_LOG_CONFIG macro, with overrideable
//     defaults (gologger.DefaultConfig / With* options).
//   - ForceNewFile / ResetLogFile (rotate to a fresh file on demand).
//   - ClearConsole (mirrors C++ ClearConsole / Windows AllocConsole).
//   - Remote logging over UDP to align wire-compatibility with C++ (WithRemoteProto).
//   - GetLoggerEntity / GetLogName / GetSize entity inspection.
//   - Statistics (including average FPS) and scheduled non-log file cleanup.
package main

import (
	"fmt"
	"time"

	gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"
	Common "github.com/maxhaosl/CYGoLogger/ICYLogger/Common"
	Core "github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
)

func main() {
	fmt.Println("=== CYGoLogger Gaps-Coverage Example ===")

	// ---- 1. Override defaults then init with one line (mirrors CY_LOG_CONFIG) ----
	cfg := gologger.DefaultConfig()
	cfg.SetShowConsole(true)
	cfg.SetClearPeriodSec(30)
	_ = cfg
	gologger.InitDefaultWithOpts("./logs",
		gologger.WithConsole(true),
		gologger.WithRemoteProto(Core.RemoteProtoUDP), // align with C++ UDP remote appender
		gologger.WithWriteRemote(false),               // no server here; just shows the API
		gologger.WithRestriction(true, true, 60, 24, 300, 60, 5*1024*1024, 20, 500*1024*1024, 1024*1024*1024),
	)
	defer gologger.Close()

	// ---- 2. Standard logging (auto file/line/func capture) ----
	gologger.Info("gap-coverage demo running")
	gologger.Debug("debug detail x=%d", 7)
	gologger.Warn("a warning message")

	// ---- 3. Inspect entities (GetId/GetLogName/GetSize) ----
	if e := gologger.GetLoggerEntity(gologger.LogTypeInfo); e != nil {
		gologger.Info("info entity id=%d logName=%s sizeBytes=%d",
			e.GetId(), e.GetLogName(), e.GetSize())
	}

	// ---- 4. Force a fresh file on demand (mirrors C++ ForceEntityNewFile) ----
	gologger.ForceNewFile()
	gologger.Info("rotated to a fresh file via ForceNewFile")

	// ---- 5. Clear the console screen (mirrors C++ ClearConsole) ----
	gologger.ClearConsole()

	// ---- 6. Hex + escape logging still work ----
	gologger.HexInfo([]byte("binary\x00\x01payload"))
	gologger.EscapeInfo("value with ]brackets, and commas")

	// ---- 7. Common utility coverage (FPS average, PrintLog, TrimString, Verify) ----
	pf := Common.GetCYPublicFunctionInstance()
	pf.PrintLog("printed via CYPublicFunction.PrintLog (level-coloured)")
	pf.PrintTraceLog("printed via CYPublicFunction.PrintTraceLog")
	fmt.Printf("  TrimString(%q) = %q\n", "  hi  ", pf.TrimString("  hi  "))
	fmt.Printf("  Local UTC offset hours = %.2f\n", pf.GetLocalUTCOffsetHours())
	fmt.Printf("  NowNano = %d\n", Common.NowNano())
	// Verify panics on false; here it passes.
	pf.Verify(1+1 == 2, "math broke")

	// ---- 8. FPS average + statistics ----
	fc := Common.NewCYFPSCounter(Core.LOG_FPS_CHECK_DURATION)
	fc.Start()
	for i := 0; i < 50; i++ {
		fc.Increment()
	}
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("  FPS current=%.1f average=%.1f\n", fc.GetFPS(), fc.GetAverageFPS())
	fc.Stop()

	// ---- 9. Runtime configuration shortcuts (one-line, no GetInstance()) ----
	fmt.Println("--- runtime shortcuts ---")
	gologger.SetWriteRemote(true)                            // mount Remote appender
	gologger.SetWriteSys(true)                               // mount Sys appender
	gologger.HexRemote([]byte{0xCA, 0xFE})                   // hex-dump to Remote
	gologger.EscapeRemote("remote msg with ]brackets")       // escape-log to Remote
	gologger.HexSys([]byte{0xBA, 0xBE})                      // hex-dump to Sys
	gologger.EscapeSys("sys msg with ]brackets")             // escape-log to Sys
	gologger.SetLogLevel(gologger.LogFilterWarnsAndErrors)   // raise threshold
	gologger.Info("this INFO is filtered out by LogFilterWarnsAndErrors")
	gologger.SetLogLevel(gologger.LogFilterAll)              // restore
	gologger.SetLayout(gologger.LogLayoutTypeBuildin2)       // switch to date layout
	gologger.Info("layout is now Buildin2")
	gologger.SetLayout(gologger.LogLayoutTypeBuildin1)       // restore
	gologger.Info("layout restored to Buildin1")
	gologger.SetWriteRemote(false)                           // unmount
	gologger.SetWriteSys(false)                              // unmount

	// ---- 10. Scheduled reset of all log files (mirrors C++ ResetLogFile) ----
	gologger.ResetLogFile()
	gologger.Info("all log files reset via ResetLogFile")

	// ---- 11. Statistics snapshot ----
	var stats Core.STStatistics
	if gologger.GetInstance().GetStats(&stats) {
		fmt.Printf("  total lines=%d total bytes=%d\n", stats.NTotalLine, stats.NTotalByte)
	}

	gologger.Flush()
	fmt.Println("=== Example Complete (see ./logs for output) ===")
}
