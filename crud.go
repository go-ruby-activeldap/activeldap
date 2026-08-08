// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import "fmt"

// FindOptions are the keyword options of ActiveLdap's find/search:
//
//	find(:all, filter: "(mail=*)", base: "ou=People,dc=x", scope: :one, attributes: [...])
//
// A zero FindOptions means "the class defaults": the mapping scope, the class
// base DN, the objectClass guard as the only filter, and all attributes.
type FindOptions struct {
	// Filter is an extra condition AND-combined with the objectClass guard. It
	// may be any [Filter] (including one built from a Hash of conditions via
	// [Conditions]); nil adds no extra condition.
	Filter Filter
	// Base overrides the search base DN (the base: option). Empty uses the
	// class base DN.
	Base string
	// Scope overrides the search scope; nil uses the mapping scope.
	Scope *Scope
	// Attributes limits the returned attributes; nil returns all.
	Attributes []string
	// Limit caps the number of records returned when > 0 (ActiveLdap's :limit).
	Limit int
}

// Conditions builds the [Filter] for a Hash-style :filter option — a map of
// attribute→values AND-combined as equalities, the common ActiveLdap
// find(filter: {uid: "alice"}) form. It returns nil for an empty map.
func Conditions(conds map[string][]string) Filter { return filterFromConditions(conds) }

func (c *Class) searchRequest(opts FindOptions) SearchRequest {
	scope := c.mapping.Scope
	if opts.Scope != nil {
		scope = *opts.Scope
	}
	base := opts.Base
	if base == "" {
		base = c.BaseDN()
	}
	filter := And(classesFilter(c.mapping.Classes), opts.Filter)
	return SearchRequest{
		Base:       base,
		Scope:      scope,
		Filter:     filter.String(),
		Attributes: opts.Attributes,
	}
}

// Search runs a search and returns every matching record, the Go form of
// Model.search / find(:all). It applies the class objectClass guard, the given
// options, and (if set) the limit.
func (c *Class) Search(opts FindOptions) ([]*Base, error) {
	entries, err := c.conn.Directory.Search(c.searchRequest(opts))
	if err != nil {
		return nil, err
	}
	out := make([]*Base, 0, len(entries))
	for _, e := range entries {
		if opts.Limit > 0 && len(out) >= opts.Limit {
			break
		}
		out = append(out, c.newFromEntry(e))
	}
	return out, nil
}

// FindAll returns all records matching the options (find(:all, ...)).
func (c *Class) FindAll(opts FindOptions) ([]*Base, error) { return c.Search(opts) }

// FindFirst returns the first matching record, or nil if none — find(:first, ...).
func (c *Class) FindFirst(opts FindOptions) (*Base, error) {
	opts.Limit = 1
	recs, err := c.Search(opts)
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, nil
	}
	return recs[0], nil
}

// EntryNotFoundError is returned by [Class.Find] when no entry has the requested
// dn_attribute value — ActiveLdap's EntryNotFound.
type EntryNotFoundError struct {
	Class string
	ID    string
}

func (e *EntryNotFoundError) Error() string {
	return fmt.Sprintf("%s: could not find entry with id %q", e.Class, e.ID)
}

// Find loads the single record whose dn_attribute equals id — find(id). It
// searches at ScopeBase on the record's computed DN, raising [EntryNotFoundError]
// when absent.
func (c *Class) Find(id string) (*Base, error) {
	base := ScopeBase
	rec, err := c.FindFirst(FindOptions{
		Base:   c.DNForID(id),
		Scope:  &base,
		Filter: Equal(c.mapping.DNAttribute, id),
	})
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, &EntryNotFoundError{Class: c.name, ID: id}
	}
	return rec, nil
}

// Exist reports whether a record with the given dn_attribute value exists —
// ActiveLdap's exist?.
func (c *Class) Exist(id string) (bool, error) {
	rec, err := c.Find(id)
	if err != nil {
		if _, ok := err.(*EntryNotFoundError); ok {
			return false, nil
		}
		return false, err
	}
	return rec != nil, nil
}

