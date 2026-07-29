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

// Package layout provides layout implementations for formatting log messages.
package Layout

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/maxhaosl/cygologger/ICYLogger/Common"
	"github.com/maxhaosl/cygologger/ICYLogger/Core"
	"github.com/maxhaosl/cygologger/ICYLogger/Filter"
)

// builderPool reuses strings.Builder instances across GetFormatMessage calls to
// keep per-line heap allocations minimal. The final sb.String() copies the bytes
// into an immutable string, which does NOT alias the pooled buffer, so returning
// it after the builder is Put back is safe.
var builderPool = sync.Pool{New: func() any { return &strings.Builder{} }}

// appendUint / appendInt write an integer directly into b via strconv.AppendXxx
// over a stack scratch buffer, so no intermediate string is allocated (strconv's
// Sprintf/Itoa variants would each heap-allocate a string on every log line).
func appendUint(b *strings.Builder, v uint64) {
	var tmp [20]byte
	b.Write(strconv.AppendUint(tmp[:0], v, 10))
}

func appendInt(b *strings.Builder, v int) {
	var tmp [20]byte
	b.Write(strconv.AppendInt(tmp[:0], int64(v), 10))
}

// appendTimestamp writes "YYYY-MM-DD HH:MM:SS.mmm" into b using a stack scratch
// buffer, replacing the per-call fmt.Sprintf that GetTimeStamps used (which
// allocated a string on every log line). This alone removes one of the largest
// steady-state allocations on the hot path.
func appendTimestamp(b *strings.Builder, nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) {
	var buf [32]byte
	n := appendPadded(buf[:], nYY, 4)
	buf[n] = '-'
	n++
	n += appendPadded(buf[n:], nMM, 2)
	buf[n] = '-'
	n++
	n += appendPadded(buf[n:], nDD, 2)
	buf[n] = ' '
	n++
	n += appendPadded(buf[n:], nHR, 2)
	buf[n] = ':'
	n++
	n += appendPadded(buf[n:], nMN, 2)
	buf[n] = ':'
	n++
	n += appendPadded(buf[n:], nSC, 2)
	buf[n] = '.'
	n++
	n += appendPadded(buf[n:], nMMN, 3)
	b.Write(buf[:n])
}

// appendPadded writes v zero-padded to width digits into buf and returns the
// number of bytes written (always == width).
func appendPadded(buf []byte, v, width int) int {
	var tmp [20]byte
	s := strconv.AppendInt(tmp[:0], int64(v), 10)
	k := 0
	for k < width-len(s) {
		buf[k] = '0'
		k++
	}
	return k + copy(buf[k:], s)
}

// CYLoggerTemplateLayoutEscape provides escape character handling for layouts.
type CYLoggerTemplateLayoutEscape struct{}

func (le *CYLoggerTemplateLayoutEscape) GetEscapeChar() rune {
	return Common.LogEscapeChar
}

func (le *CYLoggerTemplateLayoutEscape) GetDelimiters() string {
	return string([]rune{
		Common.LogHeaderStart,
		Common.LogHeaderEnd,
		Common.LogFieldNameEnd,
		Common.LogFieldValueEnd,
		Common.LogExtensionFieldValueEnd,
	})
}

func (le *CYLoggerTemplateLayoutEscape) EscapeString(src string) string {
	delimiters := le.GetDelimiters()
	escapeChar := le.GetEscapeChar()
	runes := []rune(src)
	result := make([]rune, 0, len(runes)*2)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c == escapeChar {
			result = append(result, escapeChar, escapeChar)
			continue
		}
		for _, d := range delimiters {
			if c == d {
				result = append(result, escapeChar)
				break
			}
		}
		result = append(result, c)
	}
	return string(result)
}

func (le *CYLoggerTemplateLayoutEscape) WriteEscapeString(sb *strings.Builder, strMsg string) {
	sb.WriteString(le.EscapeString(strMsg))
}

// CYLoggerTemplateLayout1 is built-in layout: [HH:MM:SS.mmm][Type][PID][TID][Msg]
type CYLoggerTemplateLayout1 struct {
	CYLoggerTemplateLayoutEscape
	escape *Filter.ICYLoggerPatternFilter
}

