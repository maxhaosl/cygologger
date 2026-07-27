/*
 * CYGoLogger License
 * -----------
 *
 * CYGoLogger is licensed under the terms of the MIT license reproduced below.
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

package Statistics

import (
	"testing"

	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
)

// TestReset verifies that Reset() zeroes all cumulative counters (lines, bytes,
// FPS) while leaving queue-length reporting functional, mirroring C++
// CYStatistics::Reset().
func TestReset(t *testing.T) {
	s := GetCYStatisticsInstance()

	// Simulate some accumulated traffic.
	s.IncrementLine(Core.LogTypeInfo, 100)
	s.ReportQueueLengths(Core.LogTypeInfo, 5, 3)

	var before Core.STStatistics
	if !s.GetStats(&before) {
		t.Fatal("GetStats returned false")
	}
	if before.NInfoLine == 0 {
		t.Fatalf("NInfoLine = 0 before Reset, want > 0")
	}
	if before.NInfoPublicQueue != 5 || before.NInfoPrivateQueue != 3 {
		t.Fatalf("Info queue before Reset = (%d,%d), want (5,3)",
			before.NInfoPublicQueue, before.NInfoPrivateQueue)
	}

	// Reset must zero the cumulative counters.
	s.Reset()

	var after Core.STStatistics
	if !s.GetStats(&after) {
		t.Fatal("GetStats returned false")
	}
	if after.NInfoLine != 0 {
		t.Errorf("NInfoLine = %d after Reset, want 0", after.NInfoLine)
	}
	if after.NTotalLine != 0 {
		t.Errorf("NTotalLine = %d after Reset, want 0", after.NTotalLine)
	}

	// Queue-length reporting must keep working after a reset (the fields are
	// re-populated on the next appender tick, so they self-heal).
	s.ReportQueueLengths(Core.LogTypeInfo, 9, 8)
	var post Core.STStatistics
	if !s.GetStats(&post) {
		t.Fatal("GetStats returned false")
	}
	if post.NInfoPublicQueue != 9 || post.NInfoPrivateQueue != 8 {
		t.Errorf("Info queue after Reset = (%d,%d), want (9,8)",
			post.NInfoPublicQueue, post.NInfoPrivateQueue)
	}
}

// TestReportQueueLengths verifies that per-type queue lengths are surfaced in
// STStatistics, including the Console/Remote "D"-sub-queue slot used by the Go
// single-queue appenders (mirroring C++ Add*PublicQueue / Add*PrivateQueue).
func TestReportQueueLengths(t *testing.T) {
	s := GetCYStatisticsInstance()

	s.ReportQueueLengths(Core.LogTypeInfo, 5, 3)
	s.ReportQueueLengths(Core.LogTypeConsole, 7, 2)
	s.ReportQueueLengths(Core.LogTypeRemote, 11, 4)
	s.ReportQueueLengths(Core.LogTypeMain, 13, 6)

	var q Core.STStatistics
	if !s.GetStats(&q) {
		t.Fatal("GetStats returned false")
	}

	if q.NInfoPublicQueue != 5 || q.NInfoPrivateQueue != 3 {
		t.Errorf("Info queue = (%d,%d), want (5,3)", q.NInfoPublicQueue, q.NInfoPrivateQueue)
	}
	// Console/Remote/Main expose six per-subtype public queues in C++; the Go
	// single-queue appender reports into the Debug ("D") sub-queue slot.
	if q.NConsolePublicDQueue != 7 || q.NConsolePrivateQueue != 2 {
		t.Errorf("Console queue = (%d,%d), want (7,2)", q.NConsolePublicDQueue, q.NConsolePrivateQueue)
	}
	if q.NRemotePublicDQueue != 11 || q.NRemotePrivateQueue != 4 {
		t.Errorf("Remote queue = (%d,%d), want (11,4)", q.NRemotePublicDQueue, q.NRemotePrivateQueue)
	}
	if q.NMainPublicDQueue != 13 || q.NMainPrivateQueue != 6 {
		t.Errorf("Main queue = (%d,%d), want (13,6)", q.NMainPublicDQueue, q.NMainPrivateQueue)
	}
}
