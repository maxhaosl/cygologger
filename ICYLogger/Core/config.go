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

package Core

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// CYLoggerConfig manages global logger configuration.
// This is the unified configuration point, replacing the former Config package.
type CYLoggerConfig struct {
	mu              sync.RWMutex
	szLogPath       string
	szErrorLogPath  string
	bShowConsole    bool
	bWriteRemote    bool
	bWriteSys       bool
	eFileMode       ELogFileMode
	eLayoutType     ELogLayoutType
	eLogLevelFilter ELogLevelFilter
	bMountMain      bool // 是否挂载 Main 聚合文件（默认 true，向后兼容旧行为）
	// bWithThreadId controls whether each log line records the goroutine ID
	// (the T: field). Obtaining the goroutine ID requires runtime.Stack, whose
	// internal runtime lock serialises ALL logging goroutines under high
	// concurrency (CPU profiles showed >90% of logging CPU inside
	// runtime.Stack). Default true for backward compatibility; latency/
	// throughput-sensitive services should set WithThreadId(false), matching
	// industry practice (zap/zerolog do not record goroutine IDs by default).
	bWithThreadId bool

	// szRemoteAddr is the remote log server address (host:port). It mirrors the
	// C++ CY_LOG_APPENDER remote endpoint (default 127.0.0.1:7000).
	szRemoteAddr string
	// eRemoteProto selects the remote transport (TCP by default; UDP to align
	// with the C++ wire format).
	eRemoteProto ERemoteProto
	// nClearPeriodSec is the background cleanup/rotation check period in seconds.
	nClearPeriodSec int

	// Rotation / restriction policy. Mirrors the 10 parameters of the C++
	// CYLoggerImpl::SetRestriction and is applied to the runtime CYFileRestriction
	// during Init.
	bLimitEnable          bool
	bClearUnLogFile       bool
	nLimitTimeClearLog    int
	nLimitTimeExpiredFile int
	nCheckFileSizeTime    int
	nCheckFileCountTime   int
	nCheckFileSize        int
	nCountPerType         int
	nCheckFileTypeSize    int
	nCheckAllFileSize     int
}

var g_CYLoggerConfigInstance *CYLoggerConfig
var g_CYLoggerConfigOnce sync.Once

// GetCYLoggerConfigInstance returns the singleton logger configuration.
func GetCYLoggerConfigInstance() *CYLoggerConfig {
	g_CYLoggerConfigOnce.Do(func() {
		g_CYLoggerConfigInstance = &CYLoggerConfig{
			szLogPath:       "",
			szErrorLogPath:  "Error.log",
			bShowConsole:    DefaultLogShowConsoleWindow,
			bWriteRemote:    DefaultLogWriteRemote,
			bWriteSys:       DefaultLogWriteSys,
			eFileMode:       DefaultLogFileMode,
			eLayoutType:     DefaultLogLayoutType,
			eLogLevelFilter: DefaultLogLevelFilter,
			bMountMain:      LOG_MOUNT_MAIN,
			bWithThreadId:   LOG_WITH_THREAD_ID,

			szRemoteAddr:          "127.0.0.1:7000",
			eRemoteProto:          RemoteProtoTCP,
			nClearPeriodSec:       60,
			bLimitEnable:          DefaultLogLimitEnable,
			bClearUnLogFile:       DefaultLogLimitClearUnLogFile,
			nLimitTimeClearLog:    DefaultLogTimeClearLog,
			nLimitTimeExpiredFile: DefaultLogTimeExpiredFile,
			nCheckFileSizeTime:    DefaultLogCheckFileSizeTime,
			nCheckFileCountTime:   DefaultLogCheckFileCountTime,
			nCheckFileSize:        DefaultLogCheckFileSize,
			nCountPerType:         DefaultLogCountPerType,
			nCheckFileTypeSize:    DefaultLogCheckFileTypeSize,
			nCheckAllFileSize:     DefaultLogCheckAllFileSize,
		}
	})
	return g_CYLoggerConfigInstance
}