func NewCYLoggerTemplateLayout1() *CYLoggerTemplateLayout1 {
	return &CYLoggerTemplateLayout1{}
}

func (l *CYLoggerTemplateLayout1) SetFilter(f *Filter.ICYLoggerPatternFilter) {
	l.escape = f
}

func (l *CYLoggerTemplateLayout1) GetTypeIndex() int32 { return 1 }

func (l *CYLoggerTemplateLayout1) GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d.%03d", nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
}

func (l *CYLoggerTemplateLayout1) GetFormatMessage(strChannel string, eMsgType Core.ELogType, nSeverCode int,
	strMsg, strFile, strFunction string,
	nLine int, nProcessId, nThreadId uint64,
	nYY, nMM, nDD, nHR, nMN, nSC, nMMN int,
	bEscape bool) string {

	sb := builderPool.Get().(*strings.Builder)
	sb.Reset()
	sb.Grow(256)
	hs := Common.LogHeaderStart
	he := Common.LogHeaderEnd
	fv := Common.LogFieldValueEnd
	fn := Common.LogFieldNameEnd
	ev := Common.LogExtensionFieldValueEnd

	// [Time]
	sb.WriteRune(hs)
	// Buildin3 keeps the SHORT time format ("HH:MM:SS", no date, no ms) — its
	// GetTimeStamps is deliberately different from the full-date layouts 1/2/4,
	// so it must use the layout's own formatter rather than appendTimestamp.
	sb.WriteString(l.GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN))
	sb.WriteRune(he)

	// |Type[:SeverCode]
	sb.WriteRune(fv)
	if nSeverCode >= 0 {
		sb.WriteString(layoutTypeCode(eMsgType))
		sb.WriteRune(':')
		appendInt(sb, nSeverCode)
	} else {
		sb.WriteString(layoutTypeCode(eMsgType))
	}

	// |P:pid
	sb.WriteRune(fv)
	sb.WriteString("P:")
	appendUint(sb, nProcessId)

	// |T:tid
	sb.WriteRune(fv)
	sb.WriteString("T:")
	appendUint(sb, nThreadId)

	// |Key=Value#... (extension field via the pattern filter)
	if bEscape && l.escape != nil {
		if ext := l.escape.FilterRequest(&strMsg, l.GetDelimiters(), l.GetEscapeChar(), fn, ev); ext != "" {
			sb.WriteRune(fv)
			sb.WriteString(ext)
		}
	}

	// |file::func(line)]
	sb.WriteRune(fv)
	sb.WriteString(layoutBaseName(strFile))
	sb.WriteString("::")
	sb.WriteString(strFunction)
	sb.WriteRune('(')
	appendInt(sb, nLine)
	sb.WriteRune(')')
	sb.WriteRune(he)

	// leading space before the message body
	sb.WriteRune(' ')

	if strChannel != "" {
		sb.WriteRune(hs)
		sb.WriteString("Channel:")
		sb.WriteString(strChannel)
		sb.WriteRune(he)
	}

	sb.WriteString(strMsg)
	sb.WriteString("\n")
	out := sb.String()
	builderPool.Put(sb)
	return out
}

func (l *CYLoggerTemplateLayout1) typeName(e Core.ELogType) string {
	return layoutTypeCode(e)
}

// CYLoggerTemplateLayout2 is built-in layout with file/line/func info.
type CYLoggerTemplateLayout2 struct {
	CYLoggerTemplateLayoutEscape
	escape *Filter.ICYLoggerPatternFilter
}

func NewCYLoggerTemplateLayout2() *CYLoggerTemplateLayout2 {
	return &CYLoggerTemplateLayout2{}
}

func (l *CYLoggerTemplateLayout2) SetFilter(f *Filter.ICYLoggerPatternFilter) {
	l.escape = f
}

func (l *CYLoggerTemplateLayout2) GetTypeIndex() int32 { return 2 }

func (l *CYLoggerTemplateLayout2) GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d.%03d", nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
}

