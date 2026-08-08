// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"sort"
	"strings"
)

// Filter is an RFC 4515 LDAP search filter, the value ActiveLdap builds from a
// find/search :filter option (a String is used verbatim; a Hash of conditions is
// AND-combined) and passes to Net::LDAP. Filters compose with [And], [Or] and
// [Not]; leaves are built with [Equal], [Present] and [Substring]. String
// renders the parenthesised text.
type Filter interface {
	String() string
	// isFilter is an unexported marker keeping the interface closed to this
	// package's node types.
	isFilter()
}

type eqFilter struct{ attr, value string }
type presentFilter struct{ attr string }
type substringFilter struct {
	attr           string
	initial, final string
	any            []string
}
type andFilter struct{ subs []Filter }
type orFilter struct{ subs []Filter }
type notFilter struct{ sub Filter }
type rawFilter struct{ text string }

func (eqFilter) isFilter()        {}
func (presentFilter) isFilter()   {}
func (substringFilter) isFilter() {}
func (andFilter) isFilter()       {}
func (orFilter) isFilter()        {}
func (notFilter) isFilter()       {}
func (rawFilter) isFilter()       {}

// Equal builds an equality filter (attr=value), escaping the value per RFC 4515.
func Equal(attr, value string) Filter { return eqFilter{attr: attr, value: value} }

// Present builds a presence filter (attr=*).
func Present(attr string) Filter { return presentFilter{attr: attr} }

// Substring builds a substring filter (attr=initial*any*...*final). Any of
// initial, the any-parts, or final may be empty; an all-empty Substring is
// equivalent to Present.
func Substring(attr, initial string, anyParts []string, final string) Filter {
	return substringFilter{attr: attr, initial: initial, any: anyParts, final: final}
}

// And builds a conjunction (&...). With a single sub-filter it returns that
// sub-filter unchanged (no redundant &-wrapper), matching ActiveLdap's filter
// simplification; with none it returns the "match everything" objectClass=*
// present filter is left to callers — And of nothing renders as an empty group.
func And(subs ...Filter) Filter {
	subs = compact(subs)
	if len(subs) == 1 {
		return subs[0]
	}
	return andFilter{subs: subs}
}

// Or builds a disjunction (|...). A single sub-filter is returned unchanged.
func Or(subs ...Filter) Filter {
	subs = compact(subs)
	if len(subs) == 1 {
		return subs[0]
	}
	return orFilter{subs: subs}
}

// Not builds a negation (!...).
func Not(sub Filter) Filter { return notFilter{sub: sub} }

// RawFilter wraps a filter string the caller already has (the String form of a
// find/search :filter option), so it composes with the built nodes. It is
// emitted verbatim; if it is not already parenthesised it is wrapped in parens.
func RawFilter(text string) Filter { return rawFilter{text: strings.TrimSpace(text)} }

func compact(subs []Filter) []Filter {
	out := subs[:0:0]
	for _, s := range subs {
		if s != nil {
			out = append(out, s)
		}
	}
	return out
}

func (f eqFilter) String() string {
	return "(" + f.attr + "=" + EscapeFilterValue(f.value) + ")"
}

func (f presentFilter) String() string { return "(" + f.attr + "=*)" }

func (f substringFilter) String() string {
	parts := []string{EscapeFilterValue(f.initial)}
	for _, a := range f.any {
		parts = append(parts, EscapeFilterValue(a))
	}
	parts = append(parts, EscapeFilterValue(f.final))
	return "(" + f.attr + "=" + strings.Join(parts, "*") + ")"
}

func (f andFilter) String() string { return group('&', f.subs) }
func (f orFilter) String() string  { return group('|', f.subs) }
func (f notFilter) String() string { return "(!" + f.sub.String() + ")" }

func (f rawFilter) String() string {
	if strings.HasPrefix(f.text, "(") && strings.HasSuffix(f.text, ")") {
		return f.text
	}
	return "(" + f.text + ")"
}

func group(op byte, subs []Filter) string {
	var b strings.Builder
	b.WriteByte('(')
	b.WriteByte(op)
	for _, s := range subs {
		b.WriteString(s.String())
	}
	b.WriteByte(')')
	return b.String()
}

// EscapeFilterValue escapes an assertion value for an RFC 4515 filter: the
// characters * ( ) \ and NUL become \2a \28 \29 \5c \00. Everything else is
// passed through, so a caller may embed a literal '*' only via [Substring].
func EscapeFilterValue(v string) string {
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		switch v[i] {
		case '*':
			b.WriteString(`\2a`)
		case '(':
			b.WriteString(`\28`)
		case ')':
			b.WriteString(`\29`)
		case '\\':
			b.WriteString(`\5c`)
		case '\x00':
			b.WriteString(`\00`)
		default:
			b.WriteByte(v[i])
		}
	}
	return b.String()
}

// filterFromConditions builds an AND filter from a map of attribute→values
// conditions, exactly as ActiveLdap folds a Hash :filter into an equality
// conjunction. A key with multiple values becomes an OR of equalities for that
// attribute (attr in values). Keys are visited in sorted order for a
// deterministic, testable result. An empty map yields nil (no condition).
func filterFromConditions(conds map[string][]string) Filter {
	if len(conds) == 0 {
		return nil
	}
	keys := make([]string, 0, len(conds))
	for k := range conds {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var subs []Filter
	for _, k := range keys {
		vals := conds[k]
		switch len(vals) {
		case 0:
			subs = append(subs, Present(k))
		case 1:
			subs = append(subs, Equal(k, vals[0]))
		default:
			eqs := make([]Filter, len(vals))
			for i, v := range vals {
				eqs[i] = Equal(k, v)
			}
			subs = append(subs, Or(eqs...))
		}
	}
	return And(subs...)
}

// classesFilter builds the (&(objectClass=a)(objectClass=b)…) filter that
// restricts a search to entries carrying every mapped objectClass, the guard
// ActiveLdap adds to every find so a model only ever loads its own kind. With no
// classes it returns the (objectClass=*) present filter.
func classesFilter(classes []string) Filter {
	if len(classes) == 0 {
		return Present("objectClass")
	}
	subs := make([]Filter, len(classes))
	for i, c := range classes {
		subs[i] = Equal("objectClass", c)
	}
	return And(subs...)
}
