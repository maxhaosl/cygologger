/*
 * cygologger License ... (MIT license) ...
 */

// Package Statistics tracks all logging statistics atomically.
package Statistics

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/maxhaosl/cygologger/ICYLogger/Core"
	Common "github.com/maxhaosl/cygologger/ICYLogger/Common"
)

// perTypeStats holds statistics for a single log type.
type perTypeStats struct {
	nLine atomic.Uint64
	nByte atomic.Uint64
	fps   *Common.CYFPSCounter
}

type CYStatistics struct {
	Common.CYNoCopy
	mu        sync.Mutex
	startTime atomic.Int64 // UnixNano baseline used for AverageFps calculation

	nTotalLine       atomic.Uint64
	nTotalByte       atomic.Uint64
	fTotalCurrentFps atomic.Value
	fTotalAverageFps atomic.Value
	totalFps         *Common.CYFPSCounter

	console perTypeStats
	trace   perTypeStats
	debug   perTypeStats
	info    perTypeStats
	warn    perTypeStats
	e       perTypeStats // "error" is a keyword
	fatal   perTypeStats
	main    perTypeStats
	remote  perTypeStats
	sys     perTypeStats

	// Per-type public queue lengths (matching C++ per-subtype queues for Console/Main/Remote)
	nConsolePublicDQueue atomic.Uint32
	nConsolePublicTQueue atomic.Uint32
	nConsolePublicIQueue atomic.Uint32
	nConsolePublicWQueue atomic.Uint32
	nConsolePublicEQueue atomic.Uint32
	nConsolePublicFQueue atomic.Uint32
	nConsolePrivateQueue atomic.Uint32

	nTracePublicQueue  atomic.Uint32
	nTracePrivateQueue atomic.Uint32

	nDebugPublicQueue  atomic.Uint32
	nDebugPrivateQueue atomic.Uint32

	nInfoPublicQueue  atomic.Uint32
	nInfoPrivateQueue atomic.Uint32

	nWarnPublicQueue  atomic.Uint32
	nWarnPrivateQueue atomic.Uint32

	nErrorPublicQueue  atomic.Uint32
	nErrorPrivateQueue atomic.Uint32

	nFatalPublicQueue  atomic.Uint32
	nFatalPrivateQueue atomic.Uint32

	nMainPublicDQueue atomic.Uint32
	nMainPublicTQueue atomic.Uint32
	nMainPublicIQueue atomic.Uint32
	nMainPublicWQueue atomic.Uint32
	nMainPublicEQueue atomic.Uint32
	nMainPublicFQueue atomic.Uint32
	nMainPrivateQueue atomic.Uint32

	nRemotePublicDQueue atomic.Uint32
	nRemotePublicTQueue atomic.Uint32
	nRemotePublicIQueue atomic.Uint32
	nRemotePublicWQueue atomic.Uint32
	nRemotePublicEQueue atomic.Uint32
	nRemotePublicFQueue atomic.Uint32
	nRemotePrivateQueue atomic.Uint32

	nSysPublicQueue  atomic.Uint32
	nSysPrivateQueue atomic.Uint32

	nTotalPublicQueue  atomic.Uint64
	nTotalPrivateQueue atomic.Uint64
}

var g_CYStatisticsInstance *CYStatistics
var g_CYStatisticsOnce sync.Once

func newFpsCounter() *Common.CYFPSCounter {
	fps := Common.NewCYFPSCounter(Core.LOG_FPS_CHECK_DURATION)
	fps.Start()
	return fps
}

