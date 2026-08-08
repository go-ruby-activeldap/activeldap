// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Tests here close the remaining exact-branch coverage on the interface markers,
// the hex and substring helpers, whitespace-tolerant filter parsing, and the
// dangling-reference association path.
package activeldap

import "testing"

// TestFilterMarkers invokes the unexported isFilter marker on every concrete
// node so the closed-interface tags are covered.
func TestFilterMarkers(t *testing.T) {
	nodes := []Filter{
		eqFilter{}, presentFilter{}, substringFilter{}, andFilter{}, orFilter{},
		notFilter{}, rawFilter{},
	}
	for _, n := range nodes {
		n.isFilter()
	}
}

func TestHexNibbleAllBranches(t *testing.T) {
	// digit, lower-hex, upper-hex.
	if hexByte('3', '9') != 0x39 {
		t.Fatal("digits")
	}
	if hexByte('a', 'f') != 0xaf {
		t.Fatal("lower")
	}
	if hexByte('C', 'D') != 0xCD {
		t.Fatal("upper")
	}
}

func TestWhitespaceTolerantFilter(t *testing.T) {
	f, err := ParseFilter("  (uid=alice)  ")
	if err != nil {
		t.Fatalf("ws filter: %v", err)
	}
	if f.(eqFilter).value != "alice" {
		t.Fatalf("value: %q", f.(eqFilter).value)
	}
}

func TestParseGroupSubError(t *testing.T) {
	// A malformed leaf inside a group makes parseGroup return the sub error.
	if _, err := ParseFilter("(&(=x))"); err == nil {
		t.Fatal("expected sub-leaf error")
	}
}

func TestMatchSubstringBranches(t *testing.T) {
	e := &Entry{Attributes: map[string][]string{"cn": {"Alice Adams"}}}
	// initial matches but an any-part is absent.
	f1, _ := ParseFilter("(cn=alice*zzz)")
	if evalFilter(f1, e) {
		t.Fatal("any-part absent should not match")
	}
	// multi-valued: first value fails, second matches.
	e2 := &Entry{Attributes: map[string][]string{"cn": {"nope", "Alice Adams"}}}
	f2, _ := ParseFilter("(cn=alice*adams)")
	if !evalFilter(f2, e2) {
		t.Fatal("second value should match")
	}
	// three-segment substring: initial + one any-part + final, matching.
	f3, _ := ParseFilter("(cn=a*l*s)")
	if !evalFilter(f3, e) {
		t.Fatal("three-segment should match")
	}
	// three-segment with an absent middle any-part.
	f4, _ := ParseFilter("(cn=a*zz*s)")
	if evalFilter(f4, e) {
		t.Fatal("absent any-part should not match")
	}
	// initial mismatch.
	f5, _ := ParseFilter("(cn=xyz*)")
	if evalFilter(f5, e) {
		t.Fatal("initial mismatch should not match")
	}
}

func TestAssociationDedup(t *testing.T) {
	_, g, dir := assocFixture(t)
	// A group listing the same member DN twice resolves it once.
	dir.Seed("cn=dupes,ou=Groups,dc=example,dc=com", map[string][]string{
		"cn": {"dupes"},
		"member": {
			"uid=alice,ou=Users,dc=example,dc=com",
			"uid=alice,ou=Users,dc=example,dc=com",
		},
		"objectClass": {"groupOfNames"},
	})
	dupes, _ := g.Find("dupes")
	members, err := dupes.Association("members")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Fatalf("duplicate member DNs should dedupe to one: %d", len(members))
	}
}

func TestAssociationDanglingDN(t *testing.T) {
	_, g, dir := assocFixture(t)
	// Add a member DN that does not resolve to any entry.
	dir.Seed("cn=ghosts,ou=Groups,dc=example,dc=com", map[string][]string{
		"cn":          {"ghosts"},
		"member":      {"uid=nobody,ou=Users,dc=example,dc=com"},
		"objectClass": {"groupOfNames"},
	})
	ghosts, _ := g.Find("ghosts")
	members, err := ghosts.Association("members")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 0 {
		t.Fatalf("dangling reference should resolve to none: %#v", members)
	}
}
