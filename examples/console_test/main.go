// console_test demonstrates all logging macros with console output.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	gologger "github.com/maxhaosl/cygologger/ICYLogger"
)

func main() {
	fmt.Println("=== cygologger Console Test ===")
	fmt.Println()

	logPath := "./Log/console"
	ok := gologger.InitLogger(logPath, true)
	if !ok {
		fmt.Println("Failed to initialize logger")
		os.Exit(1)
	}
	fmt.Println("Logger initialized successfully")

	gologger.GetInstance().AddAppender(
		gologger.LogTypeConsole, "", "", gologger.LogFileModeAppend)

	gologger.GetInstance().AddAppender(
		gologger.LogTypeTrace, "test", "Trace_test.log", gologger.LogFileModeTime)
	gologger.GetInstance().AddAppender(
		gologger.LogTypeDebug, "test", "Debug_test.log", gologger.LogFileModeTime)
	gologger.GetInstance().AddAppender(
		gologger.LogTypeInfo, "test", "Info_test.log", gologger.LogFileModeTime)
	gologger.GetInstance().AddAppender(
		gologger.LogTypeWarn, "test", "Warn_test.log", gologger.LogFileModeTime)
	gologger.GetInstance().AddAppender(
		gologger.LogTypeError, "test", "Error_test.log", gologger.LogFileModeTime)
	gologger.GetInstance().AddAppender(
		gologger.LogTypeFatal, "test", "Fatal_test.log", gologger.LogFileModeTime)
	gologger.GetInstance().AddAppender(
		gologger.LogTypeMain, "test", "Main_test.log", gologger.LogFileModeTime)

	fmt.Println("All appenders registered")
	fmt.Println()

	gologger.LOG_TRACE("This is a TRACE message: count=%d", 1)
	gologger.LOG_DEBUG("This is a DEBUG message: count=%d", 2)
	gologger.LOG_INFO("This is an INFO message: count=%d", 3)
	gologger.LOG_WARN("This is a WARN message: count=%d", 4)
	gologger.LOG_ERROR("This is an ERROR message: count=%d", 5)
	gologger.LOG_FATAL("This is a FATAL message: count=%d", 6)
	gologger.LOG_MAIN("This is a MAIN message: count=%d", 7)
	gologger.LOG_SYS("This is a SYS message: count=%d", 8)

	fmt.Println()
	fmt.Println("Basic logging test complete")

	gologger.GetInstance().WriteLogFmt(
		int(gologger.LogLevelInfo), gologger.LogTypeInfo, 100,
		"console_test.go", "main", 95,
		"Structured INFO message with server code %d", 100)

	gologger.GetInstance().WriteEscapeLogFmt(
		int(gologger.LogLevelInfo), gologger.LogTypeInfo, -1,
		"console_test.go", "main", 110,
		"Escape message with special chars: [test] and ]more[")

	hexData := []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x2c, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21}
	gologger.GetInstance().WriteHexLog(
		int(gologger.LogLevelDebug), gologger.LogTypeDebug, -1,
		"console_test.go", "main", 119, hexData)

	gologger.GetInstance().WriteLog(
		int(gologger.LogLevelInfo), gologger.LogTypeInfo, -1,
		"Direct message without formatting")

	fmt.Println()
	fmt.Println("Testing filter level change to WARN and above...")
	gologger.GetInstance().SetLogLevel(gologger.LogFilterWarnsAndErrors)
	gologger.LOG_TRACE("TRACE should not appear")
	gologger.LOG_DEBUG("DEBUG should not appear")
	gologger.LOG_INFO("INFO should not appear")
	gologger.LOG_WARN("WARN should appear")
	gologger.LOG_ERROR("ERROR should appear")
	gologger.LOG_FATAL("FATAL should appear")
	gologger.GetInstance().SetLogLevel(gologger.LogFilterAll)

	fmt.Println()
	fmt.Println("Testing statistics...")
	var stats gologger.STStatistics
	gologger.GetInstance().GetStats(&stats)
	fmt.Printf("  Total Lines: %d\n", stats.NTotalLine)
	fmt.Printf("  Total Bytes: %d\n", stats.NTotalByte)

	fmt.Println()
	fmt.Println("Testing layout change to Layout 2...")
	gologger.GetInstance().SetLayout(gologger.LogLayoutTypeBuildin2, nil)
	gologger.LOG_INFO("This message uses Layout 2 format")

	fmt.Println()
	fmt.Println("Testing custom layout...")
	customLayout := gologger.NewCYLoggerTemplateLayoutCustom(
		gologger.GetCYLoggerTemplateLayoutManagerInstance().GetLayout(gologger.LogLayoutTypeBuildin3))
	gologger.GetInstance().SetLayout(gologger.LogLayoutTypeCustom, customLayout)
	gologger.LOG_INFO("This message uses custom layout format")

	gologger.GetInstance().SetLayout(gologger.LogLayoutTypeBuildin1, nil)

	fmt.Println()
	fmt.Println("Testing concurrent logging...")
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				gologger.LOG_INFO("Concurrent log from goroutine %d, iteration %d", id, j)
			}
			done <- true
		}(i)
	}
	for i := 0; i < 5; i++ {
		<-done
	}
	fmt.Println("Concurrent logging test complete")

	fmt.Println()
	fmt.Println("Testing periodic logging...")
	for i := 0; i < 5; i++ {
		gologger.LOG_INFO("Periodic log message %d at %v", i+1, time.Now().Format("15:04:05.000"))
		time.Sleep(200 * time.Millisecond)
	}

	fmt.Println()
	fmt.Println("Flushing and cleaning up...")
	gologger.FlushLogger()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("Received signal, shutting down...")
		gologger.FlushLogger()
		gologger.UnInitLogger()
		os.Exit(0)
	}()

	var finalStats gologger.STStatistics
	gologger.GetInstance().GetStats(&finalStats)
	fmt.Println()
	fmt.Printf("Final Statistics:\n")
	fmt.Printf("  Total Lines: %d\n", finalStats.NTotalLine)
	fmt.Printf("  Total Bytes: %d\n", finalStats.NTotalByte)
	fmt.Printf("  Console Lines: %d\n", finalStats.NConsoleLine)

	gologger.UnInitLogger()
	fmt.Println()
	fmt.Println("=== Console Test Complete ===")
}
