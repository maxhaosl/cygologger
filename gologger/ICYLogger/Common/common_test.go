/*
 * CYGoLogger License
 * -----------
 *
 * CYGoLogger is licensed under the terms of the MIT license reproduced below.
 * This means that CYGoLogger is free software and can be used for both academic
 * and commercial purposes at absolutely no cost.
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

package Common

import "testing"

// TestPublicFunctionPathUtils verifies the C++-aligned path helpers.
func TestPublicFunctionPathUtils(t *testing.T) {
	pf := GetCYPublicFunctionInstance()
	const p = "/var/log/Info_channel_20240101_120000.log"

	if got := pf.GetFileName(p); got != "Info_channel_20240101_120000" {
		t.Errorf("GetFileName = %q, want %q", got, "Info_channel_20240101_120000")
	}
	if got := pf.GetFileExt(p); got != "log" {
		t.Errorf("GetFileExt = %q, want %q", got, "log")
	}
	if got := pf.GetBaseLogName(p); got != "Info_channel_20240101" {
		t.Errorf("GetBaseLogName = %q, want %q", got, "Info_channel_20240101")
	}
	if got := pf.GetBasePath(p); got != "/var/log/Info_channel_20240101" {
		t.Errorf("GetBasePath = %q, want %q", got, "/var/log/Info_channel_20240101")
	}
}

// TestRdtsc verifies the rdtsc equivalent returns monotonically increasing
// timestamps.
func TestRdtsc(t *testing.T) {
	a := Rdtsc()
	b := Rdtsc()
	if b < a {
		t.Errorf("Rdtsc not monotonic: a=%d b=%d", a, b)
	}
	if a <= 0 {
		t.Errorf("Rdtsc returned %d, want > 0", a)
	}
}

// TestMessageTypePools verifies the three C++-aligned message types are
// acquirable and releasable through their dedicated pools.
func TestMessageTypePools(t *testing.T) {
	n := AcquireNormalMessage()
	ReleaseNormalMessage(n)
	e := AcquireEscapeMessage()
	ReleaseEscapeMessage(e)
	s := AcquireStrMessage()
	ReleaseStrMessage(s)
}
