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

package Layout

import (
	"strings"
	"testing"

	"github.com/maxhaosl/cygologger/ICYLogger/Core"
)

// TestLayout6Resolution verifies the new built-in layout 6 resolves from the
// manager and produces the retrieval-required format where the channel bracket
// leads the func/line bracket (no extra space between them):
//
//	[Time][TYPE|P:pid|T:tid][channel][func(line)] Msg
func TestLayout6Resolution(t *testing.T) {
	m := GetCYLoggerTemplateLayoutManagerInstance()
	l := m.GetLayout(Core.LogLayoutTypeBuildin6)
	if l == nil {
		t.Fatal("GetLayout(Buildin6) returned nil")
	}
	if l.GetTypeIndex() != 6 {
		t.Fatalf("expected type index 6, got %d", l.GetTypeIndex())
	}

	// bEscape=false so the escape-filter branch is not exercised here.
	out := l.GetFormatMessage(
		"FLOW", Core.LogTypeInfo, -1,
		"predict exit : spm=\"mars-pipeline\"", "main.go", "handler.RegisterRoutes.predict.func3.1", 40,
		50359, 47,
		2026, 7, 29, 16, 14, 37, 342,
		false,
	)

	want := "[2026-07-29 16:14:37.342][I|P:50359|T:47][FLOW][handler.RegisterRoutes.predict.func3.1(40)] predict exit : spm=\"mars-pipeline\"\n"
	if out != want {
		t.Fatalf("unexpected layout6 output:\n got: %q\nwant: %q", out, want)
	}

	// A line with an empty channel must drop the channel bracket entirely.
	outEmpty := l.GetFormatMessage(
		"", Core.LogTypeInfo, -1,
		"hello world", "main.go", "main", 42,
		1234, 5678,
		2026, 7, 28, 11, 4, 21, 564,
		false,
	)
	if !strings.Contains(outEmpty, "[I|P:1234|T:5678][main(42)]") {
		t.Fatalf("empty-channel line missing type/func block: %q", outEmpty)
	}
	if strings.Contains(outEmpty, "][]") || strings.Contains(outEmpty, "[]") {
		t.Fatalf("empty-channel line must not render an empty channel bracket: %q", outEmpty)
	}
	if !strings.HasSuffix(outEmpty, "hello world\n") {
		t.Fatalf("unexpected message suffix: %q", outEmpty)
	}
}
