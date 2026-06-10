// simple_test demonstrates basic logging functionality.
package main

import (
	"fmt"
	"os"
	"time"

	gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"
)

func main() {
	fmt.Println("=== CYGoLogger Simple Test ===")
	fmt.Println()

	ok := gologger.InitLogger("./Log", false)
	if !ok {
		fmt.Println("Failed to initialize logger")
		os.Exit(1)
	}
	fmt.Println("Logger initialized")

	gologger.GetInstance().AddAppender(
		gologger.LogTypeInfo, "simple", "Info.log", gologger.LogFileModeAppend)
	gologger.GetInstance().AddAppender(
		gologger.LogTypeError, "simple", "Error.log", gologger.LogFileModeAppend)

	gologger.LOG_INFO("Application started at %v", time.Now().Format("2006-01-02 15:04:05"))
	gologger.LOG_DEBUG("Debug information: version=1.0.0")
	gologger.LOG_WARN("This is a warning message")
	gologger.LOG_ERROR("This is an error message: code=%d", 404)
	gologger.LOG_MAIN("Main application log entry")

	for i := 0; i < 3; i++ {
		gologger.LOG_INFO("Processing item %d", i+1)
		time.Sleep(100 * time.Millisecond)
	}

	var stats gologger.STStatistics
	gologger.GetInstance().GetStats(&stats)
	fmt.Printf("Lines logged: %d\n", stats.NTotalLine)
	fmt.Printf("Bytes logged: %d\n", stats.NTotalByte)

	gologger.FlushLogger()
	gologger.UnInitLogger()

	fmt.Println()
	fmt.Println("=== Simple Test Complete ===")
}
