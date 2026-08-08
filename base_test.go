// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"reflect"
	"testing"
)

func TestNewRecord(t *testing.T) {
	c, _ := newPersonClass(t)
	b := c.New()
	if !b.NewRecord() || b.Persisted() {
		t.Fatal("new record flags")
	}
	if !reflect.DeepEqual(b.Get("objectClass"), []string{"top", "person", "inetOrgPerson"}) {
		t.Fatalf("objectClass: %#v", b.Get("objectClass"))
	}
	if b.Class() != c {
		t.Fatal("class")
	}
	// class without objectClasses
	c2 := NewClass("X", &Mapping{DNAttribute: "cn"}, c.conn)
	if len(c2.New().Get("objectClass")) != 0 {
		t.Fatal("no classes -> no objectClass")
	}
}

func TestIDAndDN(t *testing.T) {
	c, _ := newPersonClass(t)
	b := c.New()
	if b.ID() != "" || b.DN() != "" {
		t.Fatal("empty id/dn")
	}
	b.SetID("alice")
	if b.ID() != "alice" {
		t.Fatal("SetID")
	}
	if b.DN() != "uid=alice,ou=Users,dc=example,dc=com" {
		t.Fatalf("computed dn: %q", b.DN())
	}
}

func TestAccessors(t *testing.T) {
	c, _ := newPersonClass(t)
	b := c.New()
	b.Set("cn", "Alice")
	if b.One("commonName") != "Alice" { // alias + One
		t.Fatal("One via alias")
	}
	if b.One("missing") != "" {
		t.Fatal("One absent")
	}
	b.Add("mail", "a@x")
	b.Add("mail", "a@y")
	if !reflect.DeepEqual(b.Get("mail"), []string{"a@x", "a@y"}) {
		t.Fatalf("Add: %#v", b.Get("mail"))
	}
	if !b.Has("mail") {
		t.Fatal("Has")
	}
	b.Delete("mail")
	if b.Has("mail") {
		t.Fatal("Delete")
	}
	b.SetID("alice")
	if names := b.AttributeNames(); len(names) == 0 {
		t.Fatal("names")
	}
	attrs := b.Attributes()
	if attrs["cn"][0] != "Alice" {
		t.Fatalf("Attributes: %#v", attrs)
	}
	if b.Errors() == nil {
		t.Fatal("errors")
	}
}

func TestDirtyTracking(t *testing.T) {
	c, dir := newPersonClass(t)
	dir.Seed("uid=alice,ou=Users,dc=example,dc=com", map[string][]string{
		"uid": {"alice"}, "cn": {"Alice"}, "sn": {"Adams"}, "objectClass": {"top", "person", "inetOrgPerson"},
	})
	b, err := c.Find("alice")
	if err != nil {
		t.Fatal(err)
	}
	if b.Changed() {
		t.Fatal("freshly loaded should be clean")
	}
	b.Set("cn", "Alice A")
	b.Delete("sn")
	changed := b.ChangedAttributes()
	if !reflect.DeepEqual(changed, []string{"cn", "sn"}) {
		t.Fatalf("changed: %#v", changed)
	}
	ch := b.Changes()
	if ch["cn"][0][0] != "Alice" || ch["cn"][1][0] != "Alice A" {
		t.Fatalf("changes cn: %#v", ch["cn"])
	}
	if !reflect.DeepEqual(ch["sn"][1], []string(nil)) {
		t.Fatalf("changes sn after: %#v", ch["sn"][1])
	}
}

func TestNewFromEntryDNAttrFirst(t *testing.T) {
	c, _ := newPersonClass(t)
	e := &Entry{DN: "uid=alice,ou=Users,dc=example,dc=com", Attributes: map[string][]string{
		"cn": {"Alice"}, "uid": {"alice"}, "objectClass": {"person"},
	}}
	b := c.newFromEntry(e)
	if b.ID() != "alice" || !b.Persisted() {
		t.Fatal("from entry")
	}
	if b.DN() != "uid=alice,ou=Users,dc=example,dc=com" {
		t.Fatalf("override dn: %q", b.DN())
	}
	// entry without dn attribute value present
	e2 := &Entry{DN: "cn=x", Attributes: map[string][]string{"cn": {"x"}}}
	b2 := c.newFromEntry(e2)
	if b2.ID() != "" {
		t.Fatal("no uid -> empty id")
	}
}

func TestEqualValuesAndToS(t *testing.T) {
	if equalValues([]string{"a"}, []string{"a", "b"}) {
		t.Fatal("len diff")
	}
	if equalValues([]string{"a"}, []string{"b"}) {
		t.Fatal("val diff")
	}
	if !equalValues([]string{"a"}, []string{"a"}) {
		t.Fatal("equal")
	}
	c, _ := newPersonClass(t)
	b := c.New()
	if b.ToS() != "Person (no dn)" {
		t.Fatalf("ToS no dn: %q", b.ToS())
	}
	b.SetID("alice")
	if b.ToS() != "Person uid=alice,ou=Users,dc=example,dc=com" {
		t.Fatalf("ToS: %q", b.ToS())
	}
}

func TestHasObjectClass(t *testing.T) {
	c, _ := newPersonClass(t)
	b := c.New()
	if !b.hasObjectClass("PERSON") || b.hasObjectClass("nope") {
		t.Fatal("hasObjectClass")
	}
}
