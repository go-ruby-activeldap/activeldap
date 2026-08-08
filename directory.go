// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"fmt"
	"sort"
	"strings"
)

// Scope is an LDAP search scope, the ActiveLdap :scope option and the Net::LDAP
// scope constant it maps to.
type Scope int

const (
	// ScopeBase searches only the base entry itself (Net::LDAP::SearchScope_BaseObject).
	ScopeBase Scope = iota
	// ScopeOne searches the base's immediate children (…SingleLevel).
	ScopeOne
	// ScopeSub searches the base and its whole subtree (…WholeSubtree).
	ScopeSub
)

// String renders the scope as ActiveLdap's symbol name (:base/:one/:sub).
func (s Scope) String() string {
	switch s {
	case ScopeBase:
		return "base"
	case ScopeOne:
		return "one"
	case ScopeSub:
		return "sub"
	default:
		return fmt.Sprintf("Scope(%d)", int(s))
	}
}

// ParseScope maps an ActiveLdap scope symbol name to a [Scope]. It accepts
// "base"/"baseobject", "one"/"onelevel"/"singlelevel", "sub"/"subtree"/
// "wholesubtree"; an unknown name returns ok=false.
func ParseScope(name string) (Scope, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "base", "baseobject", "base_object":
		return ScopeBase, true
	case "one", "onelevel", "one_level", "singlelevel", "single_level":
		return ScopeOne, true
	case "sub", "subtree", "wholesubtree", "whole_subtree", "":
		return ScopeSub, true
	default:
		return ScopeSub, false
	}
}

// Entry is a directory entry as returned by a search — a DN plus its attributes.
// It mirrors a Net::LDAP::Entry: attribute names are case-insensitive and values
// are always a list.
type Entry struct {
	DN         string
	Attributes map[string][]string
}

// Get returns the value list for a (case-insensitively matched) attribute name.
func (e *Entry) Get(name string) []string {
	f := fold(name)
	for k, v := range e.Attributes {
		if fold(k) == f {
			return v
		}
	}
	return nil
}

// ModOp is the kind of a [ModifyOp] — the Net::LDAP modify operation symbol.
type ModOp int

const (
	// ModAdd adds values to an attribute (:add).
	ModAdd ModOp = iota
	// ModReplace replaces an attribute's values (:replace).
	ModReplace
	// ModDelete deletes an attribute or specific values (:delete).
	ModDelete
)

// String renders the op as ActiveLdap/Net::LDAP's symbol name.
func (m ModOp) String() string {
	switch m {
	case ModAdd:
		return "add"
	case ModReplace:
		return "replace"
	case ModDelete:
		return "delete"
	default:
		return fmt.Sprintf("ModOp(%d)", int(m))
	}
}

// ModifyOp is one element of a Net::LDAP modify operations list —
// [op, attribute, values].
type ModifyOp struct {
	Op        ModOp
	Attribute string
	Values    []string
}

// SearchRequest is the argument to [Directory.Search], the four fields
// ActiveLdap fills from a find/search: the base DN, the scope, the RFC 4515
// filter string, and the attributes to return (nil = all).
type SearchRequest struct {
	Base       string
	Scope      Scope
	Filter     string
	Attributes []string
}

// Directory is the seam between this ORM and a real LDAP server: the four
// Net::LDAP operations ActiveLdap uses. The host (rbgo) wires it to a bound
// Net::LDAP connection; tests wire it to [MockDirectory]. Every method returns
// an error the ORM surfaces as a save/find failure.
type Directory interface {
	Search(req SearchRequest) ([]*Entry, error)
	Add(dn string, attributes map[string][]string) error
	Modify(dn string, ops []ModifyOp) error
	Delete(dn string) error
}

// MockDirectory is an in-memory [Directory] — the ActiveLdap test double and the
// fallback the rbgo binding uses when no Net::LDAP connection is configured. It
// stores entries by their normalized DN and answers searches by scope and
// filter, so the whole ORM can be exercised with no server.
type MockDirectory struct {
	// entries is keyed by normalized DN.
	entries map[string]*Entry
	// Log records every mutating call in order, for assertions (mirrors the
	// Net::LDAP mock's operation log).
	Log []string
	// FailOn, when set, makes the named operation ("add"/"modify"/"delete"/
	// "search") return an error, to exercise the ORM's error branches.
	FailOn map[string]string
}

// NewMockDirectory builds an empty [MockDirectory].
func NewMockDirectory() *MockDirectory {
	return &MockDirectory{entries: map[string]*Entry{}, FailOn: map[string]string{}}
}

func normDN(dn string) string {
	p, ok := ParseDN(dn)
	if !ok {
		return fold(dn)
	}
	return p.Normalized()
}

