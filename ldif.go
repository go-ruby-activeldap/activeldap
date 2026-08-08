// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
)

// ToLDIF renders the record as an RFC 2849 LDIF entry — ActiveLdap's #to_ldif.
// The dn: line comes first, then every attribute value on its own
// "name: value" line, in canonical-name order with the objectClass values first
// (as ActiveLdap emits them). A value needing it (leading space/colon/<, a
// non-ASCII or control byte) is base64-encoded on a "name:: b64" line.
func (b *Base) ToLDIF() string {
	var lines []string
	lines = append(lines, ldifLine("dn", b.DN()))
	for _, v := range b.Get("objectClass") {
		lines = append(lines, ldifLine("objectClass", v))
	}
	names := append([]string(nil), b.attrs.Names()...)
	sort.Strings(names)
	for _, name := range names {
		if fold(name) == "objectclass" {
			continue
		}
		for _, v := range b.Get(name) {
			lines = append(lines, ldifLine(name, v))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// ldifLine renders one LDIF "name: value" line, base64-encoding the value when
// safe-string rules require it.
func ldifLine(name, value string) string {
	if needsBase64(value) {
		return name + ":: " + base64.StdEncoding.EncodeToString([]byte(value))
	}
	return name + ": " + value
}

// needsBase64 reports whether an LDIF value must be base64-encoded: it is empty
// after considering the leading char, begins with a space, ':' or '<', ends with
// a space, or contains a NUL/newline/non-ASCII byte.
func needsBase64(v string) bool {
	if v == "" {
		return false
	}
	switch v[0] {
	case ' ', ':', '<':
		return true
	}
	if v[len(v)-1] == ' ' {
		return true
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == 0 || c == '\n' || c == '\r' || c >= 0x80 {
			return true
		}
	}
	return false
}

// LDIFRecord is one entry parsed from an LDIF stream: its DN and attributes.
type LDIFRecord struct {
	DN         string
	Attributes map[string][]string
}

// ParseLDIF parses an RFC 2849 LDIF string into records. It handles line
// folding (a continuation line begins with a single space), comments (# …),
// blank-line record separators, and both "name: value" and base64
// "name:: value" forms. A record missing its dn: line is an error.
func ParseLDIF(s string) ([]LDIFRecord, error) {
	logical := unfoldLDIF(s)
	var records []LDIFRecord
	var cur *LDIFRecord
	flush := func() error {
		if cur == nil {
			return nil
		}
		if cur.DN == "" {
			return fmt.Errorf("ldif record missing dn")
		}
		records = append(records, *cur)
		cur = nil
		return nil
	}
	for _, line := range logical {
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		name, value, b64, err := parseLDIFLine(line)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(name, "version") {
			continue
		}
		if strings.EqualFold(name, "dn") {
			if err := flush(); err != nil {
				return nil, err
			}
			cur = &LDIFRecord{DN: value, Attributes: map[string][]string{}}
			continue
		}
		if cur == nil {
			return nil, fmt.Errorf("ldif attribute %q before dn", name)
		}
		_ = b64
		cur.Attributes[name] = append(cur.Attributes[name], value)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return records, nil
}

// parseLDIFLine splits one logical LDIF line into name, decoded value, and
// whether it was base64. "name: value" and "name:: b64" are supported; a line
// with no colon is an error.
func parseLDIFLine(line string) (name, value string, b64 bool, err error) {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return "", "", false, fmt.Errorf("ldif line without colon: %q", line)
	}
	name = line[:colon]
	rest := line[colon+1:]
	if strings.HasPrefix(rest, ":") { // "name:: base64"
		raw := strings.TrimSpace(rest[1:])
		dec, derr := base64.StdEncoding.DecodeString(raw)
		if derr != nil {
			return "", "", false, fmt.Errorf("ldif base64 decode %q: %w", name, derr)
		}
		return name, string(dec), true, nil
	}
	return name, strings.TrimSpace(rest), false, nil
}

// unfoldLDIF joins RFC 2849 continuation lines (those beginning with exactly one
// space) onto the preceding line, returning the logical lines.
func unfoldLDIF(s string) []string {
	raw := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	var out []string
	for _, l := range raw {
		if strings.HasPrefix(l, " ") && len(out) > 0 {
			out[len(out)-1] += l[1:]
			continue
		}
		out = append(out, l)
	}
	return out
}

// LoadLDIF parses an LDIF string and creates a record of this class for each
// entry, saving them through the connection — the Go form of ActiveLdap's LDIF
// import. It returns the created records; the first save error aborts and is
// returned with the records created so far.
func (c *Class) LoadLDIF(ldif string) ([]*Base, error) {
	records, err := ParseLDIF(ldif)
	if err != nil {
		return nil, err
	}
	var out []*Base
	for _, r := range records {
		b := c.New()
		for name, vals := range r.Attributes {
			b.Set(name, vals...)
		}
		// Ensure the dn_attribute is populated from the DN when absent.
		if blank(b.dnAttributeValues()) {
			if dn, ok := ParseDN(r.DN); ok && len(dn.RDNs) > 0 {
				b.Set(c.mapping.DNAttribute, dn.RDNs[0].Value)
			}
		}
		if err := b.Save(); err != nil {
			return out, err
		}
		out = append(out, b)
	}
	return out, nil
}
