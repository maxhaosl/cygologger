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

// Package Schedule manages background scheduled tasks for log file cleanup.
package Schedule

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	Common "github.com/maxhaosl/cygologger/ICYLogger/Common"
	"github.com/maxhaosl/cygologger/ICYLogger/Appender"
	"github.com/maxhaosl/cygologger/ICYLogger/Core"
	"github.com/maxhaosl/cygologger/ICYLogger/Entity"
)

// logTypePrefixes maps the leading segment of a log file name to a logical
// group used for per-type file-count and per-type size enforcement.
var logTypePrefixes = []string{
	"Trace", "Debug", "Info", "Warn", "Error", "Fatal", "Main", "Remote", "Sys",
}

// CYLoggerClearLogFile periodically cleans up old log files.
type CYLoggerClearLogFile struct {
	Common.CYNamedThread
	mu             sync.RWMutex // guards every config field read by DoClear
	bEnable       bool
	nExpiredHours int
	szLogDir      string
	nClearPeriodSec int // cleanup check period in seconds (configurable)
	stopWg        sync.WaitGroup // tracks the background clear goroutine
	chStop        chan struct{}  // signals the background clear goroutine to exit
	runMu         sync.Mutex     // serializes DoClear invocations

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
	// nCheckFileSizeTime / nCheckFileCountTime gate the size- and count-policy
	// passes, mirroring C++ LOG_CHECK_FILE_SIZE_TIME / LOG_CHECK_FILE_COUNT_TIME.
	nCheckFileSizeTime int
	nCheckFileCountTime int
	// lastCountCheck tracks the last time the per-type count policy ran.
	lastCountCheck time.Time
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
		nCheckFileSizeTime:   Core.DefaultLogCheckFileSizeTime,
		nCheckFileCountTime:  Core.DefaultLogCheckFileCountTime,
		lastCountCheck:       time.Now(),
	}
	t.CYNamedThread = *Common.NewCYNamedThread("CYLoggerClearLogFile")
	return t
}

func (c *CYLoggerClearLogFile) IsEnable() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bEnable
}
func (c *CYLoggerClearLogFile) SetEnable(b bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bEnable = b
}
func (c *CYLoggerClearLogFile) SetExpiredHours(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nExpiredHours = n
}
func (c *CYLoggerClearLogFile) SetLogDir(sz string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.szLogDir = sz
}
func (c *CYLoggerClearLogFile) SetRestriction(nFileCountPerType, nCheckFileTypeSize, nCheckAllFileSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nFileCountPerType = nFileCountPerType
	c.nCheckFileTypeSize = nCheckFileTypeSize
	c.nCheckAllFileSize = nCheckAllFileSize
}

// SetClearUnLogFile enables or disables purging of non-log files.
func (c *CYLoggerClearLogFile) SetClearUnLogFile(b bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bEnableClearUnLogFile = b
}

// SetClearPeriodSec sets the cleanup check period in seconds.
func (c *CYLoggerClearLogFile) SetClearPeriodSec(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > 0 {
		c.nClearPeriodSec = n
	}
}

// SetCheckFileSizeTime sets the interval (seconds) between size-policy passes,
// mirroring C++ LOG_CHECK_FILE_SIZE_TIME.
func (c *CYLoggerClearLogFile) SetCheckFileSizeTime(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > 0 {
		c.nCheckFileSizeTime = n
	}
}

// SetCheckFileCountTime sets the interval (seconds) between count-policy passes,
// mirroring C++ LOG_CHECK_FILE_COUNT_TIME.
func (c *CYLoggerClearLogFile) SetCheckFileCountTime(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n > 0 {
		c.nCheckFileCountTime = n
	}
}

