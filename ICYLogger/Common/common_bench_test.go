package Common

import (
	"testing"
	"time"
)

// BenchmarkMessagePoolAcquireRelease measures the pooled message churn cost
// (the hot allocation path of every write).
func BenchmarkMessagePoolAcquireRelease(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := AcquireBaseMessage()
		m.EMsgType = 3
		m.StrMsg = "pooled message body"
		m.StrFile = "bench.go"
		m.NLine = i
		m.Time = time.Time{}
		ReleaseBaseMessage(m)
	}
}

// BenchmarkMessagePoolParallel measures pool contention across goroutines.
func BenchmarkMessagePoolParallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m := AcquireBaseMessage()
			m.StrMsg = "parallel pooled message"
			ReleaseBaseMessage(m)
		}
	})
}

// BenchmarkMessageClone measures the async-appender clone path.
func BenchmarkMessageClone(b *testing.B) {
	src := AcquireBaseMessage()
	src.EMsgType = 3
	src.StrChannel = "Chan"
	src.StrMsg = "message body to clone into the private queue"
	src.StrFile = "bench.go"
	src.StrFunc = "benchFunc"
	src.NLine = 42
	defer ReleaseBaseMessage(src)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := src.Clone()
		ReleaseBaseMessage(c)
	}
}

// BenchmarkFPSCounterIncrement measures the per-message statistics cost.
func BenchmarkFPSCounterIncrement(b *testing.B) {
	c := NewCYFPSCounter(1000)
	c.Start()
	defer c.Stop()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Increment()
	}
}

// BenchmarkFPSCounterIncrementParallel measures contended statistics updates.
func BenchmarkFPSCounterIncrementParallel(b *testing.B) {
	c := NewCYFPSCounter(1000)
	c.Start()
	defer c.Stop()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Increment()
		}
	})
}
