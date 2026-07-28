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

// The following constants mirror the default values defined in the C++ header
// Inc/ICYLoggerDefine.hpp (LOG_SHOW_CONSOLE_WINDOW, LOG_WRITE_REMOTE, the ten
// LOG_* restriction parameters, LOG_LEVEL_FILTER, ...). They let users override
// behaviour by editing a single, C++-named set of knobs.

const (
	// LOG_SHOW_CONSOLE_WINDOW mirrors whether the console appender is active.
	LOG_SHOW_CONSOLE_WINDOW = false
	// LOG_WRITE_REMOTE mirrors whether logs are forwarded to a remote server.
	LOG_WRITE_REMOTE = false
	// LOG_WRITE_SYS mirrors whether logs are written to the system event log.
	LOG_WRITE_SYS = false
	// LOG_FILE_MODE mirrors the default file naming mode.
	LOG_FILE_MODE = LogFileModeTime
	// LOG_LAYOUT_TYPE mirrors the default layout template.
	LOG_LAYOUT_TYPE = LogLayoutTypeBuildin1

	// LOG_LIMIT_ENABLE mirrors the master detection switch.
	LOG_LIMIT_ENABLE = true
	// LOG_LIMIT_CLEAR_UNLOGFILE mirrors whether non-log files are purged.
	LOG_LIMIT_CLEAR_UNLOGFILE = true

	// LOG_TIME_CLEAR_LOG mirrors the expired-log cleanup interval (seconds).
	LOG_TIME_CLEAR_LOG = 60
	// LOG_TIME_EXPIRED_FILE mirrors the expired-file retention window (hours).
	LOG_TIME_EXPIRED_FILE = 24
	// LOG_CHECK_FILE_SIZE_TIME mirrors the per-size detection interval (seconds).
	LOG_CHECK_FILE_SIZE_TIME = 60 * 5
	// LOG_CHECK_FILE_COUNT_TIME mirrors the per-count detection interval (seconds).
	LOG_CHECK_FILE_COUNT_TIME = 60
	// LOG_CHECK_FILE_SIZE mirrors the per-file size threshold (bytes).
	LOG_CHECK_FILE_SIZE = 1024 * 1024 * 5
	// LOG_COUNT_PER_TYPE mirrors the max files kept per log type.
	LOG_COUNT_PER_TYPE = 20
	// LOG_CHECK_FILE_TYPE_SIZE mirrors the per-type total size threshold (bytes).
	LOG_CHECK_FILE_TYPE_SIZE = 1024 * 1024 * 500
	// LOG_CHECK_FILE_ALL_SIZE mirrors the global total size threshold (bytes).
	LOG_CHECK_FILE_ALL_SIZE = 1024 * 1024 * 1024

	// LOG_LEVEL_FILTER mirrors the default level filter (all enabled).
	LOG_LEVEL_FILTER = LogFilterAll
	// LOG_MOUNT_MAIN mirrors whether the Main aggregate file appender is mounted.
	// True keeps the historical all-types behaviour (a Main.log aggregates every
	// enabled level); set false to keep a strict per-level file set.
	LOG_MOUNT_MAIN = true
)

// DefaultConfig returns a freshly allocated *CYLoggerConfig initialised with the
// same default values as the C++ CY_LOG_CONFIG macro. Users may tweak individual
// fields or pass the whole object through functional options before Init.
func DefaultConfig() *CYLoggerConfig {
	return &CYLoggerConfig{
		szLogPath:       "",
		szErrorLogPath:  "Error.log",
		bShowConsole:    LOG_SHOW_CONSOLE_WINDOW,
		bWriteRemote:    LOG_WRITE_REMOTE,
		bWriteSys:       LOG_WRITE_SYS,
		eFileMode:       LOG_FILE_MODE,
		eLayoutType:     LOG_LAYOUT_TYPE,
		eLogLevelFilter: LOG_LEVEL_FILTER,
		bMountMain:      LOG_MOUNT_MAIN,

		szRemoteAddr:   "127.0.0.1:7000",
		nClearPeriodSec: 60,
		eRemoteProto:   RemoteProtoTCP,

		bLimitEnable:          LOG_LIMIT_ENABLE,
		bClearUnLogFile:       LOG_LIMIT_CLEAR_UNLOGFILE,
		nLimitTimeClearLog:    LOG_TIME_CLEAR_LOG,
		nLimitTimeExpiredFile: LOG_TIME_EXPIRED_FILE,
		nCheckFileSizeTime:    LOG_CHECK_FILE_SIZE_TIME,
		nCheckFileCountTime:   LOG_CHECK_FILE_COUNT_TIME,
		nCheckFileSize:        LOG_CHECK_FILE_SIZE,
		nCountPerType:         LOG_COUNT_PER_TYPE,
		nCheckFileTypeSize:    LOG_CHECK_FILE_TYPE_SIZE,
		nCheckAllFileSize:     LOG_CHECK_FILE_ALL_SIZE,
	}
}