func (l *CYLoggerTemplateLayout2) GetFormatMessage(strChannel string, eMsgType Core.ELogType, nSeverCode int,
	strMsg, strFile, strFunction string,
	nLine int, nProcessId, nThreadId uint64,
	nYY, nMM, nDD, nHR, nMN, nSC, nMMN int,
	bEscape bool) string {

	sb := builderPool.Get().(*strings.Builder)
	sb.Reset()
	sb.Grow(256)
	hs := Common.LogHeaderStart
	he := Common.LogHeaderEnd
	fv := Common.LogFieldValueEnd
	fn := Common.LogFieldNameEnd
	ev := Common.LogExtensionFieldValueEnd

	// [Time]
	sb.WriteRune(hs)
	appendTimestamp(sb, nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
	sb.WriteRune(he)

	// [Type[:SeverCode]]
	sb.WriteRune(hs)
	if nSeverCode >= 0 {
		sb.WriteString(layoutTypeCode(eMsgType))
		sb.WriteRune(':')
		appendInt(sb, nSeverCode)
	} else {
		sb.WriteString(layoutTypeCode(eMsgType))
	}
	sb.WriteRune(he)

	// |P:pid
	sb.WriteRune(fv)
	sb.WriteString("P:")
	appendUint(sb, nProcessId)

	// |T:tid]
	sb.WriteRune(fv)
	sb.WriteString("T:")
	appendUint(sb, nThreadId)
	sb.WriteRune(he)

	// [Key=Value#...] (extension field)
	if bEscape && l.escape != nil {
		if ext := l.escape.FilterRequest(&strMsg, l.GetDelimiters(), l.GetEscapeChar(), fn, ev); ext != "" {
			sb.WriteRune(hs)
			sb.WriteString(ext)
			sb.WriteRune(he)
		}
	}

	// [func(line)]
	sb.WriteRune(hs)
	sb.WriteString(strFunction)
	sb.WriteRune('(')
	appendInt(sb, nLine)
	sb.WriteRune(')')
	sb.WriteRune(he)
	sb.WriteRune(' ')

	if strChannel != "" {
		sb.WriteRune(hs)
		sb.WriteString("Channel:")
		sb.WriteString(strChannel)
		sb.WriteRune(he)
	}

	sb.WriteString(strMsg)
	sb.WriteString("\n")
	out := sb.String()
	builderPool.Put(sb)
	return out
}

func (l *CYLoggerTemplateLayout2) typeName(e Core.ELogType) string {
	return layoutTypeCode(e)
}

// CYLoggerTemplateLayout3 is built-in layout with channel and minimal info.
type CYLoggerTemplateLayout3 struct {
	CYLoggerTemplateLayoutEscape
	escape *Filter.ICYLoggerPatternFilter
}

func NewCYLoggerTemplateLayout3() *CYLoggerTemplateLayout3 {
	return &CYLoggerTemplateLayout3{}
}

func (l *CYLoggerTemplateLayout3) SetFilter(f *Filter.ICYLoggerPatternFilter) {
	l.escape = f
}

func (l *CYLoggerTemplateLayout3) GetTypeIndex() int32 { return 3 }

func (l *CYLoggerTemplateLayout3) GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	return fmt.Sprintf("%02d:%02d:%02d", nHR, nMN, nSC)
}