// Seed inserts or replaces an entry directly, bypassing the Log — used by tests
// to populate the directory before exercising the ORM.
func (m *MockDirectory) Seed(dn string, attrs map[string][]string) {
	cp := map[string][]string{}
	for k, v := range attrs {
		cp[k] = append([]string(nil), v...)
	}
	m.entries[normDN(dn)] = &Entry{DN: dn, Attributes: cp}
}

func (m *MockDirectory) fail(op string) error {
	if msg, ok := m.FailOn[op]; ok {
		return fmt.Errorf("%s failed: %s", op, msg)
	}
	return nil
}

// Search implements [Directory.Search] over the in-memory store.
func (m *MockDirectory) Search(req SearchRequest) ([]*Entry, error) {
	if err := m.fail("search"); err != nil {
		return nil, err
	}
	m.Log = append(m.Log, "search "+req.Scope.String()+" "+req.Base+" "+req.Filter)
	f, err := parseFilterString(req.Filter)
	if err != nil {
		return nil, err
	}
	base := normDN(req.Base)
	var out []*Entry
	for _, e := range m.entries {
		if !scopeMatches(req.Scope, base, normDN(e.DN)) {
			continue
		}
		if !evalFilter(f, e) {
			continue
		}
		out = append(out, projectEntry(e, req.Attributes))
	}
	sort.Slice(out, func(i, j int) bool { return normDN(out[i].DN) < normDN(out[j].DN) })
	return out, nil
}

func scopeMatches(scope Scope, base, dn string) bool {
	switch scope {
	case ScopeBase:
		return dn == base
	case ScopeOne:
		return ParseDNParent(dn) == base
	default: // ScopeSub
		return dn == base || strings.HasSuffix(dn, ","+base) || base == ""
	}
}

// ParseDNParent returns the normalized parent DN of a normalized DN string.
func ParseDNParent(normalized string) string {
	i := indexUnescaped(normalized, ',')
	if i < 0 {
		return ""
	}
	return normalized[i+1:]
}

func projectEntry(e *Entry, attrs []string) *Entry {
	out := &Entry{DN: e.DN, Attributes: map[string][]string{}}
	if len(attrs) == 0 {
		for k, v := range e.Attributes {
			out.Attributes[k] = append([]string(nil), v...)
		}
		return out
	}
	want := map[string]bool{}
	for _, a := range attrs {
		want[fold(a)] = true
	}
	for k, v := range e.Attributes {
		if want[fold(k)] {
			out.Attributes[k] = append([]string(nil), v...)
		}
	}
	return out
}

// Add implements [Directory.Add].
func (m *MockDirectory) Add(dn string, attributes map[string][]string) error {
	if err := m.fail("add"); err != nil {
		return err
	}
	key := normDN(dn)
	if _, exists := m.entries[key]; exists {
		return fmt.Errorf("entry already exists: %s", dn)
	}
	m.Log = append(m.Log, "add "+dn)
	m.Seed(dn, attributes)
	return nil
}

// Modify implements [Directory.Modify], applying add/replace/delete ops.
func (m *MockDirectory) Modify(dn string, ops []ModifyOp) error {
	if err := m.fail("modify"); err != nil {
		return err
	}
	key := normDN(dn)
	e, ok := m.entries[key]
	if !ok {
		return fmt.Errorf("no such entry: %s", dn)
	}
	for _, op := range ops {
		m.Log = append(m.Log, "modify "+op.Op.String()+" "+dn+" "+op.Attribute)
		applyMod(e, op)
	}
	return nil
}

func applyMod(e *Entry, op ModifyOp) {
	name := findAttrKey(e, op.Attribute)
	switch op.Op {
	case ModReplace:
		if len(op.Values) == 0 {
			delete(e.Attributes, name)
		} else {
			e.Attributes[name] = append([]string(nil), op.Values...)
		}
	case ModAdd:
		e.Attributes[name] = append(e.Attributes[name], op.Values...)
	case ModDelete:
		if len(op.Values) == 0 {
			delete(e.Attributes, name)
			return
		}
		e.Attributes[name] = removeValues(e.Attributes[name], op.Values)
	}
}

func findAttrKey(e *Entry, name string) string {
	f := fold(name)
	for k := range e.Attributes {
		if fold(k) == f {
			return k
		}
	}
	return name
}

func removeValues(have, drop []string) []string {
	dropSet := map[string]bool{}
	for _, d := range drop {
		dropSet[d] = true
	}
	var out []string
	for _, v := range have {
		if !dropSet[v] {
			out = append(out, v)
		}
	}
	return out
}

// Delete implements [Directory.Delete].
func (m *MockDirectory) Delete(dn string) error {
	if err := m.fail("delete"); err != nil {
		return err
	}
	key := normDN(dn)
	if _, ok := m.entries[key]; !ok {
		return fmt.Errorf("no such entry: %s", dn)
	}
	m.Log = append(m.Log, "delete "+dn)
	delete(m.entries, key)
	return nil
}
