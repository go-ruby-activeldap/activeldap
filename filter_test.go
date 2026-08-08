// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import "testing"

func TestFilterLeaves(t *testing.T) {
	if Equal("cn", "a(b)").String() != `(cn=a\28b\29)` {
		t.Fatalf("equal escape: %q", Equal("cn", "a(b)").String())
	}
	if Present("mail").String() != "(mail=*)" {
		t.Fatal("present")
	}
	if Substring("cn", "a", []string{"b"}, "c").String() != "(cn=a*b*c)" {
		t.Fatalf("substring: %q", Substring("cn", "a", []string{"b"}, "c").String())
	}
	if Substring("cn", "", nil, "").String() != "(cn=*)" {
		t.Fatalf("empty substring: %q", Substring("cn", "", nil, "").String())
	}
}

func TestFilterComposition(t *testing.T) {
	// And of one collapses.
	if And(Equal("a", "1")).String() != "(a=1)" {
		t.Fatal("and-one")
	}
	// nil compaction.
	if And(Equal("a", "1"), nil).String() != "(a=1)" {
		t.Fatalf("and-nil: %q", And(Equal("a", "1"), nil).String())
	}
	if And(Equal("a", "1"), Equal("b", "2")).String() != "(&(a=1)(b=2))" {
		t.Fatalf("and: %q", And(Equal("a", "1"), Equal("b", "2")).String())
	}
	if Or(Equal("a", "1")).String() != "(a=1)" {
		t.Fatal("or-one")
	}
	if Or(Equal("a", "1"), Equal("a", "2")).String() != "(|(a=1)(a=2))" {
		t.Fatalf("or: %q", Or(Equal("a", "1"), Equal("a", "2")).String())
	}
	if Not(Present("a")).String() != "(!(a=*))" {
		t.Fatal("not")
	}
}

func TestRawFilter(t *testing.T) {
	if RawFilter("(a=1)").String() != "(a=1)" {
		t.Fatal("raw parenthesized")
	}
	if RawFilter("a=1").String() != "(a=1)" {
		t.Fatalf("raw bare: %q", RawFilter("a=1").String())
	}
}

func TestEscapeFilterValue(t *testing.T) {
	if EscapeFilterValue("*()\\\x00") != `\2a\28\29\5c\00` {
		t.Fatalf("escape: %q", EscapeFilterValue("*()\\\x00"))
	}
	if EscapeFilterValue("plain") != "plain" {
		t.Fatal("plain")
	}
}

func TestFilterFromConditions(t *testing.T) {
	if filterFromConditions(nil) != nil {
		t.Fatal("empty conds should be nil")
	}
	f := filterFromConditions(map[string][]string{
		"uid":  {"alice"},
		"role": {"admin", "user"},
		"flag": {},
	})
	// sorted keys: flag, role, uid
	want := "(&(flag=*)(|(role=admin)(role=user))(uid=alice))"
	if f.String() != want {
		t.Fatalf("conds: %q want %q", f.String(), want)
	}
}

func TestClassesFilter(t *testing.T) {
	if classesFilter(nil).String() != "(objectClass=*)" {
		t.Fatal("no classes")
	}
	if classesFilter([]string{"top", "person"}).String() != "(&(objectClass=top)(objectClass=person))" {
		t.Fatalf("classes: %q", classesFilter([]string{"top", "person"}).String())
	}
	if classesFilter([]string{"person"}).String() != "(objectClass=person)" {
		t.Fatal("single class")
	}
}