func (l *CYLoggerTemplateLayout3) GetFormatMessage(strChannel string, eMsgType Core.ELogType, nSeverCode int,
	strMsg, strFile, strFunction string,
	nLine int, nProcessId, nThreadId uint64,
	nYY, nMM, nDD, nHR, nMN, nSC, nMMN int,
	bEscape bool) string {

	sb := builderPool.Get().(*strings.Builder)
	sb.Reset()
	sb.Grow(256)
	hs := Common.LogHeaderStart
	he := Common.LogHeaderEnd
	fv := Common.LogFieldValueEnd
	fn := Common.LogFieldNameEnd
	ev := Common.LogExtensionFieldValueEnd

	// [Time]
	sb.WriteRune(hs)
	// Buildin3 keeps the SHORT time format ("HH:MM:SS", no date, no ms) — its
	// GetTimeStamps is deliberately different from the full-date layouts 1/2/4,
	// so it must use the layout's own formatter rather than appendTimestamp.
	sb.WriteString(l.GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN))
	sb.WriteRune(he)

	// |Type[:SeverCode]
	sb.WriteRune(fv)
	if nSeverCode >= 0 {
		sb.WriteString(layoutTypeCode(eMsgType))
		sb.WriteRune(':')
		appendInt(sb, nSeverCode)
	} else {
		sb.WriteString(layoutTypeCode(eMsgType))
	}

	// |P:pid
	sb.WriteRune(fv)
	sb.WriteString("P:")
	appendUint(sb, nProcessId)

	// |T:tid
	sb.WriteRune(fv)
	sb.WriteString("T:")
	appendUint(sb, nThreadId)

	// |Key=Value#... (extension field)
	if bEscape && l.escape != nil {
		if ext := l.escape.FilterRequest(&strMsg, l.GetDelimiters(), l.GetEscapeChar(), fn, ev); ext != "" {
			sb.WriteRune(fv)
			sb.WriteString(ext)
		}
	}

	// |func(line)]
	sb.WriteRune(fv)
	sb.WriteString(strFunction)
	sb.WriteRune('(')
	appendInt(sb, nLine)
	sb.WriteRune(')')
	sb.WriteRune(he)

	if strChannel != "" {
		sb.WriteRune(hs)
		sb.WriteString("Channel:")
		sb.WriteString(strChannel)
		sb.WriteRune(he)
	}

	if nSeverCode >= 0 {
		sb.WriteRune(hs)
		sb.WriteString("ServerCode:")
		appendInt(sb, nSeverCode)
		sb.WriteRune(he)
	}

	sb.WriteString(strMsg)

	//  - [file(line)]
	sb.WriteString(" - ")
	sb.WriteRune(hs)
	sb.WriteString(layoutBaseName(strFile))
	sb.WriteRune('(')
	appendInt(sb, nLine)
	sb.WriteRune(')')
	sb.WriteRune(he)

	sb.WriteString("\n")
	out := sb.String()
	builderPool.Put(sb)
	return out
}

func (l *CYLoggerTemplateLayout3) typeName(e Core.ELogType) string {
	return layoutTypeCode(e)
}

// CYLoggerTemplateLayout4 is built-in layout 4: [Time][Type|P:pid|T:tid][func(line)] Msg.
// It mirrors the C++ CYLoggerTemplateLayout4: the type block groups the single
// type-code letter with the process/thread ids inside one bracket pair, separated
// by the field-value delimiter '|', followed by optional extension-field and
// channel brackets and a [func(line)] bracket before the message body.
type CYLoggerTemplateLayout4 struct {
	CYLoggerTemplateLayoutEscape
	escape *Filter.ICYLoggerPatternFilter
}

func NewCYLoggerTemplateLayout4() *CYLoggerTemplateLayout4 {
	return &CYLoggerTemplateLayout4{}
}

func (l *CYLoggerTemplateLayout4) SetFilter(f *Filter.ICYLoggerPatternFilter) {
	l.escape = f
}

func (l *CYLoggerTemplateLayout4) GetTypeIndex() int32 { return 4 }

func (l *CYLoggerTemplateLayout4) GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d.%03d", nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
}

