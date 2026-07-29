/*
 * cygologger License
 * -----------
 *
 * cygologger is licensed under the terms of the MIT license reproduced below.
 * This means that cygologger is free software and can be used for both academic
 * and commercial purposes at absolutely no cost.
 *
 * ===============================================================================
 *
 * Copyright (C) 2023-2024 ShiLiang.Hao <newhaosl@163.com>, foobra<vipgs99@gmail.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

package UpLoad

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CYFTPUpLoad is a minimal FTP client uploader built entirely on the Go
// standard library (no third-party dependencies). It supports passive-mode
// binary STOR uploads, which covers the "archive log then upload" use case.
type CYFTPUpLoad struct {
	CYBaseUpLoad
	conn    net.Conn
	ctrl    *bufio.Reader
	timeout time.Duration
}

// NewCYFTPUpLoad creates a new FTP uploader.
func NewCYFTPUpLoad() *CYFTPUpLoad {
	f := &CYFTPUpLoad{}
	f.upType = UpLoadTypeFTP
	return f
}

// Init connects to the FTP server and authenticates using cfg.
func (f *CYFTPUpLoad) Init(cfg *CYUpLoadConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if cfg == nil {
		return fmt.Errorf("ftp: nil config")
	}
	f.cfg = cfg
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	f.timeout = timeout

	port := cfg.Port
	if port == 0 {
		port = 21
	}
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(port))

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("ftp: dial %s failed: %w", addr, err)
	}
	f.conn = conn
	f.ctrl = bufio.NewReader(conn)

	// Read server greeting (220).
	if _, err := f.readResponse(220); err != nil {
		f.closeConn()
		return err
	}

	// USER
	if _, err := f.cmd(331, "USER %s", cfg.User); err != nil {
		// Some servers reply 230 directly if no password needed.
		f.closeConn()
		return err
	}
	// PASS
	if _, err := f.cmd(230, "PASS %s", cfg.Password); err != nil {
		f.closeConn()
		return err
	}
	// Binary mode.
	if _, err := f.cmd(200, "TYPE I"); err != nil {
		f.closeConn()
		return err
	}

	f.inited = true
	return nil
}

// Upload transfers localPath to the FTP server. If remotePath is empty, the
// base name of localPath is placed under cfg.RemoteDir.
func (f *CYFTPUpLoad) Upload(localPath, remotePath string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.inited || f.conn == nil {
		return fmt.Errorf("ftp: not initialized")
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("ftp: open local %s failed: %w", localPath, err)
	}
	defer file.Close()

	if remotePath == "" {
		remotePath = filepath.Base(localPath)
		if f.cfg.RemoteDir != "" {
			remotePath = path.Join(f.cfg.RemoteDir, remotePath)
		}
	}

	// Enter passive mode and obtain the data connection address.
	dataAddr, err := f.pasv()
	if err != nil {
		return err
	}

	dataConn, err := net.DialTimeout("tcp", dataAddr, f.timeout)
	if err != nil {
		return fmt.Errorf("ftp: data dial %s failed: %w", dataAddr, err)
	}
	defer dataConn.Close()

	// Issue STOR on the control channel (expect 150/125).
	if _, err := f.cmd(150, "STOR %s", remotePath); err != nil {
		return err
	}

	// Transfer file bytes over the data connection.
	if _, err := io.Copy(dataConn, file); err != nil {
		return fmt.Errorf("ftp: data transfer failed: %w", err)
	}
	// Close the data connection to signal EOF, then read transfer-complete (226).
	_ = dataConn.Close()
	if _, err := f.readResponse(226); err != nil {
		return err
	}
	return nil
}

// UnInit sends QUIT and closes the control connection.
func (f *CYFTPUpLoad) UnInit() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn != nil {
		_, _ = f.cmdNoLock(221, "QUIT")
		f.closeConn()
	}
	f.inited = false
	return nil
}

// ---- internal helpers ----

func (f *CYFTPUpLoad) closeConn() {
	if f.conn != nil {
		_ = f.conn.Close()
		f.conn = nil
	}
	f.ctrl = nil
}

// pasv enters passive mode and returns the "host:port" for the data connection.
func (f *CYFTPUpLoad) pasv() (string, error) {
	line, err := f.cmd(227, "PASV")
	if err != nil {
		return "", err
	}
	// Response format: 227 Entering Passive Mode (h1,h2,h3,h4,p1,p2).
	start := strings.Index(line, "(")
	end := strings.Index(line, ")")
	if start < 0 || end < 0 || end <= start {
		return "", fmt.Errorf("ftp: malformed PASV response: %s", line)
	}
	parts := strings.Split(line[start+1:end], ",")
	if len(parts) != 6 {
		return "", fmt.Errorf("ftp: malformed PASV numbers: %s", line)
	}
	host := strings.Join(parts[0:4], ".")
	p1, err1 := strconv.Atoi(strings.TrimSpace(parts[4]))
	p2, err2 := strconv.Atoi(strings.TrimSpace(parts[5]))
	if err1 != nil || err2 != nil {
		return "", fmt.Errorf("ftp: invalid PASV port: %s", line)
	}
	port := p1*256 + p2
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

// cmd sends a formatted command and validates the response code (with lock held).
func (f *CYFTPUpLoad) cmd(expectCode int, format string, args ...any) (string, error) {
	return f.cmdNoLock(expectCode, format, args...)
}

// cmdNoLock writes a command and reads/validates the response.
func (f *CYFTPUpLoad) cmdNoLock(expectCode int, format string, args ...any) (string, error) {
	if f.conn == nil {
		return "", fmt.Errorf("ftp: connection closed")
	}
	command := fmt.Sprintf(format, args...)
	_ = f.conn.SetDeadline(time.Now().Add(f.timeout))
	if _, err := fmt.Fprintf(f.conn, "%s\r\n", command); err != nil {
		return "", fmt.Errorf("ftp: send %q failed: %w", command, err)
	}
	return f.readResponse(expectCode)
}

// readResponse reads a (possibly multi-line) FTP response and checks its code.
func (f *CYFTPUpLoad) readResponse(expectCode int) (string, error) {
	if f.ctrl == nil {
		return "", fmt.Errorf("ftp: no control reader")
	}
	_ = f.conn.SetDeadline(time.Now().Add(f.timeout))

	first, err := f.ctrl.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("ftp: read response failed: %w", err)
	}
	first = strings.TrimRight(first, "\r\n")
	if len(first) < 4 {
		return "", fmt.Errorf("ftp: short response: %q", first)
	}
	code := first[:3]

	// Multi-line response: "123-...." continues until a line "123 ....".
	if first[3] == '-' {
		for {
			line, err := f.ctrl.ReadString('\n')
			if err != nil {
				return "", fmt.Errorf("ftp: read multiline failed: %w", err)
			}
			line = strings.TrimRight(line, "\r\n")
			if len(line) >= 4 && line[:3] == code && line[3] == ' ' {
				break
			}
		}
	}

	n, convErr := strconv.Atoi(code)
	if convErr != nil {
		return "", fmt.Errorf("ftp: invalid response code: %q", first)
	}
	// Accept the exact expected code, or any 2xx/1xx that matches the family
	// leader (e.g. USER may reply 230 instead of 331 when no password needed).
	if n != expectCode {
		// 1xx and 2xx are non-error; treat mismatch as error only for 4xx/5xx.
		if n >= 400 {
			return first, fmt.Errorf("ftp: unexpected response (want %d): %s", expectCode, first)
		}
	}
	return first, nil
}

// Compile-time interface check.
var _ IUpLoad = (*CYFTPUpLoad)(nil)
