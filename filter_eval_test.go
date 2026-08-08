// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import "testing"

func TestParseFilterRoundTrip(t *testing.T) {
	inputs := []string{
		"(objectClass=person)",
		"(&(objectClass=person)(uid=alice))",
		"(|(uid=a)(uid=b))",
		"(!(mail=*))",
		"(cn=a*b*c)",
		"(cn=*)",
	}
	for _, in := range inputs {
		f, err := ParseFilter(in)
		if err != nil {
			t.Fatalf("parse %q: %v", in, err)
		}
		if f.String() != in {
			t.Errorf("round-trip %q -> %q", in, f.String())
		}
	}
}

func TestParseFilterEmpty(t *testing.T) {
	f, err := ParseFilter("   ")
	if err != nil || f.String() != "(objectClass=*)" {
		t.Fatalf("empty: %v %q", err, f.String())
	}
}

func TestParseFilterHexEscape(t *testing.T) {
	f, err := ParseFilter(`(cn=a\28b)`)
	if err != nil {
		t.Fatal(err)
	}
	e := f.(eqFilter)
	if e.value != "a(b" {
		t.Fatalf("unescaped value: %q", e.value)
	}
	// Re-escapes the parsed literal '(' on String.
	if f.String() != `(cn=a\28b)` {
		t.Fatalf("re-escape: %q", f.String())
	}
}

func TestParseFilterErrors(t *testing.T) {
	bad := []string{
		"noparen",
		"(",
		"(&)",
		"(=x)",
		"(cn)",
		"(cn=a)trailing",
		"(&(a=1)", // unterminated group -> missing ')'
		"(!)",
	}
	for _, in := range bad {
		if _, err := ParseFilter(in); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestUnescapeFilterValue(t *testing.T) {
	if unescapeFilterValue(`a\2ab`) != "a*b" {
		t.Fatalf("hex: %q", unescapeFilterValue(`a\2ab`))
	}
	if unescapeFilterValue(`a\*b`) != "a*b" {
		t.Fatalf("literal: %q", unescapeFilterValue(`a\*b`))
	}
	if unescapeFilterValue(`ab\`) != "ab" {
		t.Fatalf("trailing: %q", unescapeFilterValue(`ab\`))
	}
}

func TestEvalFilter(t *testing.T) {
	e := &Entry{DN: "uid=alice,dc=com", Attributes: map[string][]string{
		"uid":         {"alice"},
		"cn":          {"Alice Adams"},
		"objectClass": {"person", "inetOrgPerson"},
	}}
	mustEval := func(filter string, want bool) {
		f, err := ParseFilter(filter)
		if err != nil {
			t.Fatalf("parse %q: %v", filter, err)
		}
		if got := evalFilter(f, e); got != want {
			t.Errorf("eval %q = %v want %v", filter, got, want)
		}
	}
	mustEval("(uid=ALICE)", true) // case-insensitive
	mustEval("(uid=bob)", false)
	mustEval("(mail=*)", false)
	mustEval("(cn=*)", true)
	mustEval("(cn=alice*adams)", true)
	mustEval("(cn=x*y)", false)
	mustEval("(&(uid=alice)(objectClass=person))", true)
	mustEval("(&(uid=alice)(objectClass=nope))", false)
	mustEval("(|(uid=bob)(uid=alice))", true)
	mustEval("(|(uid=bob)(uid=carol))", false)
	mustEval("(!(uid=bob))", true)
}

func TestEvalFilterRawAndDefault(t *testing.T) {
	e := &Entry{Attributes: map[string][]string{"uid": {"x"}}}
	if !evalFilter(RawFilter("(uid=x)"), e) {
		t.Fatal("raw eval true")
	}
	if evalFilter(RawFilter("(("), e) {
		t.Fatal("raw parse-error should be false")
	}
	// A filter node type not handled by evalFilter falls through to false.
	if evalFilter(unknownFilter{}, e) {
		t.Fatal("unknown node should be false")
	}
}

type unknownFilter struct{}

func (unknownFilter) String() string { return "(x=y)" }
func (unknownFilter) isFilter()      {}

func TestMatchSubstringFinalOnly(t *testing.T) {
	e := &Entry{Attributes: map[string][]string{"cn": {"Alice"}}}
	f, _ := ParseFilter("(cn=*ice)")
	if !evalFilter(f, e) {
		t.Fatal("final-only substring")
	}
	f2, _ := ParseFilter("(cn=*xyz)")
	if evalFilter(f2, e) {
		t.Fatal("final-only miss")
	}
}
