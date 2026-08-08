// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"fmt"
	"strings"
)

// ParseFilter parses an RFC 4515 filter string into a [Filter] tree. It handles
// the operators & | ! and the leaves attr=value, attr=* (present) and substring
// (attr=a*b*c), unescaping \\XX and \\c in assertion values. It is the inverse
// of [Filter.String] for the subset this ORM emits, letting [MockDirectory]
// evaluate the very filters the query builder produces. A malformed filter
// returns an error.
func ParseFilter(s string) (Filter, error) { return parseFilterString(s) }

func parseFilterString(s string) (Filter, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Present("objectClass"), nil
	}
	p := &filterParser{src: s}
	f, err := p.parse()
	if err != nil {
		return nil, err
	}
	p.ws()
	if p.pos != len(p.src) {
		return nil, fmt.Errorf("trailing characters in filter at %d", p.pos)
	}
	return f, nil
}

type filterParser struct {
	src string
	pos int
}

func (p *filterParser) ws() {
	for p.pos < len(p.src) && p.src[p.pos] == ' ' {
		p.pos++
	}
}

func (p *filterParser) parse() (Filter, error) {
	p.ws()
	if p.pos >= len(p.src) || p.src[p.pos] != '(' {
		return nil, fmt.Errorf("expected '(' at %d", p.pos)
	}
	p.pos++ // consume '('
	if p.pos >= len(p.src) {
		return nil, fmt.Errorf("unterminated filter")
	}
	var f Filter
	var err error
	switch p.src[p.pos] {
	case '&':
		f, err = p.parseGroup('&')
	case '|':
		f, err = p.parseGroup('|')
	case '!':
		p.pos++
		var sub Filter
		sub, err = p.parse()
		f = Not(sub)
	default:
		f, err = p.parseLeaf()
	}
	if err != nil {
		return nil, err
	}
	if p.pos >= len(p.src) || p.src[p.pos] != ')' {
		return nil, fmt.Errorf("expected ')' at %d", p.pos)
	}
	p.pos++ // consume ')'
	return f, nil
}

func (p *filterParser) parseGroup(op byte) (Filter, error) {
	p.pos++ // consume operator
	var subs []Filter
	for p.pos < len(p.src) && p.src[p.pos] == '(' {
		sub, err := p.parse()
		if err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	if len(subs) == 0 {
		return nil, fmt.Errorf("empty group at %d", p.pos)
	}
	if op == '&' {
		return And(subs...), nil
	}
	return Or(subs...), nil
}

func (p *filterParser) parseLeaf() (Filter, error) {
	start := p.pos
	eq := -1
	for p.pos < len(p.src) && p.src[p.pos] != ')' {
		if p.src[p.pos] == '=' && eq < 0 {
			eq = p.pos
		}
		p.pos++
	}
	if eq < 0 {
		return nil, fmt.Errorf("expected '=' in leaf at %d", start)
	}
	attr := strings.TrimSpace(p.src[start:eq])
	val := p.src[eq+1 : p.pos]
	if attr == "" {
		return nil, fmt.Errorf("empty attribute in leaf at %d", start)
	}
	if val == "*" {
		return Present(attr), nil
	}
	if strings.Contains(val, "*") {
		segs := strings.Split(val, "*")
		for i := range segs {
			segs[i] = unescapeFilterValue(segs[i])
		}
		return Substring(attr, segs[0], segs[1:len(segs)-1], segs[len(segs)-1]), nil
	}
	return Equal(attr, unescapeFilterValue(val)), nil
}

// unescapeFilterValue reverses [EscapeFilterValue]: \XX two-hex-digit bytes and
// \c literal characters.
func unescapeFilterValue(v string) string {
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
		}
	}
	return b.String()
}

// evalFilter reports whether entry e satisfies filter f. Comparisons are
// case-insensitive (LDAP's default caseIgnoreMatch), matching how ActiveLdap's
// mock and most string-syntax attributes compare.
func evalFilter(f Filter, e *Entry) bool {
	switch n := f.(type) {
	case eqFilter:
		return containsFold(e.Get(n.attr), n.value)
	case presentFilter:
		return len(e.Get(n.attr)) > 0
	case substringFilter:
		return matchSubstring(e.Get(n.attr), n)
	case andFilter:
		for _, s := range n.subs {
			if !evalFilter(s, e) {
				return false
			}
		}
		return true
	case orFilter:
		for _, s := range n.subs {
			if evalFilter(s, e) {
				return true
			}
		}
		return false
	case notFilter:
		return !evalFilter(n.sub, e)
	case rawFilter:
		parsed, err := parseFilterString(n.text)
		if err != nil {
			return false
		}
		return evalFilter(parsed, e)
	default:
		return false
	}
}

func containsFold(values []string, want string) bool {
	wf := strings.ToLower(want)
	for _, v := range values {
		if strings.ToLower(v) == wf {
			return true
		}
	}
	return false
}

func matchSubstring(values []string, n substringFilter) bool {
	for _, v := range values {
		lv := strings.ToLower(v)
		rest := lv
		if n.initial != "" {
			if !strings.HasPrefix(rest, strings.ToLower(n.initial)) {
				continue
			}
			rest = rest[len(n.initial):]
		}
		ok := true
		for _, a := range n.any {
			idx := strings.Index(rest, strings.ToLower(a))
			if idx < 0 {
				ok = false
				break
			}
			rest = rest[idx+len(a):]
		}
		if !ok {
			continue
		}
		if n.final != "" && !strings.HasSuffix(rest, strings.ToLower(n.final)) {
			continue
		}
		return true
	}
	return false
}