func GetCYStatisticsInstance() *CYStatistics {
	g_CYStatisticsOnce.Do(func() {
		g_CYStatisticsInstance = &CYStatistics{
			totalFps: newFpsCounter(),
			console:   perTypeStats{fps: newFpsCounter()},
			trace:     perTypeStats{fps: newFpsCounter()},
			debug:     perTypeStats{fps: newFpsCounter()},
			info:      perTypeStats{fps: newFpsCounter()},
			warn:      perTypeStats{fps: newFpsCounter()},
			e:       perTypeStats{fps: newFpsCounter()},
			fatal:     perTypeStats{fps: newFpsCounter()},
			main:    perTypeStats{fps: newFpsCounter()},
			remote:    perTypeStats{fps: newFpsCounter()},
			sys:       perTypeStats{fps: newFpsCounter()},
		}
		g_CYStatisticsInstance.startTime.Store(time.Now().UnixNano())
	})
	return g_CYStatisticsInstance
}

// computeAverageFps returns the running average FPS = total lines / elapsed seconds.
func (s *CYStatistics) computeAverageFps(totalLines uint64) float64 {
	elapsed := time.Since(time.Unix(0, s.startTime.Load())).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return float64(totalLines) / elapsed
}

func (s *CYStatistics) IncrementLine(eMsgType Core.ELogType, nBytes uint64) {
	s.nTotalLine.Add(1)
	s.nTotalByte.Add(nBytes)
	s.totalFps.Increment()

	switch eMsgType {
	case Core.LogTypeConsole:
		s.console.nLine.Add(1)
		s.console.nByte.Add(nBytes)
		s.console.fps.Increment()
	case Core.LogTypeTrace:
		s.trace.nLine.Add(1)
		s.trace.nByte.Add(nBytes)
		s.trace.fps.Increment()
	case Core.LogTypeDebug:
		s.debug.nLine.Add(1)
		s.debug.nByte.Add(nBytes)
		s.debug.fps.Increment()
	case Core.LogTypeInfo:
		s.info.nLine.Add(1)
		s.info.nByte.Add(nBytes)
		s.info.fps.Increment()
	case Core.LogTypeWarn:
		s.warn.nLine.Add(1)
		s.warn.nByte.Add(nBytes)
		s.warn.fps.Increment()
	case Core.LogTypeError:
		s.e.nLine.Add(1)
		s.e.nByte.Add(nBytes)
		s.e.fps.Increment()
	case Core.LogTypeFatal:
		s.fatal.nLine.Add(1)
		s.fatal.nByte.Add(nBytes)
		s.fatal.fps.Increment()
	case Core.LogTypeMain:
		s.main.nLine.Add(1)
		s.main.nByte.Add(nBytes)
		s.main.fps.Increment()
	case Core.LogTypeRemote:
		s.remote.nLine.Add(1)
		s.remote.nByte.Add(nBytes)
		s.remote.fps.Increment()
	case Core.LogTypeSys:
		s.sys.nLine.Add(1)
		s.sys.nByte.Add(nBytes)
		s.sys.fps.Increment()
	}
}

// IncrementLinesByType tracks lines/bytes per type explicitly (used by appenders).
func (s *CYStatistics) IncrementLinesByType(eMsgType Core.ELogType, nLines, nBytes uint64) {
	switch eMsgType {
	case Core.LogTypeConsole:
		s.console.nLine.Add(nLines)
		s.console.nByte.Add(nBytes)
	case Core.LogTypeTrace:
		s.trace.nLine.Add(nLines)
		s.trace.nByte.Add(nBytes)
	case Core.LogTypeDebug:
		s.debug.nLine.Add(nLines)
		s.debug.nByte.Add(nBytes)
	case Core.LogTypeInfo:
		s.info.nLine.Add(nLines)
		s.info.nByte.Add(nBytes)
	case Core.LogTypeWarn:
		s.warn.nLine.Add(nLines)
		s.warn.nByte.Add(nBytes)
	case Core.LogTypeError:
		s.e.nLine.Add(nLines)
		s.e.nByte.Add(nBytes)
	case Core.LogTypeFatal:
		s.fatal.nLine.Add(nLines)
		s.fatal.nByte.Add(nBytes)
	case Core.LogTypeMain:
		s.main.nLine.Add(nLines)
		s.main.nByte.Add(nBytes)
	case Core.LogTypeRemote:
		s.remote.nLine.Add(nLines)
		s.remote.nByte.Add(nBytes)
	case Core.LogTypeSys:
		s.sys.nLine.Add(nLines)
		s.sys.nByte.Add(nBytes)
	}
}

