// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import "strings"

// Mapping is the Go form of ActiveLdap's ldap_mapping declaration — how a model
// class maps to a region of the directory:
//
//	ldap_mapping dn_attribute: "uid", prefix: "ou=Users",
//	             classes: ["top", "person", "inetOrgPerson"], scope: :sub
//
// The zero Mapping is invalid; build one and pass it to [NewClass].
type Mapping struct {
	// DNAttribute is the RDN attribute of every entry of this class (the
	// dn_attribute:), e.g. "uid" or "cn". Required.
	DNAttribute string
	// Prefix is the container DN, relative to the connection base, under which
	// entries live (the prefix:), e.g. "ou=Users". May be empty.
	Prefix string
	// Classes are the objectClass values every entry of this model carries
	// (classes:). The first is conventionally the structural class. Required.
	Classes []string
	// Scope is the default search scope for finds (scope:); the zero value
	// [ScopeSub] matches ActiveLdap's default of :sub.
	Scope Scope
	// Aliases maps an alternative attribute name to its canonical name
	// (ActiveLdap attribute aliases), e.g. {"commonName": "cn"}. Case-insensitive.
	Aliases map[string]string
	// SingleValued lists attributes presented as a single scalar rather than a
	// list (schema SINGLE-VALUE), so [Base.One] is the natural accessor.
	SingleValued []string
}

// foldedAliases returns the alias map folded to lower case on both sides, the
// form [attributeSet] resolves against.
func (m *Mapping) foldedAliases() map[string]string {
	out := map[string]string{}
	for alias, canon := range m.Aliases {
		out[fold(alias)] = fold(canon)
	}
	return out
}

func (m *Mapping) isSingleValued(name string) bool {
	f := fold(name)
	for _, s := range m.SingleValued {
		if fold(s) == f {
			return true
		}
	}
	return false
}

// Connection is the bound directory a [Class] operates against — the [Directory]
// seam plus the base DN under which the class's prefix is resolved. It is the Go
// form of the state ActiveLdap::Base.setup_connection / establish_connection
// installs (host/port/bind live inside the Directory the host wires).
type Connection struct {
	// Directory is the seam performing the actual LDAP operations.
	Directory Directory
	// Base is the connection's base DN (the base: of establish_connection),
	// e.g. "dc=example,dc=com".
	Base string
}

// NewConnection builds a [Connection] over a [Directory] and base DN.
func NewConnection(dir Directory, base string) *Connection {
	return &Connection{Directory: dir, Base: base}
}

// Class is a compiled model — a [Mapping] bound to a [Connection], the Go
// counterpart of an ActiveLdap::Base subclass. Its methods ([Class.New],
// [Class.Find], [Class.Search], [Class.Exist], [Class.Create]) are the class-side
// ORM surface. Associations and validators are registered on it before use.
type Class struct {
	name         string
	mapping      *Mapping
	conn         *Connection
	aliases      map[string]string
	associations map[string]*association
	validators   []Validator
}

// NewClass compiles a [Mapping] and binds it to a [Connection], yielding a
// [Class]. The class name is used in error messages and to_s. It panics if the
// mapping omits its required DNAttribute, mirroring ActiveLdap raising on an
// incomplete ldap_mapping.
func NewClass(name string, m *Mapping, conn *Connection) *Class {
	if strings.TrimSpace(m.DNAttribute) == "" {
		panic("activeldap: ldap_mapping requires a dn_attribute")
	}
	return &Class{
		name:         name,
		mapping:      m,
		conn:         conn,
		aliases:      m.foldedAliases(),
		associations: map[string]*association{},
	}
}

// Name returns the class name.
func (c *Class) Name() string { return c.name }

// Mapping returns the class's mapping.
func (c *Class) Mapping() *Mapping { return c.mapping }

// Connection returns the class's bound connection.
func (c *Class) Connection() *Connection { return c.conn }

// BaseDN returns the DN of this class's container: the mapping prefix joined to
// the connection base — the parent under which every entry's DN is formed.
func (c *Class) BaseDN() string {
	segs := []string{}
	if p := strings.TrimSpace(c.mapping.Prefix); p != "" {
		segs = append(segs, p)
	}
	if b := strings.TrimSpace(c.conn.Base); b != "" {
		segs = append(segs, b)
	}
	return strings.Join(segs, ",")
}

// DNForID builds the full DN for a record whose dn_attribute value is id:
// "<dn_attribute>=<id>,<prefix>,<base>".
func (c *Class) DNForID(id string) string {
	return BuildDN(c.mapping.DNAttribute, id, c.mapping.Prefix, c.conn.Base)
}