// SetLogPath sets the log file output directory. An empty path falls back to
// the current working directory, mirroring the C++ CYLoggerConfig behaviour.
func (c *CYLoggerConfig) SetLogPath(szPath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if szPath == "" {
		if wd, err := os.Getwd(); err == nil {
			szPath = wd
		} else {
			szPath = "."
		}
	}
	c.szLogPath = filepath.Clean(szPath)
}

// GetLogPath returns the log file output directory.
func (c *CYLoggerConfig) GetLogPath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.szLogPath
}

// SetErrorLogPath sets the dedicated error log file path.
func (c *CYLoggerConfig) SetErrorLogPath(szPath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.szErrorLogPath = szPath
}

// GetErrorLogPath returns the dedicated error log file path. If the configured
// value is a bare file name (no directory), it is resolved against the log path.
func (c *CYLoggerConfig) GetErrorLogPath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p := c.szErrorLogPath
	if p == "" || filepath.IsAbs(p) || strings.ContainsAny(p, "/\\") {
		return p
	}
	return filepath.Join(c.szLogPath, p)
}

// SetShowConsole enables or disables console output.
func (c *CYLoggerConfig) SetShowConsole(bShow bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bShowConsole = bShow
}

// IsShowConsole returns whether console output is enabled.
func (c *CYLoggerConfig) IsShowConsole() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bShowConsole
}

// SetWriteRemote enables or disables remote log writing.
func (c *CYLoggerConfig) SetWriteRemote(bWrite bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bWriteRemote = bWrite
}

// IsWriteRemote returns whether remote log writing is enabled.
func (c *CYLoggerConfig) IsWriteRemote() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bWriteRemote
}

// SetWriteSys enables or disables system event log writing.
func (c *CYLoggerConfig) SetWriteSys(bWrite bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bWriteSys = bWrite
}

// IsWriteSys returns whether system event log writing is enabled.
func (c *CYLoggerConfig) IsWriteSys() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bWriteSys
}

// SetFileMode sets the log file naming mode (append or time-based).
func (c *CYLoggerConfig) SetFileMode(eMode ELogFileMode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eFileMode = eMode
}

// GetFileMode returns the log file naming mode.
func (c *CYLoggerConfig) GetFileMode() ELogFileMode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.eFileMode
}

// SetLayoutType sets the default log layout template type.
func (c *CYLoggerConfig) SetLayoutType(eType ELogLayoutType) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eLayoutType = eType
}

// GetLayoutType returns the default log layout template type.
func (c *CYLoggerConfig) GetLayoutType() ELogLayoutType {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.eLayoutType
}

// SetLogLevelFilter sets the log level filter (bitmask).
func (c *CYLoggerConfig) SetLogLevelFilter(eFilter ELogLevelFilter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eLogLevelFilter = eFilter
}

// GetLogLevelFilter returns the log level filter (bitmask).
func (c *CYLoggerConfig) GetLogLevelFilter() ELogLevelFilter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.eLogLevelFilter
}

// SetMountMain enables or disables the Main aggregate file appender. When
// enabled (the default), Init mounts a Main.log that aggregates every enabled
// log type. When disabled, only the per-level files are created, so callers
// can keep a strict per-level file set (e.g. Trace/Info/Warn/Error without a
// Main.log). The Main entity still exists in the control map, so aggregated
// writes become no-ops and no Main file is produced.
func (c *CYLoggerConfig) SetMountMain(b bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bMountMain = b
}

// IsMountMain returns whether the Main aggregate file appender is mounted.
func (c *CYLoggerConfig) IsMountMain() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bMountMain
}

// SetWithThreadId enables/disables recording the goroutine ID (T: field) on
// every log line. See the bWithThreadId field comment for the rationale.
func (c *CYLoggerConfig) SetWithThreadId(b bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bWithThreadId = b
}

// IsWithThreadId returns whether log lines record the goroutine ID.
func (c *CYLoggerConfig) IsWithThreadId() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bWithThreadId
}

// =============================================================================
// Remote address & cleanup period
// =============================================================================

