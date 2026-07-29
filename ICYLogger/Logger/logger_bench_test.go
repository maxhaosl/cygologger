package logger

import (
	"testing"

	"github.com/maxhaosl/cygologger/ICYLogger/Core"
)

// benchInit initializes the logger once per benchmark into a temp dir with the
// console disabled, so throughput numbers reflect the file pipeline only.
func benchInit(b *testing.B) {
	b.Helper()
	initLoggerAt(b, b.TempDir())
}

// BenchmarkWriteLogFmt measures single-goroutine formatted write throughput
// (full pipeline: level filter, message pool, layout, file write, main mirror).
func BenchmarkWriteLogFmt(b *testing.B) {
	benchInit(b)
	l := GetInstance()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
			"bench.go", "benchFunc", i, "benchmark line %d value=%s", i, "payload")
	}
	b.StopTimer()
	FlushLogger()
}

// BenchmarkWriteLogFmtParallel measures concurrent write throughput across
// GOMAXPROCS goroutines (contention on the control RWMutex + file appender).
func BenchmarkWriteLogFmtParallel(b *testing.B) {
	benchInit(b)
	l := GetInstance()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			l.WriteLogFmt(int(Core.LogLevelInfo), Core.LogTypeInfo, -1,
				"bench.go", "benchFunc", i, "parallel line %d", i)
			i++
		}
	})
	b.StopTimer()
	FlushLogger()
}

// BenchmarkWriteLogNoFormat measures the raw (pre-formatted) write path.
func BenchmarkWriteLogNoFormat(b *testing.B) {
	benchInit(b)
	l := GetInstance()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.WriteLog(int(Core.LogLevelInfo), Core.LogTypeInfo, -1, "pre-formatted benchmark line")
	}
	b.StopTimer()
	FlushLogger()
}

// BenchmarkWriteEscapeLogFmt measures the escape-formatted write path.
func BenchmarkWriteEscapeLogFmt(b *testing.B) {
	benchInit(b)
	l := GetInstance()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.WriteEscapeLogFmt(int(Core.LogLevelWarn), Core.LogTypeWarn, -1,
			"bench.go", "benchFunc", i, "escape [%d] line with = and | chars", i)
	}
	b.StopTimer()
	FlushLogger()
}

// BenchmarkWriteHexLog measures the hex-dump write path (64-byte payload).
func BenchmarkWriteHexLog(b *testing.B) {
	benchInit(b)
	l := GetInstance()
	data := make([]byte, 64)
	for i := range data {
		data[i] = byte(i)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.WriteHexLog(int(Core.LogLevelDebug), Core.LogTypeDebug, -1,
			"bench.go", "benchFunc", i, data)
	}
	b.StopTimer()
	FlushLogger()
}

// BenchmarkFormatHex isolates the hex formatting cost (no I/O).
func BenchmarkFormatHex(b *testing.B) {
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = formatHex(data)
	}
}

// BenchmarkFilteredWrite measures the cost of a write rejected by the level
// filter (the cheap path a production system relies on).
func BenchmarkFilteredWrite(b *testing.B) {
	benchInit(b)
	l := GetInstance()
	GetLoggerInstance().SetLogLevel(Core.LogFilterErrors)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.WriteLogFmt(int(Core.LogLevelDebug), Core.LogTypeDebug, -1,
			"bench.go", "benchFunc", i, "this line is filtered out %d", i)
	}
}
