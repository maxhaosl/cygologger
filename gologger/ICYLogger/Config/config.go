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

// Package Config manages global logger configuration.
package Config

import (
	"sync"

	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
	Common "github.com/maxhaosl/CYGoLogger/ICYLogger/Common"
)

// CYLoggerConfig manages global logger configuration.
type CYLoggerConfig struct {
	Common.CYNoCopy
	mu              sync.RWMutex
	szLogPath       string
	szErrorLogPath  string
	bShowConsole    bool
	bWriteRemote    bool
	bWriteSys       bool
	eFileMode       Core.ELogFileMode
	eLayoutType     Core.ELogLayoutType
	eLogLevelFilter Core.ELogLevelFilter
}

var g_CYLoggerConfigInstance *CYLoggerConfig
var g_CYLoggerConfigOnce sync.Once

func GetCYLoggerConfigInstance() *CYLoggerConfig {
	g_CYLoggerConfigOnce.Do(func() {
		g_CYLoggerConfigInstance = &CYLoggerConfig{
			szLogPath:        "",
			szErrorLogPath:   "Error.log",
			bShowConsole:     Core.DefaultLogShowConsoleWindow,
			bWriteRemote:     Core.DefaultLogWriteRemote,
			bWriteSys:        Core.DefaultLogWriteSys,
			eFileMode:        Core.DefaultLogFileMode,
			eLayoutType:      Core.DefaultLogLayoutType,
			eLogLevelFilter:  Core.DefaultLogLevelFilter,
		}
	})
	return g_CYLoggerConfigInstance
}

func (c *CYLoggerConfig) SetLogPath(szPath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.szLogPath = szPath
}
func (c *CYLoggerConfig) GetLogPath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.szLogPath
}
func (c *CYLoggerConfig) SetErrorLogPath(szPath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.szErrorLogPath = szPath
}
func (c *CYLoggerConfig) GetErrorLogPath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.szErrorLogPath
}
func (c *CYLoggerConfig) SetShowConsole(bShow bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bShowConsole = bShow
}
func (c *CYLoggerConfig) IsShowConsole() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bShowConsole
}
func (c *CYLoggerConfig) SetWriteRemote(bWrite bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bWriteRemote = bWrite
}
func (c *CYLoggerConfig) IsWriteRemote() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bWriteRemote
}
func (c *CYLoggerConfig) SetWriteSys(bWrite bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bWriteSys = bWrite
}
func (c *CYLoggerConfig) IsWriteSys() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bWriteSys
}
func (c *CYLoggerConfig) SetFileMode(eMode Core.ELogFileMode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eFileMode = eMode
}
func (c *CYLoggerConfig) GetFileMode() Core.ELogFileMode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.eFileMode
}
func (c *CYLoggerConfig) SetLayoutType(eType Core.ELogLayoutType) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eLayoutType = eType
}
func (c *CYLoggerConfig) GetLayoutType() Core.ELogLayoutType {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.eLayoutType
}
func (c *CYLoggerConfig) SetLogLevelFilter(eFilter Core.ELogLevelFilter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.eLogLevelFilter = eFilter
}
func (c *CYLoggerConfig) GetLogLevelFilter() Core.ELogLevelFilter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.eLogLevelFilter
}
