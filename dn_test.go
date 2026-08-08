// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import "testing"

func TestParseDN(t *testing.T) {
	if dn, ok := ParseDN("  "); !ok || len(dn.RDNs) != 0 {
		t.Fatalf("empty DN: got %v ok=%v", dn, ok)
	}
	dn, ok := ParseDN("uid=alice,ou=Users,dc=example,dc=com")
	if !ok {
		t.Fatal("expected ok")
	}
	if len(dn.RDNs) != 4 || dn.RDNs[0].Attribute != "uid" || dn.RDNs[0].Value != "alice" {
		t.Fatalf("bad parse: %#v", dn.RDNs)
	}
	if _, ok := ParseDN("novalue,dc=com"); ok {
		t.Fatal("expected failure on missing '='")
	}
	if _, ok := ParseDN("=x,dc=com"); ok {
		t.Fatal("expected failure on empty attribute")
	}
	if _, ok := ParseDN("uid=a,,dc=com"); ok {
		t.Fatal("expected failure on empty component")
	}
}

func TestParseDNEscapes(t *testing.T) {
	dn, ok := ParseDN(`cn=Doe\, John,dc=com`)
	if !ok || dn.RDNs[0].Value != "Doe, John" {
		t.Fatalf("escaped comma: %#v ok=%v", dn.RDNs, ok)
	}
	dn2, _ := ParseDN(`cn=\41,dc=com`)
	if dn2.RDNs[0].Value != "A" {
		t.Fatalf("hex escape: %q", dn2.RDNs[0].Value)
	}
}

func TestDNStringRoundTrip(t *testing.T) {
	dn := DN{RDNs: []RDN{{"cn", "Doe, John"}, {"dc", "com"}}}
	if got := dn.String(); got != `cn=Doe\, John,dc=com` {
		t.Fatalf("String: %q", got)
	}
}

func TestDNNormalizedAndEqual(t *testing.T) {
	a, _ := ParseDN("UID=Alice, ou=Users")
	b, _ := ParseDN("uid=alice,ou=users")
	if a.Normalized() != b.Normalized() {
		t.Fatalf("normalized mismatch: %q vs %q", a.Normalized(), b.Normalized())
	}
	if !a.Equal(b) {
		t.Fatal("expected Equal")
	}
	c, _ := ParseDN("uid=bob")
	if a.Equal(c) {
		t.Fatal("unexpected Equal")
	}
}

func TestDNParent(t *testing.T) {
	dn, _ := ParseDN("uid=alice,ou=Users,dc=com")
	if p := dn.Parent().String(); p != "ou=Users,dc=com" {
		t.Fatalf("parent: %q", p)
	}
	root := DN{}
	if len(root.Parent().RDNs) != 0 {
		t.Fatal("root parent should be root")
	}
}

func TestBuildDN(t *testing.T) {
	cases := []struct{ attr, val, prefix, base, want string }{
		{"uid", "alice", "ou=Users", "dc=com", "uid=alice,ou=Users,dc=com"},
		{"uid", "bob", "", "dc=com", "uid=bob,dc=com"},
		{"uid", "carol", "ou=Users", "", "uid=carol,ou=Users"},
		{"uid", "dan", "", "", "uid=dan"},
	}
	for _, c := range cases {
		if got := BuildDN(c.attr, c.val, c.prefix, c.base); got != c.want {
			t.Errorf("BuildDN(%q,%q,%q,%q)=%q want %q", c.attr, c.val, c.prefix, c.base, got, c.want)
		}
	}
}

func TestEscapeUnescapeDNValue(t *testing.T) {
	if escapeDNValue("") != "" {
		t.Fatal("empty")
	}
	// leading space + trailing space + inner special; '#' only escaped when leading.
	if got := escapeDNValue(" #a,b "); got != `\ #a\,b\ ` {
		t.Fatalf("escape: %q", got)
	}
	if got := escapeDNValue("#lead"); got != `\#lead` {
		t.Fatalf("leading #: %q", got)
	}
	if escapeDNValue("x\x00y") != `x\00y` {
		t.Fatalf("nul: %q", escapeDNValue("x\x00y"))
	}
	if unescapeDNValue(`x\00y`) != "x\x00y" {
		t.Fatal("unescape nul")
	}
	if unescapeDNValue(`ab\`) != "ab" {
		t.Fatalf("trailing backslash: %q", unescapeDNValue(`ab\`))
	}
	if unescapeDNValue(`\,`) != "," {
		t.Fatal("literal escape")
	}
}

func TestSplitAndIndexUnescaped(t *testing.T) {
	parts := splitUnescaped(`a\,b,c`, ',')
	if len(parts) != 2 || parts[0] != `a\,b` || parts[1] != "c" {
		t.Fatalf("split: %#v", parts)
	}
	if indexUnescaped(`a\=b=c`, '=') != 4 {
		t.Fatalf("index: %d", indexUnescaped(`a\=b=c`, '='))
	}
	if indexUnescaped("abc", '=') != -1 {
		t.Fatal("expected -1")
	}
}

func TestHexHelpers(t *testing.T) {
	if !isHexDigit('a') || !isHexDigit('F') || !isHexDigit('9') || isHexDigit('g') {
		t.Fatal("isHexDigit")
	}
	if hexByte('4', '1') != 'A' {
		t.Fatal("hexByte")
	}
	if !isHex("41", 0) || isHex("4", 0) || isHex("4z", 0) {
		t.Fatal("isHex")
	}
}
