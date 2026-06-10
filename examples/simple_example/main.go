// Package gologger provides a simple example demonstrating the type system.
package main

import (
	"fmt"

	gologger "github.com/maxhaosl/CYGoLogger/ICYLogger"
)

func main() {
	fmt.Println("=== CYGoLogger Simple Example ===")
	fmt.Println()

	fmt.Println("Log Types:")
	fmt.Printf("  LogTypeNone   = %d\n", gologger.LogTypeNone)
	fmt.Printf("  LogTypeTrace  = %d\n", gologger.LogTypeTrace)
	fmt.Printf("  LogTypeDebug  = %d\n", gologger.LogTypeDebug)
	fmt.Printf("  LogTypeInfo   = %d\n", gologger.LogTypeInfo)
	fmt.Printf("  LogTypeWarn   = %d\n", gologger.LogTypeWarn)
	fmt.Printf("  LogTypeError  = %d\n", gologger.LogTypeError)
	fmt.Printf("  LogTypeFatal  = %d\n", gologger.LogTypeFatal)
	fmt.Printf("  LogTypeMain   = %d\n", gologger.LogTypeMain)
	fmt.Printf("  LogTypeRemote = %d\n", gologger.LogTypeRemote)
	fmt.Printf("  LogTypeSys    = %d\n", gologger.LogTypeSys)
	fmt.Printf("  LogTypeMax    = %d\n", gologger.LogTypeMax)
	fmt.Println()

	fmt.Println("Log Levels:")
	fmt.Printf("  LogLevelConsole = %d\n", gologger.LogLevelConsole)
	fmt.Printf("  LogLevelTrace   = %d\n", gologger.LogLevelTrace)
	fmt.Printf("  LogLevelDebug   = %d\n", gologger.LogLevelDebug)
	fmt.Printf("  LogLevelInfo    = %d\n", gologger.LogLevelInfo)
	fmt.Printf("  LogLevelWarn    = %d\n", gologger.LogLevelWarn)
	fmt.Printf("  LogLevelError   = %d\n", gologger.LogLevelError)
	fmt.Printf("  LogLevelFatal   = %d\n", gologger.LogLevelFatal)
	fmt.Printf("  LogLevelRemote  = %d\n", gologger.LogLevelRemote)
	fmt.Printf("  LogLevelSys     = %d\n", gologger.LogLevelSys)
	fmt.Println()

	fmt.Println("Log Level Filters:")
	fmt.Printf("  LogFilterAll            = %d\n", gologger.LogFilterAll)
	fmt.Printf("  LogFilterWarnsAndErrors = %d\n", gologger.LogFilterWarnsAndErrors)
	fmt.Printf("  LogFilterErrors         = %d\n", gologger.LogFilterErrors)
	fmt.Printf("  LogFilterNone           = %d\n", gologger.LogFilterNone)
	fmt.Println()

	fmt.Println("File Modes:")
	fmt.Printf("  LogFileModeAppend = %d\n", gologger.LogFileModeAppend)
	fmt.Printf("  LogFileModeTime   = %d\n", gologger.LogFileModeTime)
	fmt.Println()

	fmt.Println("Layout Types:")
	fmt.Printf("  LogLayoutTypeCustom   = %d\n", gologger.LogLayoutTypeCustom)
	fmt.Printf("  LogLayoutTypeBuildin1 = %d\n", gologger.LogLayoutTypeBuildin1)
	fmt.Printf("  LogLayoutTypeBuildin2 = %d\n", gologger.LogLayoutTypeBuildin2)
	fmt.Printf("  LogLayoutTypeBuildin3 = %d\n", gologger.LogLayoutTypeBuildin3)
	fmt.Println()

	fmt.Println("=== Example Complete ===")
}
