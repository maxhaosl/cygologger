/*
 * cygologger License ...
 */

package Layout

import (
	"strings"
	"testing"

	"github.com/maxhaosl/cygologger/ICYLogger/Core"
)

// TestLayout4Resolution verifies the new built-in layout 4 resolves from the
// manager and produces the expected bracketed format (parity with C++ Layout4).
func TestLayout4Resolution(t *testing.T) {
	m := GetCYLoggerTemplateLayoutManagerInstance()
	l := m.GetLayout(Core.LogLayoutTypeBuildin4)
	if l == nil {
		t.Fatal("GetLayout(Buildin4) returned nil")
	}
	if l.GetTypeIndex() != 4 {
		t.Fatalf("expected type index 4, got %d", l.GetTypeIndex())
	}

	// bEscape=false so the escape-filter branch is not exercised here.
	out := l.GetFormatMessage(
		"", Core.LogTypeInfo, -1,
		"hello world", "main.go", "main", 42,
		1234, 5678,
		2026, 7, 28, 11, 4, 21, 564,
		false,
	)

	if !strings.HasPrefix(out, "[2026-07-28 11:04:21.564]") {
		t.Fatalf("unexpected time prefix: %q", out)
	}
	if !strings.Contains(out, "[I|P:1234|T:5678]") {
		t.Fatalf("missing type/pid/tid block: %q", out)
	}
	if !strings.Contains(out, "[main(42)]") {
		t.Fatalf("missing func(line) block: %q", out)
	}
	if !strings.HasSuffix(out, "hello world\n") {
		t.Fatalf("unexpected message suffix: %q", out)
	}
}
