package Filter

import (
	"strings"
	"testing"
)

// TestPatternFilterAdd verifies the chainable Add semantics: append new keys,
// update existing keys, and remove a key when the value is empty.
func TestPatternFilterAdd(t *testing.T) {
	f := NewPatternFilter()
	f.Add("k1", "v1").Add("k2", "v2")

	fields := f.GetFields()
	if len(fields) != 2 {
		t.Fatalf("fields len = %d, want 2", len(fields))
	}
	if fields[0].Key != "k1" || fields[0].Value != "v1" {
		t.Errorf("fields[0] = %+v, want {k1 v1}", fields[0])
	}

	// Update an existing key.
	f.Add("k1", "v1b")
	fields = f.GetFields()
	if len(fields) != 2 || fields[0].Value != "v1b" {
		t.Errorf("after update: fields = %+v, want k1 -> v1b (len 2)", fields)
	}

	// Empty value removes the key.
	f.Add("k1", "")
	fields = f.GetFields()
	if len(fields) != 1 || fields[0].Key != "k2" {
		t.Errorf("after removal: fields = %+v, want only k2", fields)
	}
}

// TestPatternFilterGetFieldsIsCopy verifies GetFields returns a defensive copy.
func TestPatternFilterGetFieldsIsCopy(t *testing.T) {
	f := NewPatternFilter()
	f.Add("k", "v")
	fields := f.GetFields()
	fields[0].Value = "mutated"
	if f.GetFields()[0].Value != "v" {
		t.Errorf("GetFields must return a copy; internal state was mutated")
	}
}

// TestPatternFilterChainLinks verifies SetNextFilter appends to the tail of the
// linked list and GetNextFilter walks it.
func TestPatternFilterChainLinks(t *testing.T) {
	a := NewPatternFilter()
	b := NewPatternFilter()
	c := NewPatternFilter()

	a.SetNextFilter(b)
	a.SetNextFilter(c) // must land after b, not replace it

	if a.GetNextFilter() != b {
		t.Fatalf("a.next = %p, want b %p", a.GetNextFilter(), b)
	}
	if b.GetNextFilter() != c {
		t.Fatalf("b.next = %p, want c %p", b.GetNextFilter(), c)
	}
	if c.GetNextFilter() != nil {
		t.Fatalf("c.next should be nil")
	}
}

// TestFilterRequestEscaping verifies FilterRequest renders each field as
// escaped(key) + fieldNameEnd + escaped(value) + fieldValueEnd, doubling the
// escape char and prefixing delimiters with it.
func TestFilterRequestEscaping(t *testing.T) {
	f := NewPatternFilter()
	f.Add("k=1", "v|2")

	msg := ""
	got := f.FilterRequest(&msg, "=|", '/', '=', '|')
	want := "k/=1=v/|2|"
	if got != want {
		t.Errorf("FilterRequest = %q, want %q", got, want)
	}

	// Escape char itself must be doubled.
	f2 := NewPatternFilter()
	f2.Add("a/b", "c")
	got2 := f2.FilterRequest(&msg, "=|", '/', '=', '|')
	want2 := "a//b=c|"
	if got2 != want2 {
		t.Errorf("FilterRequest escape-char = %q, want %q", got2, want2)
	}
}

// TestFilterRequestChained verifies a chained filter contributes its fields
// after the head's fields.
func TestFilterRequestChained(t *testing.T) {
	head := NewPatternFilter()
	head.Add("h", "1")
	next := NewPatternFilter()
	next.Add("n", "2")
	head.SetNextFilter(next)

	msg := ""
	got := head.FilterRequest(&msg, "=|", '/', '=', '|')
	if !strings.HasPrefix(got, "h=1|") || !strings.Contains(got, "n=2|") {
		t.Errorf("chained FilterRequest = %q, want head then next fields", got)
	}
}

// TestFilterChainSingleton verifies the chain singleton append/clear/head logic.
func TestFilterChainSingleton(t *testing.T) {
	chain := GetCYLoggerPatternFilterChainInstance()
	if chain == nil {
		t.Fatal("chain singleton is nil")
	}
	if chain != GetCYLoggerPatternFilterChainInstance() {
		t.Fatal("chain singleton is not stable")
	}

	defer chain.ClearFilters()

	chain.ClearFilters()
	if chain.GetHeadFilter() != nil {
		t.Fatal("head should be nil after ClearFilters")
	}

	f1 := NewPatternFilter()
	f2 := NewPatternFilter()
	chain.AppendFilter(f1)
	chain.AppendFilter(f2)
	if chain.GetHeadFilter() != f1 {
		t.Errorf("head = %p, want f1 %p", chain.GetHeadFilter(), f1)
	}
	if f1.GetNextFilter() != f2 {
		t.Errorf("f1.next = %p, want f2 %p", f1.GetNextFilter(), f2)
	}

	// SetHeadFilter replaces the whole chain and recomputes the tail.
	f3 := NewPatternFilter()
	chain.SetHeadFilter(f3)
	if chain.GetHeadFilter() != f3 {
		t.Errorf("head after SetHeadFilter = %p, want f3 %p", chain.GetHeadFilter(), f3)
	}
	f4 := NewPatternFilter()
	chain.AppendFilter(f4)
	if f3.GetNextFilter() != f4 {
		t.Errorf("append after SetHeadFilter must attach to the new tail")
	}
}

// TestFilterManagerSingleton verifies the manager default filter carries the
// C++-compatible ("Channel", "Message") pair and Set/Get round-trips.
func TestFilterManagerSingleton(t *testing.T) {
	m := GetCYLoggerPatternFilterManagerInstance()
	if m == nil {
		t.Fatal("manager singleton is nil")
	}

	orig := m.GetFilter()
	if orig == nil {
		t.Fatal("default filter is nil")
	}
	fields := orig.GetFields()
	if len(fields) != 1 || fields[0].Key != "Channel" || fields[0].Value != "Message" {
		t.Errorf("default filter fields = %+v, want [{Channel Message}]", fields)
	}
	defer m.SetFilter(orig)

	custom := m.CreateDefaultFilter()
	custom.Add("X", "Y")
	m.SetFilter(custom)
	if m.GetFilter() != custom {
		t.Errorf("SetFilter/GetFilter did not round-trip")
	}
}