func (l *CYLoggerTemplateLayout4) GetFormatMessage(strChannel string, eMsgType Core.ELogType, nSeverCode int,
	strMsg, strFile, strFunction string,
	nLine int, nProcessId, nThreadId uint64,
	nYY, nMM, nDD, nHR, nMN, nSC, nMMN int,
	bEscape bool) string {

	sb := builderPool.Get().(*strings.Builder)
	sb.Reset()
	sb.Grow(256)
	hs := Common.LogHeaderStart
	he := Common.LogHeaderEnd
	fv := Common.LogFieldValueEnd
	fn := Common.LogFieldNameEnd
	ev := Common.LogExtensionFieldValueEnd

	// [Time]
	sb.WriteRune(hs)
	appendTimestamp(sb, nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
	sb.WriteRune(he)

	// [Type[:SeverCode]|P:pid|T:tid]
	sb.WriteRune(hs)
	sb.WriteString(layoutTypeCode(eMsgType))
	if nSeverCode >= 0 {
		sb.WriteRune(':')
		appendInt(sb, nSeverCode)
	}
	sb.WriteRune(fv)
	sb.WriteString("P:")
	appendUint(sb, nProcessId)
	sb.WriteRune(fv)
	sb.WriteString("T:")
	appendUint(sb, nThreadId)
	sb.WriteRune(he)

	// [Key=Value#...] (extension field)
	if bEscape && l.escape != nil {
		if ext := l.escape.FilterRequest(&strMsg, l.GetDelimiters(), l.GetEscapeChar(), fn, ev); ext != "" {
			sb.WriteRune(hs)
			sb.WriteString(ext)
			sb.WriteRune(he)
		}
	}

	// [Channel]
	if strChannel != "" {
		sb.WriteRune(hs)
		sb.WriteString(strChannel)
		sb.WriteRune(he)
	}

	// [func(line)]
	sb.WriteRune(hs)
	sb.WriteString(strFunction)
	sb.WriteRune('(')
	appendInt(sb, nLine)
	sb.WriteRune(')')
	sb.WriteRune(he)
	sb.WriteRune(' ')

	sb.WriteString(strMsg)
	sb.WriteString("\n")
	out := sb.String()
	builderPool.Put(sb)
	return out
}

func (l *CYLoggerTemplateLayout4) typeName(e Core.ELogType) string {
	return layoutTypeCode(e)
}

// CYLoggerTemplateLayout5 is a custom layout: it mirrors
// Buildin4 ([Time][TYPE|P:pid|T:tid][func(line)] Msg) but renders the channel
// as a clearly labelled [CH:name] field. This makes it unambiguous which
// subsystem produced a line — a submodule or SDK log carries [CH:name], while
// the service program's own logs carry no [CH:...] bracket at all.
type CYLoggerTemplateLayout5 struct {
	CYLoggerTemplateLayoutEscape
	escape *Filter.ICYLoggerPatternFilter
}

func NewCYLoggerTemplateLayout5() *CYLoggerTemplateLayout5 {
	return &CYLoggerTemplateLayout5{}
}

func (l *CYLoggerTemplateLayout5) SetFilter(f *Filter.ICYLoggerPatternFilter) {
	l.escape = f
}

func (l *CYLoggerTemplateLayout5) GetTypeIndex() int32 { return 5 }

func (l *CYLoggerTemplateLayout5) GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d.%03d", nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
}

func (l *CYLoggerTemplateLayout5) GetFormatMessage(strChannel string, eMsgType Core.ELogType, nSeverCode int,
	strMsg, strFile, strFunction string,
	nLine int, nProcessId, nThreadId uint64,
	nYY, nMM, nDD, nHR, nMN, nSC, nMMN int,
	bEscape bool) string {

	sb := builderPool.Get().(*strings.Builder)
	sb.Reset()
	sb.Grow(256)
	hs := Common.LogHeaderStart
	he := Common.LogHeaderEnd
	fv := Common.LogFieldValueEnd

	// [Time]
	sb.WriteRune(hs)
	appendTimestamp(sb, nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
	sb.WriteRune(he)

	// [Type[:SeverCode]|P:pid|T:tid]
	sb.WriteRune(hs)
	sb.WriteString(layoutTypeCode(eMsgType))
	if nSeverCode >= 0 {
		sb.WriteRune(':')
		appendInt(sb, nSeverCode)
	}
	sb.WriteRune(fv)
	sb.WriteString("P:")
	appendUint(sb, nProcessId)
	sb.WriteRune(fv)
	sb.WriteString("T:")
	appendUint(sb, nThreadId)
	sb.WriteRune(he)

	// [CH:channel] — only when a channel is set; a missing bracket means the
	// service program itself wrote the line (vs a submodule or SDK).
	if strChannel != "" {
		sb.WriteRune(hs)
		sb.WriteString("CH:")
		sb.WriteString(strChannel)
		sb.WriteRune(he)
	}

	// [func(line)]
	sb.WriteRune(hs)
	sb.WriteString(strFunction)
	sb.WriteRune('(')
	appendInt(sb, nLine)
	sb.WriteRune(')')
	sb.WriteRune(he)
	sb.WriteRune(' ')

	sb.WriteString(strMsg)
	sb.WriteString("\n")
	out := sb.String()
	builderPool.Put(sb)
	return out
}

func (l *CYLoggerTemplateLayout5) typeName(e Core.ELogType) string {
	return layoutTypeCode(e)
}

// CYLoggerTemplateLayout6 is the retrieval-preferred layout. It mirrors
// Buildin4 ([Time][TYPE|P:pid|T:tid][func(line)] Msg) but renders the channel
// as a bare [name] bracket placed BEFORE [func(line)] instead of after it:
//
//	[Time][TYPE|P:pid|T:tid][channel][func(line)] Msg
//
// This matches the retrieval project's required log format where the channel
// (e.g. [FLOW]) leads the func/line bracket so a subsystem's channel is visible
// before the caller location.
type CYLoggerTemplateLayout6 struct {
	CYLoggerTemplateLayoutEscape
	escape *Filter.ICYLoggerPatternFilter
}

func NewCYLoggerTemplateLayout6() *CYLoggerTemplateLayout6 {
	return &CYLoggerTemplateLayout6{}
}

func (l *CYLoggerTemplateLayout6) SetFilter(f *Filter.ICYLoggerPatternFilter) {
	l.escape = f
}

func (l *CYLoggerTemplateLayout6) GetTypeIndex() int32 { return 6 }

func (l *CYLoggerTemplateLayout6) GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d.%03d", nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
}

