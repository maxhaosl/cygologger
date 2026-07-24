/*
 * CYGoLogger License
 * -----------
 *
 * CYGoLogger is licensed under the terms of the MIT license reproduced below.
 * This means that CYGoLogger is free software and can be used for both academic
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

// Package UpLoad provides log-file upload capabilities (Go port of the C++
// CYBaseUpLoad / CYFTPUpLoad / CYUpLoadFactory modules). It defines a generic
// IUpLoad interface, a factory to create uploaders by type, and a pure
// standard-library FTP implementation (no third-party dependencies).
package UpLoad

import (
	"fmt"
	"sync"
)

// EUpLoadType enumerates the supported uploader implementations.
type EUpLoadType int

const (
	UpLoadTypeNone EUpLoadType = iota
	UpLoadTypeFTP
)

// CYUpLoadConfig holds connection/authentication settings for an uploader.
type CYUpLoadConfig struct {
	Host       string // server host, e.g. "127.0.0.1"
	Port       int    // server port, e.g. 21 for FTP
	User       string // username
	Password   string // password
	RemoteDir  string // remote base directory for uploads
	TimeoutSec int    // dial/IO timeout in seconds (0 => default 30)
	Passive    bool   // FTP passive mode (recommended true)
}

// IUpLoad is the generic uploader interface. All concrete uploaders
// (FTP, and future SFTP/HTTP) implement it.
type IUpLoad interface {
	// Init establishes/prepares the uploader with the given configuration.
	Init(cfg *CYUpLoadConfig) error
	// Upload transfers the local file at localPath to remotePath on the server.
	// remotePath may be empty, in which case the base name of localPath is used
	// under the configured RemoteDir.
	Upload(localPath, remotePath string) error
	// UnInit releases any resources / closes connections held by the uploader.
	UnInit() error
	// GetType returns the uploader implementation type.
	GetType() EUpLoadType
}

// CYBaseUpLoad provides shared state for concrete uploaders (Go port of C++ CYBaseUpLoad).
type CYBaseUpLoad struct {
	mu       sync.Mutex
	cfg      *CYUpLoadConfig
	upType   EUpLoadType
	inited   bool
}

func (b *CYBaseUpLoad) GetType() EUpLoadType { return b.upType }

func (b *CYBaseUpLoad) config() *CYUpLoadConfig { return b.cfg }

// ============================================================================
// CYUpLoadFactory - creates uploaders by type (Go port of C++ CYUpLoadFactory)
// ============================================================================

// CYUpLoadFactory is a singleton factory producing IUpLoad implementations.
type CYUpLoadFactory struct {
	mu sync.Mutex
}

var (
	g_CYUpLoadFactoryInstance *CYUpLoadFactory
	g_CYUpLoadFactoryOnce     sync.Once
)

// GetCYUpLoadFactoryInstance returns the singleton upload factory.
func GetCYUpLoadFactoryInstance() *CYUpLoadFactory {
	g_CYUpLoadFactoryOnce.Do(func() {
		g_CYUpLoadFactoryInstance = &CYUpLoadFactory{}
	})
	return g_CYUpLoadFactoryInstance
}

// CreateUpLoad constructs a new uploader for the given type.
func (f *CYUpLoadFactory) CreateUpLoad(eType EUpLoadType) (IUpLoad, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch eType {
	case UpLoadTypeFTP:
		return NewCYFTPUpLoad(), nil
	default:
		return nil, fmt.Errorf("unsupported upload type: %d", eType)
	}
}

// UploadFile is a convenience one-shot helper: it creates an uploader of the
// given type, initializes it with cfg, uploads localPath -> remotePath, then
// cleans up. Ideal for "archive log then upload" flows.
func (f *CYUpLoadFactory) UploadFile(eType EUpLoadType, cfg *CYUpLoadConfig, localPath, remotePath string) error {
	up, err := f.CreateUpLoad(eType)
	if err != nil {
		return err
	}
	if err := up.Init(cfg); err != nil {
		return err
	}
	defer up.UnInit()
	return up.Upload(localPath, remotePath)
}