// Create mints a record, assigns the given attributes, and saves it — the Go
// form of Model.create. The record is returned whether or not it saved; check
// the error (a [ValidationError] on invalid input) and [Base.Persisted].
func (c *Class) Create(attrs map[string][]string) (*Base, error) {
	b := c.New()
	for name, vals := range attrs {
		b.Set(name, vals...)
	}
	return b, b.Save()
}

// Save persists the record: an INSERT (Directory.Add) for a new record, or a
// diff-based UPDATE (Directory.Modify of only the changed attributes) for an
// existing one — ActiveLdap's #save. It validates first and returns a
// [ValidationError] without touching the directory when invalid. On success the
// record becomes persisted and its dirty baseline is reset.
func (b *Base) Save() error {
	if !b.Valid() {
		return &ValidationError{Record: b, Messages: b.errors.FullMessages()}
	}
	dir := b.class.conn.Directory
	dn := b.DN()
	if !b.persisted {
		if err := dir.Add(dn, b.Attributes()); err != nil {
			return err
		}
		b.persisted = true
		b.dnOverride = dn
		b.original = b.attrs.clone()
		return nil
	}
	ops := b.modifyOps()
	if len(ops) > 0 {
		if err := dir.Modify(dn, ops); err != nil {
			return err
		}
	}
	b.original = b.attrs.clone()
	return nil
}

// SaveOK is the boolean-returning save (ActiveLdap's #save returning true/false):
// it reports success and leaves the reason in [Base.Errors] / the returned error
// discarded.
func (b *Base) SaveOK() bool { return b.Save() == nil }

// modifyOps diffs the current attributes against the load/save baseline into the
// minimal Net::LDAP modify operations: a :replace for each changed or added
// attribute, and a :delete for each attribute removed since load. The
// dn_attribute is never emitted as a modification (changing it is a rename, not
// a modify).
func (b *Base) modifyOps() []ModifyOp {
	var ops []ModifyOp
	dnAttr := fold(b.class.mapping.DNAttribute)
	for _, f := range b.ChangedAttributes() {
		if f == dnAttr {
			continue
		}
		cur, present := b.attrs.vals[f]
		if !present || len(cur) == 0 {
			ops = append(ops, ModifyOp{Op: ModDelete, Attribute: f})
			continue
		}
		ops = append(ops, ModifyOp{Op: ModReplace, Attribute: f, Values: append([]string(nil), cur...)})
	}
	return ops
}

// UpdateAttributes assigns the given attributes and saves — ActiveLdap's
// #update_attributes. It returns the save error (a [ValidationError] on invalid
// input, leaving the in-memory changes in place as ActiveLdap does).
func (b *Base) UpdateAttributes(attrs map[string][]string) error {
	for name, vals := range attrs {
		b.Set(name, vals...)
	}
	return b.Save()
}

// Destroy deletes the record from the directory — ActiveLdap's #destroy. A new
// (never-saved) record cannot be destroyed and returns an error. On success the
// record becomes non-persisted.
func (b *Base) Destroy() error {
	if !b.persisted {
		return fmt.Errorf("%s: cannot destroy a new record", b.class.name)
	}
	if err := b.class.conn.Directory.Delete(b.DN()); err != nil {
		return err
	}
	b.persisted = false
	return nil
}

// Reload re-reads the record from the directory, discarding unsaved changes and
// resetting dirty tracking — ActiveLdap's #reload. It errors if the entry no
// longer exists.
func (b *Base) Reload() error {
	fresh, err := b.class.Find(b.ID())
	if err != nil {
		return err
	}
	b.attrs = fresh.attrs
	b.original = fresh.original
	b.dnOverride = fresh.dnOverride
	b.persisted = true
	return nil
}
