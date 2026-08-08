// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"sort"
	"strings"
)

// attributeSet is the ordered, case-insensitive, alias-aware store of an entry's
// LDAP attributes — the data behind a [Base]. LDAP attribute names are
// case-insensitive, so lookups fold case; every attribute is multi-valued at the
// protocol level, so values are held as a []string. ActiveLdap presents some
// attributes as single-valued (schema SINGLE-VALUE, or a mapping declaration);
// that is a presentation choice applied by [Base], not stored here.
type attributeSet struct {
	// canon maps a folded name to the canonical (as-declared) name so output
	// preserves the caller's spelling (cn, not CN).
	canon map[string]string
	vals  map[string][]string // keyed by folded name
	order []string            // folded names in insertion order
	// aliases maps a folded alias to the folded canonical name, e.g.
	// "commonname" -> "cn". Resolution is applied before every lookup.
	aliases map[string]string
}

func newAttributeSet(aliases map[string]string) *attributeSet {
	return &attributeSet{
		canon:   map[string]string{},
		vals:    map[string][]string{},
		aliases: aliases,
	}
}

func fold(name string) string { return strings.ToLower(strings.TrimSpace(name)) }

// resolve folds name and follows an alias to its canonical folded name.
func (a *attributeSet) resolve(name string) string {
	f := fold(name)
	if c, ok := a.aliases[f]; ok {
		return c
	}
	return f
}

// Set replaces the value list for name (creating it, remembering its canonical
// spelling and insertion order on first use). A nil or empty slice still creates
// the attribute with no values.
func (a *attributeSet) Set(name string, values []string) {
	f := a.resolve(name)
	if _, seen := a.vals[f]; !seen {
		a.order = append(a.order, f)
		if _, ok := a.canon[f]; !ok {
			a.canon[f] = strings.TrimSpace(name)
		}
	}
	cp := append([]string(nil), values...)
	a.vals[f] = cp
}

// Get returns a copy of the value list for name, or nil if the attribute is
// absent.
func (a *attributeSet) Get(name string) []string {
	f := a.resolve(name)
	v, ok := a.vals[f]
	if !ok {
		return nil
	}
	return append([]string(nil), v...)
}

// Has reports whether name is present (with or without values).
func (a *attributeSet) Has(name string) bool {
	_, ok := a.vals[a.resolve(name)]
	return ok
}

// Delete removes name and its values, keeping insertion order of the rest.
func (a *attributeSet) Delete(name string) {
	f := a.resolve(name)
	if _, ok := a.vals[f]; !ok {
		return
	}
	delete(a.vals, f)
	delete(a.canon, f)
	for i, o := range a.order {
		if o == f {
			a.order = append(a.order[:i], a.order[i+1:]...)
			break
		}
	}
}

// Names returns the canonical attribute names in insertion order.
func (a *attributeSet) Names() []string {
	out := make([]string, len(a.order))
	for i, f := range a.order {
		out[i] = a.canon[f]
	}
	return out
}

// clone returns a deep copy sharing the (immutable) alias map.
func (a *attributeSet) clone() *attributeSet {
	c := newAttributeSet(a.aliases)
	c.order = append([]string(nil), a.order...)
	for f, name := range a.canon {
		c.canon[f] = name
	}
	for f, v := range a.vals {
		c.vals[f] = append([]string(nil), v...)
	}
	return c
}

// sortedNames returns the folded names sorted, for deterministic output where
// insertion order is not meaningful (e.g. modify-op diffs).
func (a *attributeSet) sortedFolded() []string {
	out := append([]string(nil), a.order...)
	sort.Strings(out)
	return out
}
