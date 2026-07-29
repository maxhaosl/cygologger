// Package gologger provides a test for type/level/filter constants.
package main

import (
	"fmt"
	"os"

	gologger "github.com/maxhaosl/cygologger/ICYLogger"
)

func check(name string, got, want any) {
	if got != want {
		fmt.Printf("FAIL: %s: got %v, want %v\n", name, got, want)
		os.Exit(1)
	}
	fmt.Printf("PASS: %s\n", name)
}

func main() {
	fmt.Println("=== cygologger Type Test ===")

	// Test log types
	check("LogTypeTrace", gologger.LogTypeTrace, gologger.ELogType(1))
	check("LogTypeDebug", gologger.LogTypeDebug, gologger.ELogType(2))
	check("LogTypeInfo", gologger.LogTypeInfo, gologger.ELogType(3))
	check("LogTypeWarn", gologger.LogTypeWarn, gologger.ELogType(4))
	check("LogTypeError", gologger.LogTypeError, gologger.ELogType(5))
	check("LogTypeFatal", gologger.LogTypeFatal, gologger.ELogType(6))
	check("LogTypeMain", gologger.LogTypeMain, gologger.ELogType(7))

	// Test log levels
	check("LogLevelTrace", gologger.LogLevelTrace, gologger.ELogLevel(2))
	check("LogLevelDebug", gologger.LogLevelDebug, gologger.ELogLevel(4))
	check("LogLevelInfo", gologger.LogLevelInfo, gologger.ELogLevel(8))
	check("LogLevelWarn", gologger.LogLevelWarn, gologger.ELogLevel(16))
	check("LogLevelError", gologger.LogLevelError, gologger.ELogLevel(32))
	check("LogLevelFatal", gologger.LogLevelFatal, gologger.ELogLevel(64))

	// Test bitwise operations
	combined := gologger.LogLevelInfo | gologger.LogLevelWarn | gologger.LogLevelError
	check("BITWISE_OR", combined, gologger.ELogLevel(56))

	hasInfo := (combined & gologger.LogLevelInfo) != 0
	check("BITWISE_HAS_INFO", hasInfo, true)

	hasDebug := (combined & gologger.LogLevelDebug) != 0
	check("BITWISE_HAS_DEBUG", hasDebug, false)

	// Test filters
	filterAll := gologger.LogFilterAll
	hasTraceInAll := (filterAll & gologger.ELogLevelFilter(gologger.LogLevelTrace)) != 0
	check("FILTER_ALL_HAS_TRACE", hasTraceInAll, true)

	filterErrors := gologger.LogFilterErrors
	hasInfoInErrors := (filterErrors & gologger.ELogLevelFilter(gologger.LogLevelInfo)) != 0
	check("FILTER_ERRORS_HAS_INFO", hasInfoInErrors, false)

	// Test logger init/uninit
	logger := gologger.GetInstance()
	check("INITIAL_STATE", logger.IsInit(), false)

	ok := gologger.InitLogger("./Log", false)
	check("INIT", ok, true)
	check("AFTER_INIT", logger.IsInit(), true)

	// Test stats
	var stats gologger.STStatistics
	ret := logger.GetStats(&stats)
	check("GET_STATS", ret, true)

	// Test level
	oldLevel := logger.GetLogLevel()
	logger.SetLogLevel(gologger.LogFilterErrors)
	check("SET_LEVEL", logger.GetLogLevel(), gologger.LogFilterErrors)
	logger.SetLogLevel(oldLevel)

	// Test flush
	logger.Flush(gologger.LogTypeMax)

	gologger.UnInitLogger()
	check("AFTER_UNINIT", logger.IsInit(), false)

	fmt.Println()
	fmt.Println("=== All Tests Passed ===")
}
