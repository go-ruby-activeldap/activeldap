// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestToLDIF(t *testing.T) {
	c, _ := newPersonClass(t)
	b := c.New()
	b.SetID("alice")
	b.Set("cn", "Alice")
	b.Set("sn", "Adams")
	b.Set("description", " leading space")
	ldif := b.ToLDIF()
	lines := strings.Split(strings.TrimSpace(ldif), "\n")
	if lines[0] != "dn: uid=alice,ou=Users,dc=example,dc=com" {
		t.Fatalf("dn line: %q", lines[0])
	}
	// objectClass values emitted first, before other attributes.
	if lines[1] != "objectClass: top" {
		t.Fatalf("objectClass first: %#v", lines)
	}
	if !strings.Contains(ldif, "cn: Alice") || !strings.Contains(ldif, "sn: Adams") {
		t.Fatalf("attrs: %s", ldif)
	}
	// leading-space value is base64-encoded.
	if !strings.Contains(ldif, "description:: ") {
		t.Fatalf("base64 value: %s", ldif)
	}
}

func TestNeedsBase64(t *testing.T) {
	cases := map[string]bool{
		"":            false,
		"plain":       false,
		" leading":    true,
		":colon":      true,
		"<less":       true,
		"trailing ":   true,
		"with\nnl":    true,
		"café":        true, // non-ASCII
		"nul\x00here": true,
	}
	for v, want := range cases {
		if got := needsBase64(v); got != want {
			t.Errorf("needsBase64(%q)=%v want %v", v, got, want)
		}
	}
}

func TestLDIFLine(t *testing.T) {
	if ldifLine("cn", "Alice") != "cn: Alice" {
		t.Fatal("plain")
	}
	if !strings.HasPrefix(ldifLine("cn", " x"), "cn:: ") {
		t.Fatal("b64")
	}
}

func TestParseLDIF(t *testing.T) {
	src := "version: 1\n" +
		"# a comment\n" +
		"dn: uid=alice,ou=Users,dc=example,dc=com\n" +
		"objectClass: person\n" +
		"cn: Alice\n" +
		" Adams\n" + // folded continuation onto cn
		"description:: " + b64("café") + "\n" +
		"\n" +
		"dn: uid=bob,ou=Users,dc=example,dc=com\n" +
		"cn: Bob\n"
	recs, err := ParseLDIF(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 2 {
		t.Fatalf("records: %d", len(recs))
	}
	if recs[0].DN != "uid=alice,ou=Users,dc=example,dc=com" {
		t.Fatalf("dn: %q", recs[0].DN)
	}
	if recs[0].Attributes["cn"][0] != "AliceAdams" {
		t.Fatalf("folded cn: %#v", recs[0].Attributes["cn"])
	}
	if recs[0].Attributes["description"][0] != "café" {
		t.Fatalf("b64 decode: %#v", recs[0].Attributes["description"])
	}
}

func TestParseLDIFErrors(t *testing.T) {
	if _, err := ParseLDIF("cn: Alice\n"); err == nil {
		t.Fatal("attr before dn")
	}
	if _, err := ParseLDIF("dn: x\nnocolon\n"); err == nil {
		t.Fatal("line without colon")
	}
	if _, err := ParseLDIF("dn:: not!base64!\n"); err == nil {
		t.Fatal("bad base64")
	}
	// A final record with an empty dn value errors at the closing flush (no
	// trailing blank line, so it is the end-of-stream flush that catches it).
	if _, err := ParseLDIF("dn:\ncn: x"); err == nil {
		t.Fatal("empty dn record (final flush)")
	}
	// An empty-dn record followed by a new dn: errors at the record-boundary flush.
	if _, err := ParseLDIF("dn:\ndn: uid=x,dc=com\ncn: y"); err == nil {
		t.Fatal("empty dn record (boundary flush)")
	}
	// An empty-dn record followed by a blank line errors at the blank-line flush.
	if _, err := ParseLDIF("dn:\n\n"); err == nil {
		t.Fatal("empty dn record (blank-line flush)")
	}
}

func TestUnfoldAndParseLine(t *testing.T) {
	lines := unfoldLDIF("a: 1\n b\nc: 2\r\n")
	if lines[0] != "a: 1b" || lines[1] != "c: 2" {
		t.Fatalf("unfold: %#v", lines)
	}
	name, val, b, err := parseLDIFLine("cn: Alice")
	if err != nil || name != "cn" || val != "Alice" || b {
		t.Fatalf("plain line: %q %q %v %v", name, val, b, err)
	}
	_, _, b2, _ := parseLDIFLine("cn:: " + b64("x"))
	if !b2 {
		t.Fatal("b64 flag")
	}
}

func TestLoadLDIF(t *testing.T) {
	c, dir := newPersonClass(t)
	// dn_attribute (uid) is absent from the attributes but present in the DN;
	// LoadLDIF derives it from the DN.
	src := "dn: uid=carol,ou=Users,dc=example,dc=com\n" +
		"objectClass: top\nobjectClass: person\nobjectClass: inetOrgPerson\n" +
		"cn: Carol\n"
	recs, err := c.LoadLDIF(src)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(recs) != 1 || recs[0].ID() != "carol" || !recs[0].Persisted() {
		t.Fatalf("loaded: %#v", recs)
	}
	if !containsLog(dir.Log, "add uid=carol,ou=Users,dc=example,dc=com") {
		t.Fatalf("log: %#v", dir.Log)
	}
	// parse error path
	if _, err := c.LoadLDIF("cn: x\n"); err == nil {
		t.Fatal("parse error")
	}
	// save error path (add fails)
	dir.FailOn["add"] = "boom"
	if _, err := c.LoadLDIF("dn: uid=dan,ou=Users,dc=example,dc=com\nobjectClass: top\nobjectClass: person\nobjectClass: inetOrgPerson\ncn: Dan\n"); err == nil {
		t.Fatal("save error")
	}
}

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