func (l *CYLoggerTemplateLayout6) GetFormatMessage(strChannel string, eMsgType Core.ELogType, nSeverCode int,
	strMsg, strFile, strFunction string,
	nLine int, nProcessId, nThreadId uint64,
	nYY, nMM, nDD, nHR, nMN, nSC, nMMN int,
	bEscape bool) string {

	sb := builderPool.Get().(*strings.Builder)
	sb.Reset()
	sb.Grow(256)
	hs := Common.LogHeaderStart
	he := Common.LogHeaderEnd
	fv := Common.LogFieldValueEnd
	fn := Common.LogFieldNameEnd
	ev := Common.LogExtensionFieldValueEnd

	// [Time]
	sb.WriteRune(hs)
	appendTimestamp(sb, nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
	sb.WriteRune(he)

	// [Type[:SeverCode]|P:pid|T:tid]
	sb.WriteRune(hs)
	sb.WriteString(layoutTypeCode(eMsgType))
	if nSeverCode >= 0 {
		sb.WriteRune(':')
		appendInt(sb, nSeverCode)
	}
	sb.WriteRune(fv)
	sb.WriteString("P:")
	appendUint(sb, nProcessId)
	sb.WriteRune(fv)
	sb.WriteString("T:")
	appendUint(sb, nThreadId)
	sb.WriteRune(he)

	// [Key=Value#...] (extension field)
	if bEscape && l.escape != nil {
		if ext := l.escape.FilterRequest(&strMsg, l.GetDelimiters(), l.GetEscapeChar(), fn, ev); ext != "" {
			sb.WriteRune(hs)
			sb.WriteString(ext)
			sb.WriteRune(he)
		}
	}

	// [channel] — rendered BEFORE [func(line)] so the subsystem channel leads
	// the caller location (this is the retrieval-required ordering).
	if strChannel != "" {
		sb.WriteRune(hs)
		sb.WriteString(strChannel)
		sb.WriteRune(he)
	}

	// [func(line)]
	sb.WriteRune(hs)
	sb.WriteString(strFunction)
	sb.WriteRune('(')
	appendInt(sb, nLine)
	sb.WriteRune(')')
	sb.WriteRune(he)
	sb.WriteRune(' ')

	sb.WriteString(strMsg)
	sb.WriteString("\n")
	out := sb.String()
	builderPool.Put(sb)
	return out
}

func (l *CYLoggerTemplateLayout6) typeName(e Core.ELogType) string {
	return layoutTypeCode(e)
}

// CYLoggerTemplateLayoutCustom wraps a user-provided layout.
type CYLoggerTemplateLayoutCustom struct {
	CYLoggerTemplateLayoutEscape
	inner  ICYLoggerTemplateLayout
	escape *Filter.ICYLoggerPatternFilter
}

func NewCYLoggerTemplateLayoutCustom(inner ICYLoggerTemplateLayout) *CYLoggerTemplateLayoutCustom {
	return &CYLoggerTemplateLayoutCustom{inner: inner}
}

func (l *CYLoggerTemplateLayoutCustom) SetFilter(f *Filter.ICYLoggerPatternFilter) {
	l.escape = f
	if l.inner != nil {
		if s, ok := l.inner.(interface {
			SetFilter(*Filter.ICYLoggerPatternFilter)
		}); ok {
			s.SetFilter(f)
		}
	}
}

func (l *CYLoggerTemplateLayoutCustom) GetTypeIndex() int32 {
	if l.inner != nil {
		return l.inner.GetTypeIndex()
	}
	return 0
}

func (l *CYLoggerTemplateLayoutCustom) GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN int) string {
	if l.inner != nil {
		return l.inner.GetTimeStamps(nYY, nMM, nDD, nHR, nMN, nSC, nMMN)
	}
	return ""
}