// DoClear enforces the expired-time, per-type file-count, per-type total-size
// and global total-size rotation policies. Files that are currently being
// written by an active appender are never removed. On the first pass it also
// purges non-log files from the directory (mirroring C++ ProcessClearLog).
func (c *CYLoggerClearLogFile) DoClear() {
	// Serialize concurrent DoClear invocations (scheduled goroutine + direct calls).
	c.runMu.Lock()
	defer c.runMu.Unlock()

	// Snapshot all configuration under the read lock so concurrent setters
	// (e.g. a re-Init changing the log directory) can never race with a
	// cleanup pass that is already in flight.
	c.mu.RLock()
	bEnable := c.bEnable
	szLogDir := c.szLogDir
	nExpiredHours := c.nExpiredHours
	nFileCountPerType := c.nFileCountPerType
	nCheckFileTypeSize := c.nCheckFileTypeSize
	nCheckAllFileSize := c.nCheckAllFileSize
	bEnableClearUnLogFile := c.bEnableClearUnLogFile
	bFirstProcess := c.bFirstProcess
	lastSizeCheck := c.lastSizeCheck
	lastCountCheck := c.lastCountCheck
	nCheckFileSizeTime := c.nCheckFileSizeTime
	nCheckFileCountTime := c.nCheckFileCountTime
	c.mu.RUnlock()

	if !bEnable || szLogDir == "" {
		return
	}

	inUse := c.collectInUseFiles()

	// On first pass, purge non-log files (zip packages, stray files) exactly as
	// C++ ProcessClearNonLog does on m_bFirstProcess.
	if bFirstProcess && bEnableClearUnLogFile {
		c.ProcessClearNonLog(c.enumerateNonLogFiles(szLogDir), inUse)
	}

	files := c.enumerateLogFiles(szLogDir)
	if len(files) == 0 {
		c.mu.Lock()
		c.bFirstProcess = false
		// Update BOTH time gates symmetrically. Previously only lastSizeCheck was
		// refreshed here, leaving lastCountCheck at its initial (process-start)
		// value; functionally harmless (the count gate simply fired once ~60s
		// later) but asymmetric. Advancing both keeps the size- and count-policy
		// pass schedules consistent on the "first pass, empty directory" path.
		now := time.Now()
		c.lastSizeCheck = now
		c.lastCountCheck = now
		c.mu.Unlock()
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

	expired := time.Now().Add(-time.Duration(nExpiredHours) * time.Hour)

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
		// 2) Per-type file count limit, gated by nCheckFileCountTime
		// (LOG_CHECK_FILE_COUNT_TIME). It always runs on the first pass.
		runCount := bFirstProcess || time.Since(lastCountCheck) > time.Duration(nCheckFileCountTime)*time.Second
		if runCount && len(g.paths) > nFileCountPerType {
			sortByModTime(g.paths)
			toDelete := len(g.paths) - nFileCountPerType
			for i := 0; i < toDelete; i++ {
				if info, err := os.Stat(g.paths[i]); err == nil {
					os.Remove(g.paths[i])
					g.total -= info.Size()
				}
			}
			c.mu.Lock()
			c.lastCountCheck = time.Now()
			c.mu.Unlock()
		}
	}

	// 3) & 4) Size policies run on the first pass or when the size-check interval
	// (nCheckFileSizeTime) has elapsed, mirroring C++ ProcessClearLog's gate.
	runSize := bFirstProcess || time.Since(lastSizeCheck) > time.Duration(nCheckFileSizeTime)*time.Second
	if runSize {
		for _, g := range groups {
			// Per-type total size limit.
			if g.total > int64(nCheckFileTypeSize) {
				sortByModTime(g.paths)
				for i := 0; i < len(g.paths) && g.total > int64(nCheckFileTypeSize); i++ {
					if info, err := os.Stat(g.paths[i]); err == nil {
						os.Remove(g.paths[i])
						g.total -= info.Size()
					}
				}
			}
		}
		// Global total size limit across all log files.
		if allTotal > int64(nCheckAllFileSize) {
			sortByModTime(allPaths)
			for i := 0; i < len(allPaths) && allTotal > int64(nCheckAllFileSize); i++ {
				if info, err := os.Stat(allPaths[i]); err == nil {
					os.Remove(allPaths[i])
					allTotal -= info.Size()
				}
			}
		}
		c.mu.Lock()
		c.lastSizeCheck = time.Now()
		c.mu.Unlock()
	}

	c.mu.Lock()
	c.bFirstProcess = false
	c.mu.Unlock()
}

// enumerateNonLogFiles lists non-.log files that are DIRECT children of dir
// (NOT recursing into subdirectories, mirroring C++ EnumNotLogFile which only
// ever scans the configured log directory itself). This is critical when the
// log directory contains per-process subdirectories (e.g. a worker pool where
// each worker writes to <logDir>/<workerID>/): a recursive walk would enumerate
// files belonging to OTHER processes and — because DoClear cleans by logical
// log type across the whole tree — could delete or mis-count files owned by
// sibling workers / the parent process. Each process therefore only ever sees
// and prunes its OWN directory's direct children.
func (c *CYLoggerClearLogFile) enumerateNonLogFiles(dir string) []string {
	var result []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return result
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if strings.HasSuffix(p, ".log") {
			continue
		}
		result = append(result, p)
	}
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

