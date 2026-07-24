package Schedule

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDoZipLog(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "Info_20240101_120000.log")
	content := []byte("line1\nline2\nline3\n")
	if err := os.WriteFile(logPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(dir, "Info.zip")

	z := NewCYLoggerDoZipLog()
	if !z.DoZipLog(logPath, zipPath) {
		t.Fatal("DoZipLog returned false")
	}

	// Verify the zip contains the exact original content.
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	if len(zr.File) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(zr.File))
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	buf := make([]byte, len(content))
	n, _ := rc.Read(buf)
	if string(buf[:n]) != string(content) {
		t.Fatalf("zip content mismatch: %q", buf[:n])
	}
}

func TestClearLogFileRotation(t *testing.T) {
	dir := t.TempDir()

	// Create 25 files of type "Info" (default limit is 20) plus one "Main".
	for i := 0; i < 25; i++ {
		p := filepath.Join(dir, "Info_"+time.Now().Add(time.Duration(i)*time.Millisecond).Format("150405.000")+".log")
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	mainPath := filepath.Join(dir, "Main_120000.log")
	if err := os.WriteFile(mainPath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	cl := NewCYLoggerClearLogFile(dir, 24)
	cl.SetRestriction(20, 1024*1024*500, 1024*1024*1024)
	cl.DoClear()

	remaining := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(path) == ".log" {
			remaining++
		}
		return nil
	})
	if remaining != 21 { // 20 Info + 1 Main
		t.Fatalf("expected 21 files after rotation (20 Info + 1 Main), got %d", remaining)
	}
}

func TestClearLogFileGlobalSize(t *testing.T) {
	dir := t.TempDir()
	// 5 files of 100 bytes each => 500 bytes total; global limit 200 => keep oldest 2.
	for i := 0; i < 5; i++ {
		p := filepath.Join(dir, "Info_"+time.Now().Add(time.Duration(i)*time.Millisecond).Format("150405.000")+".log")
		if err := os.WriteFile(p, []byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cl := NewCYLoggerClearLogFile(dir, 24)
	cl.SetRestriction(100, 1024*1024*500, 200)
	cl.DoClear()

	remaining := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(path) == ".log" {
			remaining++
		}
		return nil
	})
	if remaining != 2 {
		t.Fatalf("expected 2 files after global size limit, got %d", remaining)
	}
}
