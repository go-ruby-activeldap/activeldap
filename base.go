// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import "strings"

// Base is a single LDAP entry as an ActiveRecord-style object — the instance
// side of the ORM, the Go counterpart of an ActiveLdap::Base instance. It holds
// the current attribute values, tracks which changed since it was loaded/saved
// (dirty tracking), knows its class (mapping + connection), and answers whether
// it is new or persisted. Mint one with [Class.New] or load one with the finder
// methods.
type Base struct {
	class     *Class
	attrs     *attributeSet
	original  *attributeSet // snapshot at load/save, for dirty tracking
	persisted bool
	// dnOverride, when non-empty, is an explicit DN (a loaded entry's actual DN)
	// used in preference to the computed one.
	dnOverride string
	errors     *Errors
}

// New mints a new, unsaved record of this class with the objectClass values from
// the mapping already set — the Go form of Model.new. Additional attributes are
// assigned afterwards with [Base.Set].
func (c *Class) New() *Base {
	b := &Base{
		class:    c,
		attrs:    newAttributeSet(c.aliases),
		original: newAttributeSet(c.aliases),
		errors:   newErrors(),
	}
	if len(c.mapping.Classes) > 0 {
		b.attrs.Set("objectClass", append([]string(nil), c.mapping.Classes...))
	}
	return b
}

// newFromEntry builds a persisted record from a directory [Entry], snapshotting
// the loaded attributes as the dirty-tracking baseline.
func (c *Class) newFromEntry(e *Entry) *Base {
	b := &Base{
		class:      c,
		attrs:      newAttributeSet(c.aliases),
		persisted:  true,
		dnOverride: e.DN,
		errors:     newErrors(),
	}
	// Deterministic-ish assignment; DN attribute first so #id is always set.
	if vals := e.Get(c.mapping.DNAttribute); len(vals) > 0 {
		b.attrs.Set(c.mapping.DNAttribute, vals)
	}
	for name, vals := range e.Attributes {
		if fold(name) == fold(c.mapping.DNAttribute) {
			continue
		}
		b.attrs.Set(name, vals)
	}
	b.original = b.attrs.clone()
	return b
}

// Class returns the record's class.
func (b *Base) Class() *Class { return b.class }

// NewRecord reports whether the record has not yet been saved (ActiveLdap's
// new_entry? / !persisted?).
func (b *Base) NewRecord() bool { return !b.persisted }

// Persisted reports whether the record exists in the directory.
func (b *Base) Persisted() bool { return b.persisted }

// ID returns the record's dn_attribute value — ActiveLdap's #id.
func (b *Base) ID() string { return b.One(b.class.mapping.DNAttribute) }

// SetID assigns the dn_attribute value.
func (b *Base) SetID(id string) { b.Set(b.class.mapping.DNAttribute, id) }

// DN returns the record's distinguished name: its explicit loaded DN if it has
// one, otherwise the DN computed from the dn_attribute value, prefix and base.
// It returns "" when a new record has no dn_attribute value yet.
func (b *Base) DN() string {
	if b.dnOverride != "" {
		return b.dnOverride
	}
	id := b.ID()
	if id == "" {
		return ""
	}
	return b.class.DNForID(id)
}

// Get returns all values of an attribute (following aliases), or nil.
func (b *Base) Get(name string) []string { return b.attrs.Get(name) }

// One returns the first value of an attribute, or "" — the natural accessor for
// a single-valued attribute.
func (b *Base) One(name string) string {
	v := b.attrs.Get(name)
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

// Set assigns an attribute's values, replacing any existing ones, and records
// the change for dirty tracking. Passing no values clears the attribute.
func (b *Base) Set(name string, values ...string) {
	b.attrs.Set(name, values)
}

// Add appends values to an attribute (a multi-valued convenience), preserving
// existing values.
func (b *Base) Add(name string, values ...string) {
	cur := b.attrs.Get(name)
	b.attrs.Set(name, append(cur, values...))
}

// Has reports whether the attribute is present.
func (b *Base) Has(name string) bool { return b.attrs.Has(name) }

// Delete removes an attribute entirely.
func (b *Base) Delete(name string) { b.attrs.Delete(name) }

// AttributeNames returns the canonical attribute names in assignment order.
func (b *Base) AttributeNames() []string { return b.attrs.Names() }

// Attributes returns a name→values snapshot of the record's attributes, the Go
// form of ActiveLdap's #attributes hash.
func (b *Base) Attributes() map[string][]string {
	out := map[string][]string{}
	for _, name := range b.attrs.Names() {
		out[name] = b.attrs.Get(name)
	}
	return out
}

// Errors returns the validation errors from the last [Base.Valid] call.
func (b *Base) Errors() *Errors { return b.errors }

// Changed reports whether any attribute differs from the load/save baseline —
// ActiveLdap's #changed?. A new record is considered changed once it has any
// attribute beyond its objectClasses.
func (b *Base) Changed() bool { return len(b.ChangedAttributes()) > 0 }

// ChangedAttributes returns the folded names of attributes whose values differ
// from the baseline, sorted — the keys of ActiveLdap's #changes.
func (b *Base) ChangedAttributes() []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range b.attrs.sortedFolded() {
		cur := b.attrs.vals[f]
		old := b.original.vals[f]
		if !equalValues(cur, old) {
			out = append(out, f)
			seen[f] = true
		}
	}
	// Attributes removed since baseline (present in original, absent now).
	for _, f := range b.original.sortedFolded() {
		if _, ok := b.attrs.vals[f]; !ok && !seen[f] {
			out = append(out, f)
		}
	}
	return out
}

// Changes returns, per changed attribute, the [before, after] value pair —
// ActiveLdap's #changes.
func (b *Base) Changes() map[string][2][]string {
	out := map[string][2][]string{}
	for _, f := range b.ChangedAttributes() {
		out[f] = [2][]string{
			append([]string(nil), b.original.vals[f]...),
			append([]string(nil), b.attrs.vals[f]...),
		}
	}
	return out
}

func equalValues(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ToS renders a short description used in errors and inspection.
func (b *Base) ToS() string {
	dn := b.DN()
	if dn == "" {
		dn = "(no dn)"
	}
	return b.class.name + " " + dn
}

// dnAttributeValues returns the current dn_attribute values, used by DN and
// validations.
func (b *Base) dnAttributeValues() []string {
	return b.attrs.Get(b.class.mapping.DNAttribute)
}

// hasObjectClass reports whether a given objectClass is present (case-insensitive).
func (b *Base) hasObjectClass(class string) bool {
	for _, c := range b.attrs.Get("objectClass") {
		if strings.EqualFold(c, class) {
			return true
		}
	}
	return false
}