func (s *CYStatistics) GetStats(pStats *Core.STStatistics) bool {
	if pStats == nil {
		return false
	}
	pStats.NTotalLine = s.nTotalLine.Load()
	pStats.NTotalByte = s.nTotalByte.Load()
	pStats.FTotalCurrentFps = s.totalFps.GetFPS()
	pStats.FTotalAverageFps = s.computeAverageFps(pStats.NTotalLine)
	pStats.NTotalPublicQueue = uint32(s.nTotalPublicQueue.Load())
	pStats.NTotalPrivateQueue = uint32(s.nTotalPrivateQueue.Load())

	// Console
	pStats.NConsoleLine = s.console.nLine.Load()
	pStats.NConsoleByte = s.console.nByte.Load()
	pStats.FConsoleCurrentFps = s.console.fps.GetFPS()
	pStats.FConsoleAverageFps = s.computeAverageFps(pStats.NConsoleLine)
	pStats.NConsolePublicDQueue = s.nConsolePublicDQueue.Load()
	pStats.NConsolePublicTQueue = s.nConsolePublicTQueue.Load()
	pStats.NConsolePublicIQueue = s.nConsolePublicIQueue.Load()
	pStats.NConsolePublicWQueue = s.nConsolePublicWQueue.Load()
	pStats.NConsolePublicEQueue = s.nConsolePublicEQueue.Load()
	pStats.NConsolePublicFQueue = s.nConsolePublicFQueue.Load()
	pStats.NConsolePrivateQueue = s.nConsolePrivateQueue.Load()

	// Trace
	pStats.NTraceLine = s.trace.nLine.Load()
	pStats.NTraceByte = s.trace.nByte.Load()
	pStats.FTraceCurrentFps = s.trace.fps.GetFPS()
	pStats.FTraceAverageFps = s.computeAverageFps(pStats.NTraceLine)
	pStats.NTracePublicQueue = s.nTracePublicQueue.Load()
	pStats.NTracePrivateQueue = s.nTracePrivateQueue.Load()

	// Debug
	pStats.NDebugLine = s.debug.nLine.Load()
	pStats.NDebugByte = s.debug.nByte.Load()
	pStats.FDebugCurrentFps = s.debug.fps.GetFPS()
	pStats.FDebugAverageFps = s.computeAverageFps(pStats.NDebugLine)
	pStats.NDebugPublicQueue = s.nDebugPublicQueue.Load()
	pStats.NDebugPrivateQueue = s.nDebugPrivateQueue.Load()

	// Info
	pStats.NInfoLine = s.info.nLine.Load()
	pStats.NInfoByte = s.info.nByte.Load()
	pStats.FInfoCurrentFps = s.info.fps.GetFPS()
	pStats.FInfoAverageFps = s.computeAverageFps(pStats.NInfoLine)
	pStats.NInfoPublicQueue = s.nInfoPublicQueue.Load()
	pStats.NInfoPrivateQueue = s.nInfoPrivateQueue.Load()

	// Warn
	pStats.NWarnLine = s.warn.nLine.Load()
	pStats.NWarnByte = s.warn.nByte.Load()
	pStats.FWarnCurrentFps = s.warn.fps.GetFPS()
	pStats.FWarnAverageFps = s.computeAverageFps(pStats.NWarnLine)
	pStats.NWarnPublicQueue = s.nWarnPublicQueue.Load()
	pStats.NWarnPrivateQueue = s.nWarnPrivateQueue.Load()

	// Error
	pStats.NErrorLine = s.e.nLine.Load()
	pStats.NErrorByte = s.e.nByte.Load()
	pStats.FErrorCurrentFps = s.e.fps.GetFPS()
	pStats.FErrorAverageFps = s.computeAverageFps(pStats.NErrorLine)
	pStats.NErrorPublicQueue = s.nErrorPublicQueue.Load()
	pStats.NErrorPrivateQueue = s.nErrorPrivateQueue.Load()

	// Fatal
	pStats.NFatalLine = s.fatal.nLine.Load()
	pStats.NFatalByte = s.fatal.nByte.Load()
	pStats.FFatalCurrentFps = s.fatal.fps.GetFPS()
	pStats.FFatalAverageFps = s.computeAverageFps(pStats.NFatalLine)
	pStats.NFatalPublicQueue = s.nFatalPublicQueue.Load()
	pStats.NFatalPrivateQueue = s.nFatalPrivateQueue.Load()

	// Main
	pStats.NMainLine = s.main.nLine.Load()
	pStats.NMainByte = s.main.nByte.Load()
	pStats.FMainCurrentFps = s.main.fps.GetFPS()
	pStats.FMainAverageFps = s.computeAverageFps(pStats.NMainLine)
	pStats.NMainPublicDQueue = s.nMainPublicDQueue.Load()
	pStats.NMainPublicTQueue = s.nMainPublicTQueue.Load()
	pStats.NMainPublicIQueue = s.nMainPublicIQueue.Load()
	pStats.NMainPublicWQueue = s.nMainPublicWQueue.Load()
	pStats.NMainPublicEQueue = s.nMainPublicEQueue.Load()
	pStats.NMainPublicFQueue = s.nMainPublicFQueue.Load()
	pStats.NMainPrivateQueue = s.nMainPrivateQueue.Load()

	// Remote
	pStats.NRemoteLine = s.remote.nLine.Load()
	pStats.NRemoteByte = s.remote.nByte.Load()
	pStats.FRemoteCurrentFps = s.remote.fps.GetFPS()
	pStats.FRemoteAverageFps = s.computeAverageFps(pStats.NRemoteLine)
	pStats.NRemotePublicDQueue = s.nRemotePublicDQueue.Load()
	pStats.NRemotePublicTQueue = s.nRemotePublicTQueue.Load()
	pStats.NRemotePublicIQueue = s.nRemotePublicIQueue.Load()
	pStats.NRemotePublicWQueue = s.nRemotePublicWQueue.Load()
	pStats.NRemotePublicEQueue = s.nRemotePublicEQueue.Load()
	pStats.NRemotePublicFQueue = s.nRemotePublicFQueue.Load()
	pStats.NRemotePrivateQueue = s.nRemotePrivateQueue.Load()

	// Sys
	pStats.NSysLine = s.sys.nLine.Load()
	pStats.NSysByte = s.sys.nByte.Load()
	pStats.FSysCurrentFps = s.sys.fps.GetFPS()
	pStats.FSysAverageFps = s.computeAverageFps(pStats.NSysLine)
	pStats.NSysPublicQueue = s.nSysPublicQueue.Load()
	pStats.NSysPrivateQueue = s.nSysPrivateQueue.Load()

	return true
}

