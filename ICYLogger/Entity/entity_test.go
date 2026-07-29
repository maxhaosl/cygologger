package Entity

import (
	"sync/atomic"
	"testing"

	Appender "github.com/maxhaosl/cygologger/ICYLogger/Appender"
	Common "github.com/maxhaosl/cygologger/ICYLogger/Common"
	"github.com/maxhaosl/cygologger/ICYLogger/Core"
	"github.com/maxhaosl/cygologger/ICYLogger/Filter"
	"github.com/maxhaosl/cygologger/ICYLogger/Layout"
)

// fakeAppender is an in-memory IAppender used to observe entity routing without
// touching the filesystem or spawning goroutines.
type fakeAppender struct {
	logType   Core.ELogType
	enabled   atomic.Bool
	writes    atomic.Int64
	flushes   atomic.Int64
	rotations atomic.Int64
	unInits   atomic.Int64
	logName   string
	size      int64
	layout    Layout.ICYLoggerTemplateLayout
	filter    *Filter.ICYLoggerPatternFilter
}

func newFakeAppender(t Core.ELogType, name string, size int64) *fakeAppender {
	f := &fakeAppender{logType: t, logName: name, size: size}
	f.enabled.Store(true)
	return f
}

func (f *fakeAppender) Init() bool                              { return true }
func (f *fakeAppender) UnInit()                                 { f.unInits.Add(1) }
func (f *fakeAppender) Flush()                                  { f.flushes.Add(1) }
func (f *fakeAppender) Write(*Common.CYBaseMessage) bool        { f.writes.Add(1); return true }
func (f *fakeAppender) GetLogType() Core.ELogType               { return f.logType }
func (f *fakeAppender) GetChannel() string                      { return "" }
func (f *fakeAppender) GetFile() string                         { return "" }
func (f *fakeAppender) GetLogPath() string                      { return "" }
func (f *fakeAppender) SetLogPath(string)                       {}
func (f *fakeAppender) GetFileMode() Core.ELogFileMode          { return Core.LogFileModeTime }
func (f *fakeAppender) IsEnable() bool                          { return f.enabled.Load() }
func (f *fakeAppender) SetEnable(b bool)                        { f.enabled.Store(b) }
func (f *fakeAppender) GetLayout() Layout.ICYLoggerTemplateLayout { return f.layout }
func (f *fakeAppender) SetLayout(l Layout.ICYLoggerTemplateLayout) { f.layout = l }
func (f *fakeAppender) GetFilter() *Filter.ICYLoggerPatternFilter { return f.filter }
func (f *fakeAppender) SetFilter(p *Filter.ICYLoggerPatternFilter) { f.filter = p }
func (f *fakeAppender) GetFPSCounter() *Common.CYFPSCounter     { return nil }
func (f *fakeAppender) GetQueueSize() int                       { return 0 }
func (f *fakeAppender) Run()                                    {}
func (f *fakeAppender) Start(func())                            {}
func (f *fakeAppender) GetId() Core.ELogType                    { return f.logType }
func (f *fakeAppender) GetLogName() string                      { return f.logName }
func (f *fakeAppender) ForceNewFile()                           { f.rotations.Add(1) }
func (f *fakeAppender) GetSize() int64                          { return f.size }
func (f *fakeAppender) Copy(string)                             {}
func (f *fakeAppender) ClearContents()                          {}

var _ Appender.IAppender = (*fakeAppender)(nil)

// TestEntityAddRemoveAppender verifies appender bookkeeping.
func TestEntityAddRemoveAppender(t *testing.T) {
	e := NewCYLoggerEntity(Core.LogTypeInfo)
	if e.GetLogType() != Core.LogTypeInfo || e.GetId() != Core.LogTypeInfo {
		t.Fatalf("entity type/id mismatch")
	}

	a1 := newFakeAppender(Core.LogTypeInfo, "a1.log", 10)
	a2 := newFakeAppender(Core.LogTypeInfo, "a2.log", 20)
	e.AddAppender(a1)
	e.AddAppender(a2)

	if e.GetAppenderCount() != 2 {
		t.Fatalf("count = %d, want 2", e.GetAppenderCount())
	}
	if e.GetAppender(0) != Appender.IAppender(a1) || e.GetAppender(1) != Appender.IAppender(a2) {
		t.Errorf("GetAppender index mismatch")
	}
	if e.GetAppender(-1) != nil || e.GetAppender(2) != nil {
		t.Errorf("out-of-range GetAppender must return nil")
	}

	e.RemoveAppender(a1)
	if e.GetAppenderCount() != 1 || e.GetAppender(0) != Appender.IAppender(a2) {
		t.Errorf("RemoveAppender did not remove the right appender")
	}
}