func (l *CYLoggerTemplateLayoutCustom) GetFormatMessage(strChannel string, eMsgType Core.ELogType, nSeverCode int,
	strMsg, strFile, strFunction string,
	nLine int, nProcessId, nThreadId uint64,
	nYY, nMM, nDD, nHR, nMN, nSC, nMMN int,
	bEscape bool) string {

	if l.inner != nil {
		return l.inner.GetFormatMessage(strChannel, eMsgType, nSeverCode,
			strMsg, strFile, strFunction, nLine, nProcessId, nThreadId,
			nYY, nMM, nDD, nHR, nMN, nSC, nMMN, bEscape)
	}
	return strMsg + "\n"
}

// CYLoggerTemplateLayoutManager manages layout instances.
type CYLoggerTemplateLayoutManager struct {
	Common.CYNoCopy
	mu       sync.RWMutex
	layouts  map[int32]ICYLoggerTemplateLayout
	default_ ICYLoggerTemplateLayout
}

var g_CYLoggerTemplateLayoutManagerInstance *CYLoggerTemplateLayoutManager
var g_CYLoggerTemplateLayoutManagerOnce sync.Once

func GetCYLoggerTemplateLayoutManagerInstance() *CYLoggerTemplateLayoutManager {
	g_CYLoggerTemplateLayoutManagerOnce.Do(func() {
		m := &CYLoggerTemplateLayoutManager{
			layouts: make(map[int32]ICYLoggerTemplateLayout),
		}
		m.layouts[1] = NewCYLoggerTemplateLayout1()
		m.layouts[2] = NewCYLoggerTemplateLayout2()
		m.layouts[3] = NewCYLoggerTemplateLayout3()
		m.layouts[4] = NewCYLoggerTemplateLayout4()
		m.layouts[5] = NewCYLoggerTemplateLayout5()
		m.layouts[6] = NewCYLoggerTemplateLayout6()
		m.default_ = m.layouts[1]
		g_CYLoggerTemplateLayoutManagerInstance = m
	})
	return g_CYLoggerTemplateLayoutManagerInstance
}

func (m *CYLoggerTemplateLayoutManager) GetLayout(eLayoutType Core.ELogLayoutType) ICYLoggerTemplateLayout {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if eLayoutType == Core.LogLayoutTypeCustom {
		return m.default_
	}
	if l, ok := m.layouts[int32(eLayoutType)]; ok {
		return l
	}
	return m.default_
}

func (m *CYLoggerTemplateLayoutManager) SetLayout(eLayoutType Core.ELogLayoutType, pLayout ICYLoggerTemplateLayout) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if eLayoutType == Core.LogLayoutTypeCustom {
		m.default_ = pLayout
	} else {
		m.layouts[int32(eLayoutType)] = pLayout
	}
}

func (m *CYLoggerTemplateLayoutManager) RegisterLayout(nIndex int32, pLayout ICYLoggerTemplateLayout) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.layouts[nIndex] = pLayout
}

// layoutTypeCode maps a log type to its single-character C++ code used in all
// built-in layouts (T/D/I/W/E/F, plus M/R/S for Main/Remote/Sys).
func layoutTypeCode(e Core.ELogType) string {
	switch e {
	case Core.LogTypeTrace:
		return "T"
	case Core.LogTypeDebug:
		return "D"
	case Core.LogTypeInfo:
		return "I"
	case Core.LogTypeWarn:
		return "W"
	case Core.LogTypeError:
		return "E"
	case Core.LogTypeFatal:
		return "F"
	case Core.LogTypeMain:
		return "M"
	case Core.LogTypeRemote:
		return "R"
	case Core.LogTypeSys:
		return "S"
	default:
		return "U"
	}
}

// layoutBaseName returns the file base name, mirroring the C++ GetFileName used
// by the built-in layouts.
func layoutBaseName(s string) string {
	return filepath.Base(s)
}
