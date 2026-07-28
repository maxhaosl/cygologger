package Appender

import (
	"testing"
	"time"

	Common "github.com/maxhaosl/CYGoLogger/ICYLogger/Common"
	Core "github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
	Layout "github.com/maxhaosl/CYGoLogger/ICYLogger/Layout"
)

// newBenchMessage builds a realistic pooled message for appender benchmarks.
func newBenchMessage(i int) *Common.CYBaseMessage {
	m := Common.AcquireBaseMessage()
	m.EMsgType = int(Core.LogTypeInfo)
	m.NSeverCode = -1
	m.StrMsg = "appender benchmark line with a realistic message body"
	m.StrFile = "bench.go"
	m.StrFunc = "benchFunc"
	m.NLine = i
	m.Time = time.Now()
	m.NProcessId = 4242
	m.NThreadId = 84848
	return m
}

// BenchmarkFileAppenderWrite measures the synchronous file appender write path
// (layout formatting + buffered file I/O + rotation check).
func BenchmarkFileAppenderWrite(b *testing.B) {
	f := GetCYLoggerAppenderFactoryInstance()
	app := f.CreateAppender(Core.LogTypeInfo, "", "", b.TempDir(), Core.LogFileModeTime)
	if app == nil {
		b.Fatal("CreateAppender returned nil")
	}
	app.SetLayout(Layout.GetCYLoggerTemplateLayoutManagerInstance().GetLayout(Core.LogLayoutTypeBuildin1))
	app.SetEnable(true)
	if !app.Init() {
		b.Fatal("appender Init failed")
	}
	defer app.UnInit()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := newBenchMessage(i)
		app.Write(m)
		Common.ReleaseBaseMessage(m)
	}
	b.StopTimer()
	app.Flush()
}

// BenchmarkFileAppenderWriteParallel measures contended synchronous writes to a
// single file appender.
func BenchmarkFileAppenderWriteParallel(b *testing.B) {
	f := GetCYLoggerAppenderFactoryInstance()
	app := f.CreateAppender(Core.LogTypeInfo, "", "", b.TempDir(), Core.LogFileModeTime)
	if app == nil {
		b.Fatal("CreateAppender returned nil")
	}
	app.SetLayout(Layout.GetCYLoggerTemplateLayoutManagerInstance().GetLayout(Core.LogLayoutTypeBuildin1))
	app.SetEnable(true)
	if !app.Init() {
		b.Fatal("appender Init failed")
	}
	defer app.UnInit()

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m := newBenchMessage(i)
			app.Write(m)
			Common.ReleaseBaseMessage(m)
			i++
		}
	})
	b.StopTimer()
	app.Flush()
}