// TestEntityWriteRespectsEnable verifies Write routes to enabled appenders only.
func TestEntityWriteRespectsEnable(t *testing.T) {
	e := NewCYLoggerEntity(Core.LogTypeInfo)
	on := newFakeAppender(Core.LogTypeInfo, "", 0)
	off := newFakeAppender(Core.LogTypeInfo, "", 0)
	off.SetEnable(false)
	e.AddAppender(on)
	e.AddAppender(off)

	msg := Common.AcquireBaseMessage()
	msg.EMsgType = int(Core.LogTypeInfo)
	msg.StrMsg = "hello"
	e.Write(msg)
	Common.ReleaseBaseMessage(msg)

	if on.writes.Load() != 1 {
		t.Errorf("enabled appender writes = %d, want 1", on.writes.Load())
	}
	if off.writes.Load() != 0 {
		t.Errorf("disabled appender writes = %d, want 0", off.writes.Load())
	}
}

// TestEntityAggregations verifies GetLogName (first non-empty), GetSize (sum),
// ForceNewFile / Flush / SetEnable fan-out.
func TestEntityAggregations(t *testing.T) {
	e := NewCYLoggerEntity(Core.LogTypeWarn)
	a1 := newFakeAppender(Core.LogTypeWarn, "", 15)
	a2 := newFakeAppender(Core.LogTypeWarn, "warn.log", 25)
	e.AddAppender(a1)
	e.AddAppender(a2)

	if got := e.GetLogName(); got != "warn.log" {
		t.Errorf("GetLogName = %q, want first non-empty %q", got, "warn.log")
	}
	if got := e.GetSize(); got != 40 {
		t.Errorf("GetSize = %d, want 40", got)
	}

	e.ForceNewFile()
	if a1.rotations.Load() != 1 || a2.rotations.Load() != 1 {
		t.Errorf("ForceNewFile must fan out to every appender")
	}
	e.Flush()
	if a1.flushes.Load() != 1 || a2.flushes.Load() != 1 {
		t.Errorf("Flush must fan out to every appender")
	}
	e.SetEnable(false)
	if a1.IsEnable() || a2.IsEnable() {
		t.Errorf("SetEnable(false) must fan out to every appender")
	}
}

// TestEntityUnInitDetaches verifies UnInit tears down and clears the appender list.
func TestEntityUnInitDetaches(t *testing.T) {
	e := NewCYLoggerEntity(Core.LogTypeDebug)
	a := newFakeAppender(Core.LogTypeDebug, "", 0)
	e.AddAppender(a)

	if !e.Init() {
		t.Fatalf("Init should succeed")
	}
	e.UnInit()
	if a.unInits.Load() != 1 {
		t.Errorf("appender UnInit calls = %d, want 1", a.unInits.Load())
	}
	if e.GetAppenderCount() != 0 {
		t.Errorf("appender count after UnInit = %d, want 0", e.GetAppenderCount())
	}
}

// TestEntityFactory verifies the singleton pre-seeds every log type, CreateEntity
// is idempotent, and ReleaseLoggerEntity removes then CreateEntity re-creates.
func TestEntityFactory(t *testing.T) {
	f := GetCYLoggerEntityFactoryInstance()
	if f == nil || f != GetCYLoggerEntityFactoryInstance() {
		t.Fatal("factory singleton is not stable")
	}

	for lt := Core.LogTypeNone + 1; lt < Core.LogTypeMax; lt++ {
		if f.GetEntity(lt) == nil {
			t.Errorf("pre-seeded entity missing for type %v", lt)
		}
	}

	e := f.GetEntity(Core.LogTypeError)
	if f.CreateEntity(Core.LogTypeError) != e {
		t.Errorf("CreateEntity of existing type must return the same instance")
	}
	if f.GetLoggerEntity(Core.LogTypeError) != e {
		t.Errorf("GetLoggerEntity must alias GetEntity")
	}

	// Release then re-create (restore the pre-seeded state for other tests).
	f.ReleaseLoggerEntity(Core.LogTypeError)
	if f.GetEntity(Core.LogTypeError) != nil {
		t.Errorf("entity must be gone after ReleaseLoggerEntity")
	}
	recreated := f.CreateEntity(Core.LogTypeError)
	if recreated == nil || f.GetEntity(Core.LogTypeError) != recreated {
		t.Errorf("CreateEntity must re-register the released type")
	}
}

// TestEntityFactoryForceNewFileFanOut verifies ForceEntityNewFile reaches
// appenders across entities.
func TestEntityFactoryForceNewFileFanOut(t *testing.T) {
	f := GetCYLoggerEntityFactoryInstance()
	e := f.CreateEntity(Core.LogTypeTrace)
	a := newFakeAppender(Core.LogTypeTrace, "", 0)
	e.AddAppender(a)
	defer e.RemoveAppender(a)

	f.ForceEntityNewFile()
	if a.rotations.Load() == 0 {
		t.Errorf("ForceEntityNewFile did not reach the appender")
	}
}
