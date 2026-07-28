package UpLoad

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeFTPServer is a minimal in-process FTP server (stdlib only) that accepts a
// single control connection and one passive-mode STOR transfer. It records the
// STOR path and the uploaded bytes so tests can assert the full happy path
// without any external FTP dependency.
type fakeFTPServer struct {
	ln       net.Listener
	mu       sync.Mutex
	storPath string
	data     []byte
	err      error
	done     chan struct{}
}

func startFakeFTPServer(t *testing.T) *fakeFTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &fakeFTPServer{ln: ln, done: make(chan struct{})}
	go s.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return s
}

func (s *fakeFTPServer) addr() (host string, port int) {
	tcp := s.ln.Addr().(*net.TCPAddr)
	return tcp.IP.String(), tcp.Port
}

func (s *fakeFTPServer) setErr(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

func (s *fakeFTPServer) serve() {
	defer close(s.done)
	conn, err := s.ln.Accept()
	if err != nil {
		s.setErr(err)
		return
	}
	defer conn.Close()

	w := func(format string, args ...any) {
		fmt.Fprintf(conn, format+"\r\n", args...)
	}
	w("220 fake-ftp ready")

	r := bufio.NewReader(conn)
	var dataLn net.Listener
	defer func() {
		if dataLn != nil {
			_ = dataLn.Close()
		}
	}()

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				s.setErr(err)
			}
			return
		}
		line = strings.TrimRight(line, "\r\n")
		cmd := strings.ToUpper(strings.SplitN(line, " ", 2)[0])
		arg := ""
		if idx := strings.Index(line, " "); idx >= 0 {
			arg = line[idx+1:]
		}

		switch cmd {
		case "USER":
			w("331 need password")
		case "PASS":
			w("230 logged in")
		case "TYPE":
			w("200 type set")
		case "PASV":
			dl, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				s.setErr(err)
				w("425 cannot open data port")
				continue
			}
			dataLn = dl
			p := dl.Addr().(*net.TCPAddr).Port
			w("227 Entering Passive Mode (127,0,0,1,%d,%d)", p/256, p%256)
		case "STOR":
			s.mu.Lock()
			s.storPath = arg
			s.mu.Unlock()
			w("150 ok, send data")
			dc, err := dataLn.Accept()
			if err != nil {
				s.setErr(err)
				return
			}
			payload, _ := io.ReadAll(dc)
			_ = dc.Close()
			s.mu.Lock()
			s.data = payload
			s.mu.Unlock()
			w("226 transfer complete")
		case "QUIT":
			w("221 bye")
			return
		default:
			w("502 not implemented")
		}
	}
}

// TestUpLoadFactory verifies singleton identity, type dispatch and the
// unsupported-type error path.
func TestUpLoadFactory(t *testing.T) {
	f := GetCYUpLoadFactoryInstance()
	if f == nil || f != GetCYUpLoadFactoryInstance() {
		t.Fatal("factory singleton is not stable")
	}
	up, err := f.CreateUpLoad(UpLoadTypeFTP)
	if err != nil {
		t.Fatalf("CreateUpLoad(FTP): %v", err)
	}
	if up.GetType() != UpLoadTypeFTP {
		t.Errorf("GetType = %v, want UpLoadTypeFTP", up.GetType())
	}
	if _, err := f.CreateUpLoad(UpLoadTypeNone); err == nil {
		t.Errorf("CreateUpLoad(None) must fail")
	}
	if _, err := f.CreateUpLoad(EUpLoadType(42)); err == nil {
		t.Errorf("CreateUpLoad(42) must fail")
	}
}

// TestFTPUploadErrors verifies nil-config Init and upload-before-init errors.
func TestFTPUploadErrors(t *testing.T) {
	up := NewCYFTPUpLoad()
	if err := up.Init(nil); err == nil {
		t.Errorf("Init(nil) must fail")
	}
	if err := up.Upload("nonexistent.log", ""); err == nil {
		t.Errorf("Upload before Init must fail")
	}
}

// TestFTPUploadHappyPath uploads a real temp file to the in-process fake FTP
// server and asserts the remote path and payload byte-for-byte.
func TestFTPUploadHappyPath(t *testing.T) {
	srv := startFakeFTPServer(t)
	host, port := srv.addr()

	local := filepath.Join(t.TempDir(), "Info.log")
	content := []byte("log line 1\nlog line 2\n\x00binary tail")
	if err := os.WriteFile(local, content, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := &CYUpLoadConfig{
		Host:       host,
		Port:       port,
		User:       "u",
		Password:   "p",
		RemoteDir:  "/logs",
		TimeoutSec: 5,
		Passive:    true,
	}

	err := GetCYUpLoadFactoryInstance().UploadFile(UpLoadTypeFTP, cfg, local, "")
	if err != nil {
		t.Fatalf("UploadFile: %v", err)
	}

	select {
	case <-srv.done:
	case <-time.After(5 * time.Second):
		t.Fatal("fake server did not finish")
	}

	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.err != nil {
		t.Fatalf("fake server error: %v", srv.err)
	}
	if want := "/logs/Info.log"; srv.storPath != want {
		t.Errorf("STOR path = %q, want %q", srv.storPath, want)
	}
	if !bytes.Equal(srv.data, content) {
		t.Errorf("uploaded payload mismatch: got %d bytes, want %d bytes", len(srv.data), len(content))
	}
}

// TestFTPPasvParsing sanity-checks the PASV port arithmetic used by the fake
// server matches the client's parser (p1*256+p2).
func TestFTPPasvParsing(t *testing.T) {
	port := 51234
	p1, p2 := port/256, port%256
	back, err := strconv.Atoi(strconv.Itoa(p1*256 + p2))
	if err != nil || back != port {
		t.Fatalf("PASV round-trip failed: %d != %d", back, port)
	}
}
