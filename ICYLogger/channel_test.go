package ICYLogger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestChannelRendering verifies that the channel-aware logging functions carry a
// per-message channel that is rendered by the layout (the Channel: field),
// mirroring the C++ ICYLogger::WriteLog(szChannel, ...) behaviour.
func TestChannelRendering(t *testing.T) {
	dir, err := os.MkdirTemp("", "cygologger-channel-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)

	if !InitDefault(dir) {
		t.Fatalf("InitDefault failed")
	}
	defer Close()

	const ch = "MyChannel"
	InfoCh(ch, "hello %s", "world")
	// Also exercise the LOG_*_CH legacy form and the direct variant.
	LOG_WARN_CH(ch, "legacy warn on %s", ch)
	DebugCh(ch, "debug line")

	Flush()

	// The default layout 1 renders "Channel:<ch>" for a non-empty channel.
	matches, _ := filepath.Glob(filepath.Join(dir, "Info_*.log"))
	if len(matches) == 0 {
		t.Fatalf("no Info log file produced in %s", dir)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Channel:"+ch) {
		t.Fatalf("expected channel %q to be rendered, got:\n%s", ch, content)
	}
	if !strings.Contains(content, "hello world") {
		t.Fatalf("expected message body to be rendered, got:\n%s", content)
	}
}
