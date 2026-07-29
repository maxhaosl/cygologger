package Layout

import (
	"testing"

	"github.com/maxhaosl/cygologger/ICYLogger/Core"
)

// benchLayout runs GetFormatMessage for the given layout type with a typical
// message shape, reporting allocations per formatted line.
func benchLayout(b *testing.B, eType Core.ELogLayoutType) {
	b.Helper()
	layout := GetCYLoggerTemplateLayoutManagerInstance().GetLayout(eType)
	if layout == nil {
		b.Fatalf("no layout for type %v", eType)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = layout.GetFormatMessage("BenchChan", Core.LogTypeInfo, -1,
			"benchmark message body with some realistic length for formatting",
			"bench.go", "benchFunc", 128, 4242, 84848,
			2026, 7, 28, 12, 34, 56, 789, false)
	}
}

func BenchmarkLayout1Format(b *testing.B) { benchLayout(b, Core.LogLayoutTypeBuildin1) }
func BenchmarkLayout2Format(b *testing.B) { benchLayout(b, Core.LogLayoutTypeBuildin2) }
func BenchmarkLayout3Format(b *testing.B) { benchLayout(b, Core.LogLayoutTypeBuildin3) }
func BenchmarkLayout4Format(b *testing.B) { benchLayout(b, Core.LogLayoutTypeBuildin4) }

// BenchmarkLayout1FormatEscaped measures the escape-enabled formatting path.
func BenchmarkLayout1FormatEscaped(b *testing.B) {
	layout := GetCYLoggerTemplateLayoutManagerInstance().GetLayout(Core.LogLayoutTypeBuildin1)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = layout.GetFormatMessage("BenchChan", Core.LogTypeWarn, -1,
			"message with [brackets] = equals | pipes # hashes to escape",
			"bench.go", "benchFunc", 128, 4242, 84848,
			2026, 7, 28, 12, 34, 56, 789, true)
	}
}

// BenchmarkLayoutTimeStamps isolates the timestamp rendering cost.
func BenchmarkLayoutTimeStamps(b *testing.B) {
	layout := GetCYLoggerTemplateLayoutManagerInstance().GetLayout(Core.LogLayoutTypeBuildin1)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = layout.GetTimeStamps(2026, 7, 28, 12, 34, 56, 789)
	}
}
