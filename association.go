// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import "fmt"

// association is a registered belongs_to/has_many relationship. The two ends are
// symmetric over a (foreignKey, primaryKey) pair: the "foreign" side carries an
// attribute (foreignKey) whose values reference the "primary" side's primaryKey.
// Either key may be the sentinel "dn" ([DNKey]), meaning "match the entry's DN
// rather than an attribute".
type association struct {
	name       string
	target     *Class
	foreignKey string
	primaryKey string
	many       bool
}

// DNKey is the sentinel key meaning "the entry's distinguished name" in an
// association's foreign_key/primary_key — the ActiveLdap foreign_key: "dn" form.
const DNKey = "dn"

// BelongsTo registers a belongs_to association: this record's foreignKey values
// reference the target's primaryKey. Resolving it yields the target record(s)
// whose primaryKey matches — the "parent" of this record. Use [DNKey] for a
// DN-valued reference (e.g. member: "dn").
func (c *Class) BelongsTo(name string, target *Class, foreignKey, primaryKey string) {
	c.associations[fold(name)] = &association{
		name: name, target: target, foreignKey: foreignKey, primaryKey: primaryKey, many: false,
	}
}

// HasMany registers a has_many association: target records carry a foreignKey
// referencing this record's primaryKey. Resolving it yields every target that
// points back — the "children" of this record.
func (c *Class) HasMany(name string, target *Class, foreignKey, primaryKey string) {
	c.associations[fold(name)] = &association{
		name: name, target: target, foreignKey: foreignKey, primaryKey: primaryKey, many: true,
	}
}

// Association resolves a named association from this record, returning every
// matching target record. An unknown name returns an error.
func (b *Base) Association(name string) ([]*Base, error) {
	assoc, ok := b.class.associations[fold(name)]
	if !ok {
		return nil, fmt.Errorf("%s: unknown association %q", b.class.name, name)
	}
	// belongs_to: read this record's foreignKey, match target's primaryKey.
	// has_many:   read this record's primaryKey, match target's foreignKey.
	var localKey, targetKey string
	if assoc.many {
		localKey, targetKey = assoc.primaryKey, assoc.foreignKey
	} else {
		localKey, targetKey = assoc.foreignKey, assoc.primaryKey
	}
	values := b.keyValues(localKey)
	return resolveByValues(assoc.target, targetKey, values)
}

// AssociationOne resolves a singular association ([Class.BelongsTo]) and returns
// the first matching record, or nil if none.
func (b *Base) AssociationOne(name string) (*Base, error) {
	recs, err := b.Association(name)
	if err != nil {
		return nil, err
	}
	if len(recs) == 0 {
		return nil, nil
	}
	return recs[0], nil
}

// keyValues returns the values of an association key on this record: the DN when
// key is [DNKey], otherwise the attribute's values.
func (b *Base) keyValues(key string) []string {
	if key == DNKey {
		if dn := b.DN(); dn != "" {
			return []string{dn}
		}
		return nil
	}
	return b.Get(key)
}

// resolveByValues finds every record of target whose key matches one of values.
// When key is [DNKey] each value is looked up as a DN (a base-scoped read);
// otherwise an OR-of-equalities filter is issued over the class. Duplicate
// results (by normalized DN) are collapsed.
func resolveByValues(target *Class, key string, values []string) ([]*Base, error) {
	if len(values) == 0 {
		return nil, nil
	}
	var out []*Base
	seen := map[string]bool{}
	add := func(rec *Base) {
		if rec == nil {
			return
		}
		k := normDN(rec.DN())
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, rec)
	}
	if key == DNKey {
		for _, v := range values {
			rec, err := target.findByDN(v)
			if err != nil {
				return nil, err
			}
			add(rec)
		}
		return out, nil
	}
	eqs := make([]Filter, len(values))
	for i, v := range values {
		eqs[i] = Equal(key, v)
	}
	recs, err := target.Search(FindOptions{Filter: Or(eqs...)})
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		add(r)
	}
	return out, nil
}

// findByDN loads a single record by its full DN via a base-scoped search,
// returning nil when the entry is absent. Unlike [Class.Find] it does not raise
// on a miss, so association resolution can skip dangling references.
func (c *Class) findByDN(dn string) (*Base, error) {
	scope := ScopeBase
	return c.FindFirst(FindOptions{Base: dn, Scope: &scope})
}
