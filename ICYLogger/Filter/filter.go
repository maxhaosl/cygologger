/*
 * cygologger License
 * -----------
 *
 * cygologger is licensed under the terms of the MIT license reproduced below.
 * This means that cygologger is free software and can be licensed for both academic
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

// Package Filter provides the pattern filtering interface and filter chain implementations.
package Filter

import (
	"strings"
	"sync"

	Common "github.com/maxhaosl/cygologger/ICYLogger/Common"
)

// Escape characters for log message parsing.
// These are re-exported from Common (the single source of truth) so that
// Filter, Layout and Common all share identical delimiters matching the C++
// version: header [ ], field name =, field value |, extension field value #.
const (
	LogEscape                 = Common.LogEscapeChar
	LogHeaderStart            = Common.LogHeaderStart
	LogHeaderEnd              = Common.LogHeaderEnd
	LogFieldNameEnd           = Common.LogFieldNameEnd
	LogFieldValueEnd          = Common.LogFieldValueEnd
	LogExtensionFieldValueEnd = Common.LogExtensionFieldValueEnd
)

// TupleFieldType represents a key-value filter pair.
type TupleFieldType struct {
	Key   string
	Value string
}

// ICYLoggerPatternFilter filters log messages, escaping special characters.
type ICYLoggerPatternFilter struct {
	nextFilter *ICYLoggerPatternFilter
	fields    []TupleFieldType
}

func NewPatternFilter() *ICYLoggerPatternFilter {
	return &ICYLoggerPatternFilter{fields: make([]TupleFieldType, 0)}
}

func (f *ICYLoggerPatternFilter) Add(key, value string) *ICYLoggerPatternFilter {
	for i := range f.fields {
		if f.fields[i].Key == key {
			if value == "" {
				f.fields = append(f.fields[:i], f.fields[i+1:]...)
			} else {
				f.fields[i].Value = value
			}
			return f
		}
	}
	f.fields = append(f.fields, TupleFieldType{Key: key, Value: value})
	return f
}

func (f *ICYLoggerPatternFilter) GetFields() []TupleFieldType {
	result := make([]TupleFieldType, len(f.fields))
	copy(result, f.fields)
	return result
}

func (f *ICYLoggerPatternFilter) SetNextFilter(pFilter *ICYLoggerPatternFilter) {
	if f.nextFilter == nil {
		f.nextFilter = pFilter
	} else {
		cur := f.nextFilter
		for cur.nextFilter != nil {
			cur = cur.nextFilter
		}
		cur.nextFilter = pFilter
	}
}

func (f *ICYLoggerPatternFilter) GetNextFilter() *ICYLoggerPatternFilter {
	return f.nextFilter
}

func (f *ICYLoggerPatternFilter) FilterRequest(strMsg *string,
	delimiters string, escapeChar rune, cFieldNameEnd, cFieldValueEnd rune) string {

	var sb strings.Builder
	for _, tp := range f.fields {
		s := f.escape(*strMsg, tp.Key, delimiters, escapeChar)
		sb.WriteString(s)
		sb.WriteRune(cFieldNameEnd)
		s = f.escape(*strMsg, tp.Value, delimiters, escapeChar)
		sb.WriteString(s)
		sb.WriteRune(cFieldValueEnd)
	}

	if f.nextFilter != nil {
		sb.WriteString(f.nextFilter.FilterRequest(strMsg, delimiters, escapeChar, cFieldNameEnd, cFieldValueEnd))
	}
	return sb.String()
}

func (f *ICYLoggerPatternFilter) escape(strRes, strSrc, delimiters string, escapeChar rune) string {
	if strRes != strSrc {
		strRes = strSrc
	}
	runes := []rune(strRes)
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

// CYLoggerPatternFilterChain manages a chain of pattern filters.
type CYLoggerPatternFilterChain struct {
	Common.CYNoCopy
	mu   sync.RWMutex
	head *ICYLoggerPatternFilter
	tail *ICYLoggerPatternFilter
}

var g_CYLoggerPatternFilterChainInstance *CYLoggerPatternFilterChain
var g_CYLoggerPatternFilterChainOnce sync.Once

func GetCYLoggerPatternFilterChainInstance() *CYLoggerPatternFilterChain {
	g_CYLoggerPatternFilterChainOnce.Do(func() {
		g_CYLoggerPatternFilterChainInstance = &CYLoggerPatternFilterChain{}
	})
	return g_CYLoggerPatternFilterChainInstance
}

func (c *CYLoggerPatternFilterChain) SetHeadFilter(pFilter *ICYLoggerPatternFilter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.head = pFilter
	if pFilter != nil {
		t := pFilter
		for t.GetNextFilter() != nil {
			t = t.GetNextFilter()
		}
		c.tail = t
	} else {
		c.tail = nil
	}
}

func (c *CYLoggerPatternFilterChain) GetHeadFilter() *ICYLoggerPatternFilter {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.head
}

func (c *CYLoggerPatternFilterChain) AppendFilter(pFilter *ICYLoggerPatternFilter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.head == nil {
		c.head = pFilter
		c.tail = pFilter
		return
	}
	c.tail.SetNextFilter(pFilter)
	c.tail = pFilter
}

func (c *CYLoggerPatternFilterChain) ClearFilters() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.head = nil
	c.tail = nil
}

// CYLoggerPatternFilterManager manages the global filter instance.
type CYLoggerPatternFilterManager struct {
	Common.CYNoCopy
	mu     sync.RWMutex
	filter *ICYLoggerPatternFilter
}

var g_CYLoggerPatternFilterManagerInstance *CYLoggerPatternFilterManager
var g_CYLoggerPatternFilterManagerOnce sync.Once

func GetCYLoggerPatternFilterManagerInstance() *CYLoggerPatternFilterManager {
	g_CYLoggerPatternFilterManagerOnce.Do(func() {
		pf := NewPatternFilter()
		pf.Add("Channel", "Message")
		g_CYLoggerPatternFilterManagerInstance = &CYLoggerPatternFilterManager{filter: pf}
	})
	return g_CYLoggerPatternFilterManagerInstance
}

func (m *CYLoggerPatternFilterManager) GetFilter() *ICYLoggerPatternFilter {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.filter
}

func (m *CYLoggerPatternFilterManager) SetFilter(pFilter *ICYLoggerPatternFilter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filter = pFilter
}

func (m *CYLoggerPatternFilterManager) CreateDefaultFilter() *ICYLoggerPatternFilter {
	return NewPatternFilter()
}