// SetPublicQueueSize sets the total public queue size.
func (s *CYStatistics) SetPublicQueueSize(nSize uint64) {
	s.nTotalPublicQueue.Store(nSize)
}

// SetPrivateQueueSize sets the total private queue size.
func (s *CYStatistics) SetPrivateQueueSize(nSize uint64) {
	s.nTotalPrivateQueue.Store(nSize)
}

// SetPerTypeQueueSize sets the per-type queue size (used by BufferAppender).
func (s *CYStatistics) SetPerTypeQueueSize(eLogType Core.ELogType, nPubSize, nPriSize uint32) {
	s.setPerTypeVal(eLogType, nPubSize, nPriSize)
}

func (s *CYStatistics) setPerTypeVal(eLogType Core.ELogType, pub, pri uint32) {
	switch eLogType {
	case Core.LogTypeConsole:
		s.nConsolePublicDQueue.Store(pub)
		s.nConsolePrivateQueue.Store(pri)
	case Core.LogTypeTrace:
		s.nTracePublicQueue.Store(pub)
		s.nTracePrivateQueue.Store(pri)
	case Core.LogTypeDebug:
		s.nDebugPublicQueue.Store(pub)
		s.nDebugPrivateQueue.Store(pri)
	case Core.LogTypeInfo:
		s.nInfoPublicQueue.Store(pub)
		s.nInfoPrivateQueue.Store(pri)
	case Core.LogTypeWarn:
		s.nWarnPublicQueue.Store(pub)
		s.nWarnPrivateQueue.Store(pri)
	case Core.LogTypeError:
		s.nErrorPublicQueue.Store(pub)
		s.nErrorPrivateQueue.Store(pri)
	case Core.LogTypeFatal:
		s.nFatalPublicQueue.Store(pub)
		s.nFatalPrivateQueue.Store(pri)
	case Core.LogTypeMain:
		s.nMainPublicDQueue.Store(pub)
		s.nMainPrivateQueue.Store(pri)
	case Core.LogTypeRemote:
		s.nRemotePublicDQueue.Store(pub)
		s.nRemotePrivateQueue.Store(pri)
	case Core.LogTypeSys:
		s.nSysPublicQueue.Store(pub)
		s.nSysPrivateQueue.Store(pri)
	}
}

