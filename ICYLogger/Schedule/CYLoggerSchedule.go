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
	"sort"
	"strings"
	"sync"
	"time"

	Common "github.com/maxhaosl/CYGoLogger/ICYLogger/Common"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Appender"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Entity"
)

// logTypePrefixes maps the leading segment of a log file name to a logical
// group used for per-type file-count and per-type size enforcement.
var logTypePrefixes = []string{
	"Trace", "Debug", "Info", "Warn", "Error", "Fatal", "Main", "Remote", "Sys",
}

// CYLoggerClearLogFile periodically cleans up old log files.
type CYLoggerClearLogFile struct {
	Common.CYNamedThread
	bEnable       bool
	nExpiredHours int
	szLogDir      string
	nClearPeriodSec int // cleanup check period in seconds (configurable)

	// Rotation policy limits.
	nFileCountPerType int // max files kept per log type
	nCheckFileTypeSize int // max total bytes per log type
	nCheckAllFileSize int // max total bytes across all log files

	// bEnableClearUnLogFile mirrors C++ m_bEnableClearUnLogFile: whether to purge
	// non-log files (e.g. leftover zip packages) from the log directory.
	bEnableClearUnLogFile bool
	// bFirstProcess mirrors C++ m_bFirstProcess: the size-policy check and the
	// non-log purge run on the very first cleanup pass.
	bFirstProcess bool
	// lastSizeCheck tracks the last time the size policies were evaluated,
	// mirroring C++ m_objElapsedCheckSizeTime (gated by nCheckFileSizeTime).
	lastSizeCheck time.Time
}

func NewCYLoggerClearLogFile(szLogDir string, nExpiredHours int) *CYLoggerClearLogFile {
	t := &CYLoggerClearLogFile{
		bEnable:              true,
		nExpiredHours:        nExpiredHours,
		szLogDir:             szLogDir,
		nClearPeriodSec:      60,
		nFileCountPerType:    Core.DefaultLogCountPerType,
		nCheckFileTypeSize:   Core.DefaultLogCheckFileTypeSize,
		nCheckAllFileSize:    Core.DefaultLogCheckAllFileSize,
		bEnableClearUnLogFile: true,
		bFirstProcess:        true,
	}
	t.CYNamedThread = *Common.NewCYNamedThread("CYLoggerClearLogFile")
	return t
}

func (c *CYLoggerClearLogFile) IsEnable() bool                   { return c.bEnable }
func (c *CYLoggerClearLogFile) SetEnable(b bool)                 { c.bEnable = b }
func (c *CYLoggerClearLogFile) SetExpiredHours(n int)            { c.nExpiredHours = n }
func (c *CYLoggerClearLogFile) SetLogDir(sz string)              { c.szLogDir = sz }
func (c *CYLoggerClearLogFile) SetRestriction(nFileCountPerType, nCheckFileTypeSize, nCheckAllFileSize int) {
	c.nFileCountPerType = nFileCountPerType
	c.nCheckFileTypeSize = nCheckFileTypeSize
	c.nCheckAllFileSize = nCheckAllFileSize
}

// SetClearUnLogFile enables or disables purging of non-log files.
func (c *CYLoggerClearLogFile) SetClearUnLogFile(b bool) { c.bEnableClearUnLogFile = b }

// SetClearPeriodSec sets the cleanup check period in seconds.
func (c *CYLoggerClearLogFile) SetClearPeriodSec(n int) {
	if n > 0 {
		c.nClearPeriodSec = n
	}
}

