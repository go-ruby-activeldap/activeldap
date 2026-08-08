// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import "testing"

// personMapping is the shared test mapping: a person under ou=Users.
func personMapping() *Mapping {
	return &Mapping{
		DNAttribute:  "uid",
		Prefix:       "ou=Users",
		Classes:      []string{"top", "person", "inetOrgPerson"},
		Scope:        ScopeSub,
		Aliases:      map[string]string{"commonName": "cn"},
		SingleValued: []string{"uid", "cn"},
	}
}

// newPersonClass builds the shared test class bound to a fresh mock directory.
func newPersonClass(t *testing.T) (*Class, *MockDirectory) {
	t.Helper()
	dir := NewMockDirectory()
	conn := NewConnection(dir, "dc=example,dc=com")
	return NewClass("Person", personMapping(), conn), dir
}

func TestMappingHelpers(t *testing.T) {
	m := personMapping()
	fa := m.foldedAliases()
	if fa["commonname"] != "cn" {
		t.Fatalf("foldedAliases: %#v", fa)
	}
	if !m.isSingleValued("UID") || m.isSingleValued("mail") {
		t.Fatal("isSingleValued")
	}
}

func TestNewClassPanicsWithoutDNAttribute(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	NewClass("Bad", &Mapping{Classes: []string{"top"}}, NewConnection(NewMockDirectory(), ""))
}

func TestClassAccessors(t *testing.T) {
	c, dir := newPersonClass(t)
	if c.Name() != "Person" || c.Mapping().DNAttribute != "uid" || c.Connection().Directory != dir {
		t.Fatal("accessors")
	}
	if c.BaseDN() != "ou=Users,dc=example,dc=com" {
		t.Fatalf("BaseDN: %q", c.BaseDN())
	}
	if c.DNForID("alice") != "uid=alice,ou=Users,dc=example,dc=com" {
		t.Fatalf("DNForID: %q", c.DNForID("alice"))
	}
}

func TestBaseDNVariants(t *testing.T) {
	dir := NewMockDirectory()
	// prefix only
	c1 := NewClass("A", &Mapping{DNAttribute: "cn", Prefix: "ou=X"}, NewConnection(dir, ""))
	if c1.BaseDN() != "ou=X" {
		t.Fatalf("prefix only: %q", c1.BaseDN())
	}
	// base only
	c2 := NewClass("B", &Mapping{DNAttribute: "cn"}, NewConnection(dir, "dc=com"))
	if c2.BaseDN() != "dc=com" {
		t.Fatalf("base only: %q", c2.BaseDN())
	}
	// neither
	c3 := NewClass("C", &Mapping{DNAttribute: "cn"}, NewConnection(dir, ""))
	if c3.BaseDN() != "" {
		t.Fatalf("neither: %q", c3.BaseDN())
	}
}
