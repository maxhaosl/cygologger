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

// Package Entity provides the logger entity and entity factory.
package Entity

import (
	"sync"

	"github.com/maxhaosl/CYGoLogger/ICYLogger/Core"
	Common "github.com/maxhaosl/CYGoLogger/ICYLogger/Common"
	"github.com/maxhaosl/CYGoLogger/ICYLogger/Appender"
)

// CYLoggerEntity manages appenders for a specific log type.
type CYLoggerEntity struct {
	Common.CYNoCopy
	mu        sync.RWMutex
	eLogType  Core.ELogType
	appenders []Appender.IAppender
}

func NewCYLoggerEntity(eLogType Core.ELogType) *CYLoggerEntity {
	return &CYLoggerEntity{eLogType: eLogType, appenders: make([]Appender.IAppender, 0)}
}

func (e *CYLoggerEntity) GetLogType() Core.ELogType { return e.eLogType }

// GetId returns the unique log-type id of this entity (mirrors C++ GetId).
func (e *CYLoggerEntity) GetId() Core.ELogType { return e.eLogType }

// GetLogName returns the active log file path of the first file-backed appender,
// mirroring C++ GetLogName which delegates to its single appender.
func (e *CYLoggerEntity) GetLogName() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, app := range e.appenders {
		if name := app.GetLogName(); name != "" {
			return name
		}
	}
	return ""
}

// GetSize returns the total size in bytes of all file-backed appenders owned by
// this entity (mirrors delegating to each appender's GetSize).
func (e *CYLoggerEntity) GetSize() int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var total int64
	for _, app := range e.appenders {
		total += app.GetSize()
	}
	return total
}

// ForceNewFile forces every appender to rotate to a fresh file (mirrors C++ ForceNewFile).
func (e *CYLoggerEntity) ForceNewFile() {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, app := range e.appenders {
		app.ForceNewFile()
	}
}

func (e *CYLoggerEntity) AddAppender(appender_ Appender.IAppender) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.appenders = append(e.appenders, appender_)
}

func (e *CYLoggerEntity) RemoveAppender(appender_ Appender.IAppender) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i, a := range e.appenders {
		if a == appender_ {
			e.appenders = append(e.appenders[:i], e.appenders[i+1:]...)
			return
		}
	}
}

func (e *CYLoggerEntity) GetAppender(index int) Appender.IAppender {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if index >= 0 && index < len(e.appenders) {
		return e.appenders[index]
	}
	return nil
}

func (e *CYLoggerEntity) GetAppenderCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.appenders)
}

func (e *CYLoggerEntity) Write(msg *Common.CYBaseMessage) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, app := range e.appenders {
		if !app.IsEnable() {
			continue
		}
		// Each appender owns its message-handling policy: synchronous appenders
		// (file/main) write in-place without releasing, while asynchronous ones
		// (console/remote/buffer/system) clone the message into their queue and
		// release the clone when consumed. The original msg is released exactly
		// once by the caller after routing completes.
		app.Write(msg)
	}
}

func (e *CYLoggerEntity) Flush() {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, app := range e.appenders {
		app.Flush()
	}
}

func (e *CYLoggerEntity) Init() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	ok := true
	for _, app := range e.appenders {
		if !app.Init() {
			ok = false
		}
	}
	return ok
}

func (e *CYLoggerEntity) UnInit() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, app := range e.appenders {
		app.UnInit()
		// Mirror the C++ detach path: once an entity is torn down its appenders
		// must be removed from the global appender registry, otherwise the
		// registry only ever grows and can never reflect the live mount state.
		Appender.GetCYLoggerAppenderFactoryInstance().UnregisterAppender(app)
	}
	e.appenders = nil
}

func (e *CYLoggerEntity) SetEnable(bEnable bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, app := range e.appenders {
		app.SetEnable(bEnable)
	}
}

// CYLoggerEntityFactory creates and manages logger entities by type.
type CYLoggerEntityFactory struct {
	Common.CYNoCopy
	mu       sync.RWMutex
	entities map[Core.ELogType]*CYLoggerEntity
}

var g_CYLoggerEntityFactoryInstance *CYLoggerEntityFactory
var g_CYLoggerEntityFactoryOnce sync.Once

func GetCYLoggerEntityFactoryInstance() *CYLoggerEntityFactory {
	g_CYLoggerEntityFactoryOnce.Do(func() {
		f := &CYLoggerEntityFactory{entities: make(map[Core.ELogType]*CYLoggerEntity)}
		for t := Core.LogTypeNone + 1; t < Core.LogTypeMax; t++ {
			f.entities[t] = NewCYLoggerEntity(t)
		}
		g_CYLoggerEntityFactoryInstance = f
	})
	return g_CYLoggerEntityFactoryInstance
}

func (f *CYLoggerEntityFactory) GetEntity(eLogType Core.ELogType) *CYLoggerEntity {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.entities[eLogType]
}

func (f *CYLoggerEntityFactory) CreateEntity(eLogType Core.ELogType) *CYLoggerEntity {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e, ok := f.entities[eLogType]; ok {
		return e
	}
	e := NewCYLoggerEntity(eLogType)
	f.entities[eLogType] = e
	return e
}

// GetLoggerEntity returns the entity registered for eLogType, mirroring C++ GetLoggerEntity.
func (f *CYLoggerEntityFactory) GetLoggerEntity(eLogType Core.ELogType) *CYLoggerEntity {
	return f.GetEntity(eLogType)
}

// ReleaseLoggerEntity flushes and detaches the entity for eLogType, then removes it
// from the factory map (mirrors C++ ReleaseLoggerEntity).
func (f *CYLoggerEntityFactory) ReleaseLoggerEntity(eLogType Core.ELogType) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if e, ok := f.entities[eLogType]; ok {
		e.Flush()
		e.UnInit()
		delete(f.entities, eLogType)
	}
}

// ForceEntityNewFile forces every registered entity to rotate to a fresh file
// (mirrors C++ ForceEntityNewFile).
func (f *CYLoggerEntityFactory) ForceEntityNewFile() {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, e := range f.entities {
		e.ForceNewFile()
	}
}

// ReleaseAllLoggerEntity flushes, detaches and removes every registered entity
// (mirrors C++ ReleaseAllLoggerEntity).
func (f *CYLoggerEntityFactory) ReleaseAllLoggerEntity() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for t, e := range f.entities {
		e.Flush()
		e.UnInit()
		delete(f.entities, t)
	}
}

func (f *CYLoggerEntityFactory) InitAll() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	ok := true
	for _, e := range f.entities {
		if !e.Init() {
			ok = false
		}
	}
	return ok
}

func (f *CYLoggerEntityFactory) UnInitAll() {
	// Snapshot entity list before releasing factory lock.
	// e.UnInit() acquires entity write locks, so holding f.mu
	// would cause a read-write deadlock with AddAppender.
	f.mu.RLock()
	entities := make([]*CYLoggerEntity, 0, len(f.entities))
	for _, e := range f.entities {
		entities = append(entities, e)
	}
	f.mu.RUnlock()

	for _, e := range entities {
		e.UnInit()
	}
}

func (f *CYLoggerEntityFactory) FlushAll() {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, e := range f.entities {
		e.Flush()
	}
}