// SetRemoteAddr sets the remote log server address (host:port).
func (c *CYLoggerConfig) SetRemoteAddr(szAddr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if szAddr != "" {
		c.szRemoteAddr = szAddr
	}
}

// GetRemoteAddr returns the remote log server address.
func (c *CYLoggerConfig) GetRemoteAddr() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.szRemoteAddr
}

// SetRemoteProto selects the remote transport protocol (TCP or UDP).
func (c *CYLoggerConfig) SetRemoteProto(eProto ERemoteProto) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eRemoteProto = eProto
}

// GetRemoteProto returns the remote transport protocol.
func (c *CYLoggerConfig) GetRemoteProto() ERemoteProto {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.eRemoteProto
}

// SetClearPeriodSec sets the background cleanup/rotation check period in seconds.
func (c *CYLoggerConfig) SetClearPeriodSec(nSec int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if nSec > 0 {
		c.nClearPeriodSec = nSec
	}
}

// GetClearPeriodSec returns the cleanup/rotation check period in seconds.
func (c *CYLoggerConfig) GetClearPeriodSec() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nClearPeriodSec
}

// =============================================================================
// Restriction / rotation policy configuration
// =============================================================================

// SetRestriction configures the file rotation policy. It mirrors the 10
// parameters of the C++ CYLoggerImpl::SetRestriction and is applied to the
// runtime CYFileRestriction during Init.
func (c *CYLoggerConfig) SetRestriction(bEnable, bClear bool,
	nTimeClear, nTimeExpired, nSizeTime, nCountTime, nSize, nCount, nTypeSize, nAllSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bLimitEnable = bEnable
	c.bClearUnLogFile = bClear
	c.nLimitTimeClearLog = nTimeClear
	c.nLimitTimeExpiredFile = nTimeExpired
	c.nCheckFileSizeTime = nSizeTime
	c.nCheckFileCountTime = nCountTime
	c.nCheckFileSize = nSize
	c.nCountPerType = nCount
	c.nCheckFileTypeSize = nTypeSize
	c.nCheckAllFileSize = nAllSize
	// LOG_TIME_CLEAR_LOG (nTimeClear) is the periodic cleanup cycle in C++. It
	// drives the background schedule's check period, so mirror it into the
	// existing nClearPeriodSec field (guarded so a 0 value keeps the current one).
	if nTimeClear > 0 {
		c.nClearPeriodSec = nTimeClear
	}
}

// IsLimitEnable returns whether file-size checking is enabled.
func (c *CYLoggerConfig) IsLimitEnable() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bLimitEnable
}

// IsClearUnLogFile returns whether unrecognized log files are cleared.
func (c *CYLoggerConfig) IsClearUnLogFile() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bClearUnLogFile
}

// GetTimeClearLog returns the periodic cleanup interval (seconds).
func (c *CYLoggerConfig) GetTimeClearLog() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nLimitTimeClearLog
}

// GetTimeExpiredFile returns the expired-file retention window (hours).
func (c *CYLoggerConfig) GetTimeExpiredFile() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nLimitTimeExpiredFile
}

// GetCheckFileSizeTime returns the size-check interval (seconds).
func (c *CYLoggerConfig) GetCheckFileSizeTime() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nCheckFileSizeTime
}

// GetCheckFileCountTime returns the count-check interval (seconds).
func (c *CYLoggerConfig) GetCheckFileCountTime() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nCheckFileCountTime
}

// GetCheckFileSize returns the per-file size threshold (bytes).
func (c *CYLoggerConfig) GetCheckFileSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nCheckFileSize
}

// GetCountPerType returns the max files kept per log type.
func (c *CYLoggerConfig) GetCountPerType() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nCountPerType
}

// GetCheckFileTypeSize returns the per-type total size threshold (bytes).
func (c *CYLoggerConfig) GetCheckFileTypeSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nCheckFileTypeSize
}

// GetCheckAllFileSize returns the global total size threshold (bytes).
func (c *CYLoggerConfig) GetCheckAllFileSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nCheckAllFileSize
}
