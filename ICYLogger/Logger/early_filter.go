package logger

import (
	"sync/atomic"

	Core "github.com/maxhaosl/cygologger/ICYLogger/Core"
)

// gLevelFilter is a lock-free mirror of CYLoggerControl.eLogLevel (the active
// level filter). The top-level API functions in api.go consult it via
// LevelEnabled BEFORE paying for runtime.Caller / fmt.Sprintf / message
// construction, so a level suppressed by the filter is dropped at the very entry
// point instead of after the expensive work that control.route() would
// otherwise still perform — the existing in-route filter check runs far too late
// to save that cost (it executes only after apiCallerInfo + Sprintf have already
// run).
//
// It has exactly one extra writer: control.SetLogLevel, the sole mutator of
// eLogLevel, stores it on every change, so the two can never diverge. It is
// initialised to Core.DefaultLogLevelFilter (all levels enabled) so that logging
// performed before Init (when the configured filter has not yet been applied)
// behaves exactly as before: the early filter drops nothing.
var gLevelFilter atomic.Int32

func init() {
	gLevelFilter.Store(int32(Core.DefaultLogLevelFilter))
}

// LevelEnabled reports whether messages at the given level are currently
// permitted by the active level filter. It is the lock-free fast path consulted
// by every filtered API entry point; it returns true when the level bit is set
// in the active filter. A disabled level means the call returns immediately
// without capturing the caller or formatting the message.
func LevelEnabled(level Core.ELogLevel) bool {
	return gLevelFilter.Load()&int32(level) != 0
}
