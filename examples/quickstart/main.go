// Command quickstart demonstrates the one-line initialization of cygologger.
//
// A single call to gologger.InitDefault("./logs") automatically mounts a full
// set of file appenders (Trace/Debug/Info/Warn/Error/Fatal/Main) plus the
// console appender — mirroring the C++ CY_LOG_CONFIG macro. No manual
// AddAppender calls are required.
package main

import (
	"fmt"

	gologger "github.com/maxhaosl/cygologger/ICYLogger"
)

func main() {
	// ONE line: auto-mounts console + all file appenders under ./logs.
	gologger.InitDefault("./logs")
	defer gologger.Close()

	// Done. Just log. Caller file:line is captured automatically.
	gologger.Info("Application started: v%s", "1.0.0")
	gologger.Debug("Debug info: count=%d", 10)
	gologger.Warn("This is a warning")
	gologger.Error("Error occurred: code=%d", 500)

	// Extra outputs, still one-line-init:
	gologger.HexInfo([]byte{0x48, 0x65, 0x6c, 0x6c, 0x6f})
	gologger.EscapeInfo("Message with [brackets] and ]more[")

	fmt.Println("see ./logs for Trace/Debug/Info/Warn/Error/Fatal/Main.log")
}
