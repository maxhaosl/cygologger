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

// Package Schedule manages background scheduled tasks for log file cleanup.
package Schedule

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	Common "github.com/maxhaosl/CYGoLogger/ICYLogger/Common"
)

// CYLoggerClearLogFile periodically cleans up old log files.
type CYLoggerClearLogFile struct {
	Common.CYNamedThread
	bEnable       bool
	nExpiredHours int
	szLogDir      string
}

func NewCYLoggerClearLogFile(szLogDir string, nExpiredHours int) *CYLoggerClearLogFile {
	t := &CYLoggerClearLogFile{
		bEnable:       true,
		nExpiredHours: nExpiredHours,
		szLogDir:      szLogDir,
	}
	t.CYNamedThread = *Common.NewCYNamedThread("CYLoggerClearLogFile")
	return t
}

func (c *CYLoggerClearLogFile) IsEnable() bool              { return c.bEnable }
func (c *CYLoggerClearLogFile) SetEnable(b bool)            { c.bEnable = b }
func (c *CYLoggerClearLogFile) SetExpiredHours(n int)        { c.nExpiredHours = n }
func (c *CYLoggerClearLogFile) SetLogDir(sz string)          { c.szLogDir = sz }

func (c *CYLoggerClearLogFile) DoClear() {
	if !c.bEnable || c.szLogDir == "" {
		return
	}
	expired := time.Now().Add(-time.Duration(c.nExpiredHours) * time.Hour)
	filepath.Walk(c.szLogDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".log") {
			return nil
		}
		if info.ModTime().Before(expired) {
			os.Remove(path)
		}
		return nil
	})
}

func (c *CYLoggerClearLogFile) StartSchedule() {
	c.Start(func() {
		ticker := time.NewTicker(time.Duration(60) * time.Second)
		defer ticker.Stop()
		for c.IsRunning() {
			<-ticker.C
			if c.bEnable {
				c.DoClear()
			}
		}
	})
}

// CYLoggerDoZipLog is a placeholder for log zipping.
type CYLoggerDoZipLog struct {
	Common.CYNamedThread
}

func NewCYLoggerDoZipLog() *CYLoggerDoZipLog {
	t := &CYLoggerDoZipLog{}
	t.CYNamedThread = *Common.NewCYNamedThread("CYLoggerDoZipLog")
	return t
}

func (z *CYLoggerDoZipLog) DoZipLog(szLogFile, szZipFile string) bool {
	_ = szLogFile
	_ = szZipFile
	return true
}

// CYLoggerSchedule manages background scheduled tasks.
type CYLoggerSchedule struct {
	Common.CYNoCopy
	mu         sync.RWMutex
	bEnable    bool
	clearTask  *CYLoggerClearLogFile
	zipTask    *CYLoggerDoZipLog
}

var g_CYLoggerScheduleInstance *CYLoggerSchedule
var g_CYLoggerScheduleOnce sync.Once

func GetCYLoggerScheduleInstance() *CYLoggerSchedule {
	g_CYLoggerScheduleOnce.Do(func() {
		g_CYLoggerScheduleInstance = &CYLoggerSchedule{bEnable: true}
	})
	return g_CYLoggerScheduleInstance
}

func (s *CYLoggerSchedule) Init(szLogDir string, nExpiredHours int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clearTask == nil {
		s.clearTask = NewCYLoggerClearLogFile(szLogDir, nExpiredHours)
		s.clearTask.SetEnable(s.bEnable)
	}
	if s.zipTask == nil {
		s.zipTask = NewCYLoggerDoZipLog()
	}
}

func (s *CYLoggerSchedule) StartSchedule() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.clearTask != nil {
		s.clearTask.StartSchedule()
	}
}

func (s *CYLoggerSchedule) StopSchedule() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.clearTask != nil {
		s.clearTask.Stop()
	}
}

func (s *CYLoggerSchedule) SetEnable(b bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bEnable = b
	if s.clearTask != nil {
		s.clearTask.SetEnable(b)
	}
}

func (s *CYLoggerSchedule) SetExpiredHours(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clearTask != nil {
		s.clearTask.SetExpiredHours(n)
	}
}

func (s *CYLoggerSchedule) SetLogDir(sz string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clearTask != nil {
		s.clearTask.SetLogDir(sz)
	}
}

func (s *CYLoggerSchedule) DoZipLog(szLogFile, szZipFile string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.zipTask != nil {
		return s.zipTask.DoZipLog(szLogFile, szZipFile)
	}
	return false
}