// ReportQueueLengths records the live public and private queue lengths for the
// given appender type. It mirrors C++ CYStatistics::Add*PublicQueue /
// Add*PrivateQueue, which every appender pushes on its update tick. For
// Console/Main/Remote (which in C++ expose six per-subtype public queues) the Go
// single-queue appender reports its length into the Debug ("D") sub-queue slot,
// keeping STStatistics fully populated and aligned with the C++ layout.
func (s *CYStatistics) ReportQueueLengths(eLogType Core.ELogType, publicLen, privateLen uint32) {
	s.setPerTypeVal(eLogType, publicLen, privateLen)
}

// Reset zeroes all cumulative statistics counters (lines, bytes and FPS),
// mirroring C++ CYStatistics::Reset(). Queue-length snapshot fields are
// intentionally left untouched: they are continuously re-reported by each
// appender's update tick, so they self-heal on the next tick after a reset. The
// elapsed-time baseline used for the average-FPS computation is also reset so the
// average restarts from the reset instant.
func (s *CYStatistics) Reset() {
	s.nTotalLine.Store(0)
	s.nTotalByte.Store(0)
	s.startTime.Store(time.Now().UnixNano())

	s.totalFps.Reset()

	s.console.nLine.Store(0)
	s.console.nByte.Store(0)
	s.console.fps.Reset()

	s.trace.nLine.Store(0)
	s.trace.nByte.Store(0)
	s.trace.fps.Reset()

	s.debug.nLine.Store(0)
	s.debug.nByte.Store(0)
	s.debug.fps.Reset()

	s.info.nLine.Store(0)
	s.info.nByte.Store(0)
	s.info.fps.Reset()

	s.warn.nLine.Store(0)
	s.warn.nByte.Store(0)
	s.warn.fps.Reset()

	s.e.nLine.Store(0)
	s.e.nByte.Store(0)
	s.e.fps.Reset()

	s.fatal.nLine.Store(0)
	s.fatal.nByte.Store(0)
	s.fatal.fps.Reset()

	s.main.nLine.Store(0)
	s.main.nByte.Store(0)
	s.main.fps.Reset()

	s.remote.nLine.Store(0)
	s.remote.nByte.Store(0)
	s.remote.fps.Reset()

	s.sys.nLine.Store(0)
	s.sys.nByte.Store(0)
	s.sys.fps.Reset()
}