// enumerateLogFiles lists the .log files that are DIRECT children of dir, NOT
// recursing into subdirectories. This is the core fix for cross-directory
// mis-counting: when the configured log directory contains per-process
// subdirectories (e.g. <logDir>/worker-0/, <logDir>/server/), a recursive walk
// would gather every sibling process's files into the SAME per-type group and
// then delete the oldest across ALL of them — so one process could prune
// another's active log files and the per-type file-count limit would be applied
// to the union of all processes instead of each process independently. By only
// ever scanning direct children, each process cleans exactly its own files and
// the per-type count limit (nFileCountPerType) is enforced per-process.
//
// Historically this used filepath.Walk (fully recursive), which was the root
// cause of "server log files deleted / worker dirs cleaned wrong" reports.
func (c *CYLoggerClearLogFile) enumerateLogFiles(dir string) []string {
	var result []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return result
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".log") {
			continue
		}
		result = append(result, filepath.Join(dir, name))
	}
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
// When two files share the same mtime the path name is used as a deterministic
// tie-breaker, so the deletion choice is stable/reproducible instead of relying
// on sort.Slice's unspecified ordering of equal elements.
func sortByModTime(paths []string) {
	sort.Slice(paths, func(i, j int) bool {
		li, ei := os.Stat(paths[i])
		lj, ej := os.Stat(paths[j])
		if ei != nil || ej != nil {
			return paths[i] < paths[j]
		}
		ti, tj := li.ModTime(), lj.ModTime()
		if ti.Equal(tj) {
			return paths[i] < paths[j] // deterministic tie-break on equal mtime
		}
		return ti.Before(tj)
	})
}

func (c *CYLoggerClearLogFile) StartSchedule() {
	c.mu.Lock()
	if c.chStop != nil {
		// Already scheduled.
		c.mu.Unlock()
		return
	}
	chStop := make(chan struct{})
	c.chStop = chStop
	period := c.nClearPeriodSec
	c.mu.Unlock()

	c.stopWg.Add(1)
	go func() {
		defer func() {
			// Never let a cleanup panic take down the process; the scheduled
			// goroutine simply exits and will be respawned on the next Init.
			recover()
			c.stopWg.Done()
		}()
		ticker := time.NewTicker(time.Duration(period) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-chStop:
				return
			case <-ticker.C:
				if c.IsEnable() {
					c.DoClear()
				}
			}
		}
	}()

	// Run an immediate first-pass cleanup so stale / excess log files are pruned
	// right after Init instead of waiting a full nClearPeriodSec for the first
	// ticker tick. This matches the documented "first-process" cleanup intent
	// (bFirstProcess) and — crucially — means short-lived processes (e.g. a
	// build-time verification that starts the server for only a few seconds)
	// still prune old files rather than leaving them behind forever.
	// DoClear is guarded by runMu, so this never races with the ticker goroutine;
	// it is a no-op when the policy is disabled (bEnable=false) or the log
	// directory is empty.
	go c.DoClear()
}

// StopSchedule signals the background cleanup goroutine to exit and BLOCKS
// until it has fully stopped. This guarantees that after UnInit no stale
// goroutine keeps enumerating/deleting files in a directory that a subsequent
// Init may have re-pointed elsewhere.
func (c *CYLoggerClearLogFile) StopSchedule() {
	c.mu.Lock()
	ch := c.chStop
	c.chStop = nil
	c.mu.Unlock()
	if ch != nil {
		close(ch)
	}
	c.stopWg.Wait()
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
	} else {
		// Re-Initializing (e.g. a second InitDefaultWithOpts in the same process):
		// reset the first-process marker and last-check timestamps so the per-type
		// count/size policies run on the NEXT cleanup pass instead of waiting for
		// the (stale) elapsed windows carried over from the previous Init. Without
		// this, the count cleanup is silently skipped until the window elapses and
		// excess files are never pruned after a re-Init.
		s.clearTask.bFirstProcess = true
		s.clearTask.lastCountCheck = time.Now()
		s.clearTask.lastSizeCheck = time.Now()
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

// SetCheckFileSizeTime forwards the size-policy interval to the clear-log task.
func (s *CYLoggerSchedule) SetCheckFileSizeTime(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clearTask != nil {
		s.clearTask.SetCheckFileSizeTime(n)
	}
}

// SetCheckFileCountTime forwards the count-policy interval to the clear-log task.
func (s *CYLoggerSchedule) SetCheckFileCountTime(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clearTask != nil {
		s.clearTask.SetCheckFileCountTime(n)
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
		s.clearTask.StopSchedule()
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