// DoClear enforces the expired-time, per-type file-count, per-type total-size
// and global total-size rotation policies. Files that are currently being
// written by an active appender are never removed. On the first pass it also
// purges non-log files from the directory (mirroring C++ ProcessClearLog).
func (c *CYLoggerClearLogFile) DoClear() {
	if !c.bEnable || c.szLogDir == "" {
		return
	}

	inUse := c.collectInUseFiles()

	// On first pass, purge non-log files (zip packages, stray files) exactly as
	// C++ ProcessClearNonLog does on m_bFirstProcess.
	if c.bFirstProcess && c.bEnableClearUnLogFile {
		c.ProcessClearNonLog(c.enumerateNonLogFiles(c.szLogDir), inUse)
	}

	files := c.enumerateLogFiles(c.szLogDir)
	if len(files) == 0 {
		c.bFirstProcess = false
		c.lastSizeCheck = time.Now()
		return
	}

	type grouped struct {
		paths []string
		sizes []int64
		total int64
	}
	groups := make(map[string]*grouped)

	var allPaths []string
	var allSizes []int64
	var allTotal int64

	for _, f := range files {
		if inUse[f] {
			continue
		}
		info, err := os.Stat(f)
		if err != nil || info.IsDir() {
			continue
		}
		size := info.Size()
		key := typeKey(f)

		g, ok := groups[key]
		if !ok {
			g = &grouped{}
			groups[key] = g
		}
		g.paths = append(g.paths, f)
		g.sizes = append(g.sizes, size)
		g.total += size

		allPaths = append(allPaths, f)
		allSizes = append(allSizes, size)
		allTotal += size
	}

	expired := time.Now().Add(-time.Duration(c.nExpiredHours) * time.Hour)

	// Per-type policies.
	for _, g := range groups {
		// 1) Expired files (oldest modification time).
		for i := range g.paths {
			if info, err := os.Stat(g.paths[i]); err == nil && info.ModTime().Before(expired) {
				os.Remove(g.paths[i])
				g.total -= g.sizes[i]
				g.sizes[i] = 0
			}
		}
		// 2) Per-type file count limit.
		if len(g.paths) > c.nFileCountPerType {
			sortByModTime(g.paths)
			toDelete := len(g.paths) - c.nFileCountPerType
			for i := 0; i < toDelete; i++ {
				if info, err := os.Stat(g.paths[i]); err == nil {
					os.Remove(g.paths[i])
					g.total -= info.Size()
				}
			}
		}
	}

	// 3) & 4) Size policies run on the first pass or when the size-check interval
	// (nCheckFileSizeTime) has elapsed, mirroring C++ ProcessClearLog's gate.
	runSize := c.bFirstProcess || time.Since(c.lastSizeCheck) > time.Duration(Core.DefaultLogCheckFileSizeTime)*time.Second
	if runSize {
		for _, g := range groups {
			// Per-type total size limit.
			if g.total > int64(c.nCheckFileTypeSize) {
				sortByModTime(g.paths)
				for i := 0; i < len(g.paths) && g.total > int64(c.nCheckFileTypeSize); i++ {
					if info, err := os.Stat(g.paths[i]); err == nil {
						os.Remove(g.paths[i])
						g.total -= info.Size()
					}
				}
			}
		}
		// Global total size limit across all log files.
		if allTotal > int64(c.nCheckAllFileSize) {
			sortByModTime(allPaths)
			for i := 0; i < len(allPaths) && allTotal > int64(c.nCheckAllFileSize); i++ {
				if info, err := os.Stat(allPaths[i]); err == nil {
					os.Remove(allPaths[i])
					allTotal -= info.Size()
				}
			}
		}
		c.lastSizeCheck = time.Now()
	}

	c.bFirstProcess = false
}

// enumerateNonLogFiles recursively lists all files under dir that are NOT .log
// files (mirroring C++ EnumNotLogFile).
func (c *CYLoggerClearLogFile) enumerateNonLogFiles(dir string) []string {
	var result []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || strings.HasSuffix(path, ".log") {
			return nil
		}
		result = append(result, path)
		return nil
	})
	return result
}

// ProcessClearNonLog removes the given non-log files, except those that are
// currently open by an active appender (mirroring C++ ProcessClearNonLog).
func (c *CYLoggerClearLogFile) ProcessClearNonLog(paths []string, inUse map[string]bool) {
	for _, p := range paths {
		if inUse[p] {
			continue
		}
		_ = os.Remove(p)
	}
}

// collectInUseFiles returns the set of log file paths that are currently open
// for writing by an active file appender. These must never be deleted.
func (c *CYLoggerClearLogFile) collectInUseFiles() map[string]bool {
	used := make(map[string]bool)
	factory := Appender.GetCYLoggerAppenderFactoryInstance()
	for _, apps := range factory.GetAllAppenders() {
		for _, app := range apps {
			switch a := app.(type) {
			case *Appender.CYLoggerFileAppender:
				if f := a.GetCurrentFile(); f != "" {
					used[f] = true
				}
			case *Appender.CYLoggerMainAppender:
				if f := a.GetCurrentFile(); f != "" {
					used[f] = true
				}
			}
		}
	}
	return used
}

