// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package activeldap is a pure-Go (no cgo) reimplementation of the
// ActiveRecord-style LDAP object-relational mapper of Ruby's [ActiveLdap] gem:
// the object↔entry mapping (ldap_mapping), the finder/search query builder, the
// dirty-tracking record with single- and multi-valued attributes, validations,
// belongs_to/has_many associations over distinguished names, and LDIF
// import/export. It mirrors ActiveLdap's semantics faithfully, independent of
// any Ruby runtime.
//
// # What it is — and isn't
//
// The judgement-and-string-building heart of ActiveLdap — turning a mapping plus
// a set of conditions into a distinguished name and an RFC 4515 search filter,
// tracking which attributes changed, deciding add-vs-modify on save, diffing
// dirty attributes into LDAP modify operations, validating, and serialising to
// LDIF — is fully deterministic and needs no directory server and no Ruby
// runtime, so it lives here as plain Go.
//
// Talking to an actual directory is the host's job. A [Base] is bound to a
// [Directory] seam — the small four-method surface ActiveLdap uses out of
// Net::LDAP (search / add / modify / delete). The host (go-embedded-ruby's rbgo)
// wires that seam to the bound Net::LDAP connection provided by go-ruby-ldap;
// tests wire it to the in-memory [MockDirectory]. The mapping, DN, filter,
// dirty-diff and LDIF logic is what this library owns and tests; the network is
// the seam.
//
// # Mapping
//
// A model is described by a [Mapping] — the Go form of ActiveLdap's
// ldap_mapping(dn_attribute:, prefix:, classes:, scope:). [NewClass] compiles a
// mapping (plus attribute aliases and single-valued declarations) into a
// [Class]; [Class.New] and the finder methods ([Class.Find], [Class.Search],
// [Class.Exist], [Class.Create]) mint and load [Base] records against a
// [Connection].
//
// [ActiveLdap]: https://github.com/activeldap/activeldap
package activeldap
