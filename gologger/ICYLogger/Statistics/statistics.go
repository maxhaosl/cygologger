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

// Package Statistics tracks all logging statistics atomically.
package Statistics

import (
	"sync"
	"sync/atomic"

	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
	Common "github.com/maxhaosl/CYGoLogger/ICYLogger/Common"
)

type CYStatistics struct {
	Common.CYNoCopy
	mu sync.Mutex

	nTotalLine              atomic.Uint64
	nTotalByte              atomic.Uint64
	fTotalCurrentFps        atomic.Value
	fTotalAverageFps        atomic.Value

	nConsoleLine            atomic.Uint64
	nConsoleByte            atomic.Uint64
	fConsoleCurrentFps      atomic.Value
	fConsoleAverageFps      atomic.Value

	nTraceLine, nTraceByte  atomic.Uint64
	nDebugLine, nDebugByte  atomic.Uint64
	nInfoLine, nInfoByte    atomic.Uint64
	nWarnLine, nWarnByte    atomic.Uint64
	nErrorLine, nErrorByte  atomic.Uint64
	nFatalLine, nFatalByte  atomic.Uint64
	nMainLine, nMainByte    atomic.Uint64
	nRemoteLine, nRemoteByte atomic.Uint64
	nSysLine, nSysByte      atomic.Uint64

	consoleFps  *Common.CYFPSCounter
	traceFps    *Common.CYFPSCounter
	debugFps    *Common.CYFPSCounter
	infoFps     *Common.CYFPSCounter
	warnFps     *Common.CYFPSCounter
	errorFps    *Common.CYFPSCounter
	fatalFps    *Common.CYFPSCounter
	mainFps     *Common.CYFPSCounter
	remoteFps   *Common.CYFPSCounter
	sysFps      *Common.CYFPSCounter

	nTotalPublicQueue  atomic.Uint64
	nTotalPrivateQueue atomic.Uint64
}

var g_CYStatisticsInstance *CYStatistics
var g_CYStatisticsOnce sync.Once

func GetCYStatisticsInstance() *CYStatistics {
	g_CYStatisticsOnce.Do(func() {
		g_CYStatisticsInstance = &CYStatistics{
			consoleFps: Common.NewCYFPSCounter(1000),
			traceFps:   Common.NewCYFPSCounter(1000),
			debugFps:   Common.NewCYFPSCounter(1000),
			infoFps:    Common.NewCYFPSCounter(1000),
			warnFps:    Common.NewCYFPSCounter(1000),
			errorFps:   Common.NewCYFPSCounter(1000),
			fatalFps:   Common.NewCYFPSCounter(1000),
			mainFps:    Common.NewCYFPSCounter(1000),
			remoteFps:  Common.NewCYFPSCounter(1000),
			sysFps:     Common.NewCYFPSCounter(1000),
		}
		g_CYStatisticsInstance.consoleFps.Start()
		g_CYStatisticsInstance.traceFps.Start()
		g_CYStatisticsInstance.debugFps.Start()
		g_CYStatisticsInstance.infoFps.Start()
		g_CYStatisticsInstance.warnFps.Start()
		g_CYStatisticsInstance.errorFps.Start()
		g_CYStatisticsInstance.fatalFps.Start()
		g_CYStatisticsInstance.mainFps.Start()
		g_CYStatisticsInstance.remoteFps.Start()
		g_CYStatisticsInstance.sysFps.Start()
	})
	return g_CYStatisticsInstance
}