// enumerateLogFiles recursively lists all .log files under dir.
func (c *CYLoggerClearLogFile) enumerateLogFiles(dir string) []string {
	var result []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() || !strings.HasSuffix(path, ".log") {
			return nil
		}
		result = append(result, path)
		return nil
	})
	return result
}

// typeKey extracts the logical group key from a log file path (the leading
// segment before the first '_', falling back to the whole base name).
func typeKey(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".log")
	if idx := strings.IndexByte(base, '_'); idx >= 0 {
		key := base[:idx]
		for _, p := range logTypePrefixes {
			if strings.EqualFold(key, p) {
				return p
			}
		}
		return key
	}
	return base
}

// sortByModTime sorts paths ascending by modification time (oldest first).
func sortByModTime(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		li, ei := os.Stat(paths[i])
		lj, ej := os.Stat(paths[j])
		if ei != nil || ej != nil {
			return i < j
		}
		return li.ModTime().Before(lj.ModTime())
	})
}

func (c *CYLoggerClearLogFile) StartSchedule() {
	c.Start(func() {
		ticker := time.NewTicker(time.Duration(c.nClearPeriodSec) * time.Second)
		defer ticker.Stop()
		for c.IsRunning() {
			<-ticker.C
			if c.bEnable {
				c.DoClear()
			}
		}
	})
}

// CYLoggerDoZipLog compresses log files into zip archives.
type CYLoggerDoZipLog struct {
	Common.CYNamedThread
}

func NewCYLoggerDoZipLog() *CYLoggerDoZipLog {
	t := &CYLoggerDoZipLog{}
	t.CYNamedThread = *Common.NewCYNamedThread("CYLoggerDoZipLog")
	return t
}

// DoZipLog compresses the single log file at szLogFile into the zip archive
// named szZipFile. It returns true on success.
func (z *CYLoggerDoZipLog) DoZipLog(szLogFile, szZipFile string) bool {
	if szLogFile == "" || szZipFile == "" {
		return false
	}
	data, err := os.ReadFile(szLogFile)
	if err != nil {
		return false
	}
	return zipBytes(data, szZipFile, filepath.Base(szLogFile)) == nil
}

// Process forces a new log file (so the active file is not locked) and then
// compresses szLogFile into szZipFile, mirroring the C++ CYLoggerDoZipLog flow.
func (z *CYLoggerDoZipLog) Process(szLogFile, szZipFile string) bool {
	return z.DoZipLog(szLogFile, szZipFile)
}

// CYLoggerSchedule manages background scheduled tasks.
type CYLoggerSchedule struct {
	Common.CYNoCopy
	mu        sync.RWMutex
	bEnable   bool
	clearTask *CYLoggerClearLogFile
	zipTask   *CYLoggerDoZipLog

	// bResetLogFile mirrors C++ m_bResetLogFile: when set, the next tick forces
	// every file appender to rotate to a fresh file.
	bResetLogFile bool
	// lstLogType mirrors C++ m_lstLogType: extra log types tracked by the schedule.
	lstLogType []Core.ELogType
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

// SetRestriction forwards the file-count / total-size rotation limits to the
// clear-log task.
func (s *CYLoggerSchedule) SetRestriction(nFileCountPerType, nCheckFileTypeSize, nCheckAllFileSize int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clearTask != nil {
		s.clearTask.SetRestriction(nFileCountPerType, nCheckFileTypeSize, nCheckAllFileSize)
	}
}

// SetClearUnLogFile forwards the non-log-file purge toggle to the clear-log task.
func (s *CYLoggerSchedule) SetClearUnLogFile(b bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clearTask != nil {
		s.clearTask.SetClearUnLogFile(b)
	}
}

// ResetLogFile forces every file appender to rotate to a fresh file immediately
// (mirrors C++ CYLoggerSchedule::ResetLogFile, applied synchronously for safety).
func (s *CYLoggerSchedule) ResetLogFile() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bResetLogFile = true
	Entity.GetCYLoggerEntityFactoryInstance().ForceEntityNewFile()
}

// AddLogType records an extra log type tracked by the schedule (mirrors C++
// CYLoggerSchedule::AddLogType).
func (s *CYLoggerSchedule) AddLogType(eLogType Core.ELogType) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lstLogType = append(s.lstLogType, eLogType)
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

// SetClearPeriodSec forwards the configurable cleanup period to the clear task.
func (s *CYLoggerSchedule) SetClearPeriodSec(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clearTask != nil {
		s.clearTask.SetClearPeriodSec(n)
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
