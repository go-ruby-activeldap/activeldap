// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import "strings"

// RDN is a single relative distinguished name component — one attribute=value
// pair such as cn=Alice. Multi-valued RDNs (cn=Alice+uid=alice) are not part of
// the small model ActiveLdap's dn_attribute mapping produces, so an RDN holds
// exactly one pair.
type RDN struct {
	Attribute string
	Value     string
}

// String renders the RDN as attribute=value with the value DN-escaped per
// RFC 4514 (leading/trailing space, and any of ,+"\<>; escaped).
func (r RDN) String() string { return r.Attribute + "=" + escapeDNValue(r.Value) }

// DN is a parsed distinguished name — an ordered list of RDNs, most specific
// first, exactly as ActiveLdap's DN wraps a sequence of components.
type DN struct{ RDNs []RDN }

// ParseDN parses a distinguished name string into a [DN]. Unescaped commas
// separate RDNs and the first unescaped '=' splits each RDN; surrounding
// whitespace around a component is trimmed. An empty string parses to the empty
// (root) DN. A component with no '=' makes ParseDN return ok=false.
func ParseDN(s string) (DN, bool) {
	var dn DN
	if strings.TrimSpace(s) == "" {
		return dn, true
	}
	for _, part := range splitUnescaped(s, ',') {
		part = strings.TrimSpace(part)
		if part == "" {
			return DN{}, false
		}
		eq := indexUnescaped(part, '=')
		if eq < 0 {
			return DN{}, false
		}
		attr := strings.TrimSpace(part[:eq])
		val := unescapeDNValue(strings.TrimSpace(part[eq+1:]))
		if attr == "" {
			return DN{}, false
		}
		dn.RDNs = append(dn.RDNs, RDN{Attribute: attr, Value: val})
	}
	return dn, true
}

// String renders the DN as a comma-joined, RFC 4514-escaped string.
func (d DN) String() string {
	parts := make([]string, len(d.RDNs))
	for i, r := range d.RDNs {
		parts[i] = r.String()
	}
	return strings.Join(parts, ",")
}

// Normalized returns a canonical, case-folded form used for DN equality: each
// attribute name is lower-cased and each value is lower-cased and
// whitespace-collapsed. ActiveLdap compares DNs case-insensitively, so two DNs
// are "equal" when their Normalized strings match.
func (d DN) Normalized() string {
	parts := make([]string, len(d.RDNs))
	for i, r := range d.RDNs {
		v := strings.Join(strings.Fields(strings.ToLower(r.Value)), " ")
		parts[i] = strings.ToLower(r.Attribute) + "=" + v
	}
	return strings.Join(parts, ",")
}

// Equal reports whether two DNs denote the same entry, comparing case- and
// whitespace-insensitively via [DN.Normalized].
func (d DN) Equal(o DN) bool { return d.Normalized() == o.Normalized() }

// Parent returns the DN with its most specific RDN removed — the DN of the entry
// one level up. The root DN's parent is the root DN.
func (d DN) Parent() DN {
	if len(d.RDNs) == 0 {
		return d
	}
	return DN{RDNs: append([]RDN(nil), d.RDNs[1:]...)}
}

// BuildDN composes a full DN from a leaf RDN (attribute=value), an optional
// prefix (an ActiveLdap "prefix" such as ou=Users, itself a DN), and a base DN.
// Empty prefix or base segments are skipped. The result is the ActiveLdap
// dn = "<dn_attribute>=<value>,<prefix>,<base>".
func BuildDN(attr, value, prefix, base string) string {
	segs := []string{RDN{Attribute: attr, Value: value}.String()}
	if strings.TrimSpace(prefix) != "" {
		segs = append(segs, strings.TrimSpace(prefix))
	}
	if strings.TrimSpace(base) != "" {
		segs = append(segs, strings.TrimSpace(base))
	}
	return strings.Join(segs, ",")
}

// escapeDNValue escapes an RDN value per RFC 4514: a leading '#', a leading or
// trailing space, and any of the characters ,+"\<>;= and NUL are backslash- or
// hex-escaped. It is deliberately conservative and reversible by
// unescapeDNValue.
func escapeDNValue(v string) string {
	if v == "" {
		return v
	}
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c == '\x00':
			b.WriteString(`\00`)
		case strings.IndexByte(`,+"\<>;=`, c) >= 0:
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == ' ' && (i == 0 || i == len(v)-1):
			b.WriteString(`\ `)
		case c == '#' && i == 0:
			b.WriteString(`\#`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// unescapeDNValue reverses escapeDNValue: "\XX" is a two-hex-digit byte and "\c"
// is the literal character c. A trailing lone backslash is dropped.
func unescapeDNValue(v string) string {
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		if v[i] != '\\' {
			b.WriteByte(v[i])
			continue
		}
		if isHex(v, i+1) {
			b.WriteByte(hexByte(v[i+1], v[i+2]))
			i += 2
			continue
		}
		if i+1 < len(v) {
			b.WriteByte(v[i+1])
			i++
			continue
		}
		// trailing lone backslash: drop it
	}
	return b.String()
}

func isHex(s string, i int) bool {
	if i+1 >= len(s) {
		return false
	}
	return isHexDigit(s[i]) && isHexDigit(s[i+1])
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func hexNibble(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

func hexByte(hi, lo byte) byte { return hexNibble(hi)<<4 | hexNibble(lo) }

// splitUnescaped splits s on unescaped occurrences of sep (a backslash escapes
// the following byte, including sep and another backslash).
func splitUnescaped(s string, sep byte) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			cur.WriteByte(s[i])
			cur.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == sep {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	out = append(out, cur.String())
	return out
}

// indexUnescaped returns the index of the first unescaped sep in s, or -1.
func indexUnescaped(s string, sep byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			continue
		}
		if s[i] == sep {
			return i
		}
	}
	return -1
}