func (s *CYStatistics) IncrementLine(eMsgType Core.ELogType, nBytes uint64) {
	s.nTotalLine.Add(1)
	s.nTotalByte.Add(nBytes)
	switch eMsgType {
	case Core.LogTypeConsole:
		s.nConsoleLine.Add(1)
		s.nConsoleByte.Add(nBytes)
		s.consoleFps.Increment()
	case Core.LogTypeTrace:
		s.nTraceLine.Add(1)
		s.nTraceByte.Add(nBytes)
		s.traceFps.Increment()
	case Core.LogTypeDebug:
		s.nDebugLine.Add(1)
		s.nDebugByte.Add(nBytes)
		s.debugFps.Increment()
	case Core.LogTypeInfo:
		s.nInfoLine.Add(1)
		s.nInfoByte.Add(nBytes)
		s.infoFps.Increment()
	case Core.LogTypeWarn:
		s.nWarnLine.Add(1)
		s.nWarnByte.Add(nBytes)
		s.warnFps.Increment()
	case Core.LogTypeError:
		s.nErrorLine.Add(1)
		s.nErrorByte.Add(nBytes)
		s.errorFps.Increment()
	case Core.LogTypeFatal:
		s.nFatalLine.Add(1)
		s.nFatalByte.Add(nBytes)
		s.fatalFps.Increment()
	case Core.LogTypeMain:
		s.nMainLine.Add(1)
		s.nMainByte.Add(nBytes)
		s.mainFps.Increment()
	case Core.LogTypeRemote:
		s.nRemoteLine.Add(1)
		s.nRemoteByte.Add(nBytes)
		s.remoteFps.Increment()
	case Core.LogTypeSys:
		s.nSysLine.Add(1)
		s.nSysByte.Add(nBytes)
		s.sysFps.Increment()
	}
}

func (s *CYStatistics) GetStats(pStats *Core.STStatistics) bool {
	if pStats == nil {
		return false
	}
	pStats.NTotalLine = s.nTotalLine.Load()
	pStats.NTotalByte = s.nTotalByte.Load()
	pStats.FTotalCurrentFps = s.getFloat(s.fTotalCurrentFps.Load())
	pStats.FTotalAverageFps = s.getFloat(s.fTotalAverageFps.Load())
	pStats.NTotalPublicQueue = uint32(s.nTotalPublicQueue.Load())
	pStats.NTotalPrivateQueue = uint32(s.nTotalPrivateQueue.Load())

	pStats.NConsoleLine = s.nConsoleLine.Load()
	pStats.NConsoleByte = s.nConsoleByte.Load()
	pStats.FConsoleCurrentFps = s.consoleFps.GetFPS()
	pStats.FConsoleAverageFps = s.getFloat(s.fConsoleAverageFps.Load())

	pStats.NTraceLine = s.nTraceLine.Load()
	pStats.NTraceByte = s.nTraceByte.Load()
	pStats.FTraceCurrentFps = s.traceFps.GetFPS()

	pStats.NDebugLine = s.nDebugLine.Load()
	pStats.NDebugByte = s.nDebugByte.Load()
	pStats.FDebugCurrentFps = s.debugFps.GetFPS()

	pStats.NInfoLine = s.nInfoLine.Load()
	pStats.NInfoByte = s.nInfoByte.Load()
	pStats.FInfoCurrentFps = s.infoFps.GetFPS()

	pStats.NWarnLine = s.nWarnLine.Load()
	pStats.NWarnByte = s.nWarnByte.Load()
	pStats.FWarnCurrentFps = s.warnFps.GetFPS()

	pStats.NErrorLine = s.nErrorLine.Load()
	pStats.NErrorByte = s.nErrorByte.Load()
	pStats.FErrorCurrentFps = s.errorFps.GetFPS()

	pStats.NFatalLine = s.nFatalLine.Load()
	pStats.NFatalByte = s.nFatalByte.Load()
	pStats.FFatalCurrentFps = s.fatalFps.GetFPS()

	pStats.NMainLine = s.nMainLine.Load()
	pStats.NMainByte = s.nMainByte.Load()
	pStats.FMainCurrentFps = s.mainFps.GetFPS()

	pStats.NRemoteLine = s.nRemoteLine.Load()
	pStats.NRemoteByte = s.nRemoteByte.Load()
	pStats.FRemoteCurrentFps = s.remoteFps.GetFPS()

	pStats.NSysLine = s.nSysLine.Load()
	pStats.NSysByte = s.nSysByte.Load()
	pStats.FSysCurrentFps = s.sysFps.GetFPS()

	return true
}

func (s *CYStatistics) getFloat(v any) float64 {
	if v == nil {
		return 0
	}
	return v.(float64)
}

func (s *CYStatistics) SetPublicQueueSize(nSize uint64) {
	s.nTotalPublicQueue.Store(nSize)
}

func (s *CYStatistics) SetPrivateQueueSize(nSize uint64) {
	s.nTotalPrivateQueue.Store(nSize)
}
