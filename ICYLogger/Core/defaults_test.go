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

import "testing"

// TestCPPDefaultConstantsMatch verifies that the Go default restriction
// constants exactly mirror the C++ ICYLoggerDefine values supplied by the user:
//
//	LOG_LIMIT_ENABLE            = true
//	LOG_LIMIT_CLEAR_UNLOGFILE   = true
//	LOG_TIME_CLEAR_LOG          = 60
//	LOG_TIME_EXPIRED_FILE       = 24
//	LOG_CHECK_FILE_SIZE_TIME    = 60 * 5
//	LOG_CHECK_FILE_COUNT_TIME   = 60
//	LOG_CHECK_FILE_SIZE         = 1024 * 1024 * 5
//	LOG_COUNT_PER_TYPE          = 20
//	LOG_CHECK_FILE_TYPE_SIZE    = 1024 * 1024 * 500
//	LOG_CHECK_FILE_ALL_SIZE     = 1024 * 1024 * 1024
func TestCPPDefaultConstantsMatch(t *testing.T) {
	cases := []struct {
		name string
		got  any
		want any
	}{
		{"LOG_LIMIT_ENABLE", DefaultLogLimitEnable, true},
		{"LOG_LIMIT_CLEAR_UNLOGFILE", DefaultLogLimitClearUnLogFile, true},
		{"LOG_TIME_CLEAR_LOG", DefaultLogTimeClearLog, 60},
		{"LOG_TIME_EXPIRED_FILE", DefaultLogTimeExpiredFile, 24},
		{"LOG_CHECK_FILE_SIZE_TIME", DefaultLogCheckFileSizeTime, 60 * 5},
		{"LOG_CHECK_FILE_COUNT_TIME", DefaultLogCheckFileCountTime, 60},
		{"LOG_CHECK_FILE_SIZE", DefaultLogCheckFileSize, 1024 * 1024 * 5},
		{"LOG_COUNT_PER_TYPE", DefaultLogCountPerType, 20},
		{"LOG_CHECK_FILE_TYPE_SIZE", DefaultLogCheckFileTypeSize, 1024 * 1024 * 500},
		{"LOG_CHECK_FILE_ALL_SIZE", DefaultLogCheckAllFileSize, 1024 * 1024 * 1024},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}
