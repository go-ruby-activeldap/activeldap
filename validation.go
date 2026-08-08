// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"fmt"
	"sort"
	"strings"
)

// Errors collects validation messages per attribute, the Go form of
// ActiveLdap/ActiveModel's errors object. The special key [ErrorsBase] holds
// record-level messages not tied to one attribute.
type Errors struct {
	byAttr map[string][]string
	order  []string
}

// ErrorsBase is the pseudo-attribute under which record-level errors are filed.
const ErrorsBase = "base"

func newErrors() *Errors { return &Errors{byAttr: map[string][]string{}} }

// Add files a message against an attribute (use [ErrorsBase] for record-level).
func (e *Errors) Add(attr, message string) {
	f := fold(attr)
	if _, seen := e.byAttr[f]; !seen {
		e.order = append(e.order, f)
	}
	e.byAttr[f] = append(e.byAttr[f], message)
}

// Empty reports whether there are no errors — the negation of ActiveModel's
// #any?.
func (e *Errors) Empty() bool { return len(e.byAttr) == 0 }

// Count returns the total number of messages.
func (e *Errors) Count() int {
	n := 0
	for _, msgs := range e.byAttr {
		n += len(msgs)
	}
	return n
}

// On returns the messages filed against an attribute.
func (e *Errors) On(attr string) []string {
	return append([]string(nil), e.byAttr[fold(attr)]...)
}

// FullMessages returns every message prefixed by its attribute, in insertion
// order — ActiveModel's #full_messages. Record-level ([ErrorsBase]) messages are
// emitted unprefixed.
func (e *Errors) FullMessages() []string {
	var out []string
	for _, f := range e.order {
		for _, m := range e.byAttr[f] {
			if f == ErrorsBase {
				out = append(out, m)
			} else {
				out = append(out, f+" "+m)
			}
		}
	}
	return out
}

func (e *Errors) clear() {
	e.byAttr = map[string][]string{}
	e.order = nil
}

// Validator inspects a record and files any problems on its [Errors]. Register
// one with [Class.Validate]; the built-ins ([PresenceOf], [RequiredClasses] and
// the mapping-derived DN/objectClass checks) are Validators too.
type Validator func(b *Base)

// Validate registers a custom validator on the class, run by [Base.Valid] after
// the built-in structural checks.
func (c *Class) Validate(v Validator) { c.validators = append(c.validators, v) }

// PresenceOf returns a validator asserting each named attribute has at least one
// non-blank value — ActiveLdap's validates_presence_of.
func PresenceOf(attrs ...string) Validator {
	return func(b *Base) {
		for _, a := range attrs {
			if blank(b.Get(a)) {
				b.errors.Add(a, "can't be blank")
			}
		}
	}
}

// RequiredClasses returns a validator asserting the record carries each named
// objectClass — the check ActiveLdap derives from a mapping's required classes.
func RequiredClasses(classes ...string) Validator {
	return func(b *Base) {
		for _, c := range classes {
			if !b.hasObjectClass(c) {
				b.errors.Add("objectClass", fmt.Sprintf("must include %s", c))
			}
		}
	}
}

func blank(values []string) bool {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

// Valid runs all validations and returns whether the record is error-free —
// ActiveLdap's #valid?. It clears prior errors first, then runs the structural
// checks (dn_attribute presence, mapped objectClasses present) followed by every
// registered custom validator, so [Base.Errors] reflects only the latest run.
func (b *Base) Valid() bool {
	b.errors.clear()
	// dn_attribute must be present to form a DN.
	if blank(b.dnAttributeValues()) {
		b.errors.Add(b.class.mapping.DNAttribute, "can't be blank")
	}
	// Every mapped objectClass must be present.
	RequiredClasses(b.class.mapping.Classes...)(b)
	for _, v := range b.class.validators {
		v(b)
	}
	return b.errors.Empty()
}

// ValidationError is the error a save returns when validation fails; its message
// is the joined full messages.
type ValidationError struct {
	Record   *Base
	Messages []string
}

func (e *ValidationError) Error() string {
	sorted := append([]string(nil), e.Messages...)
	sort.Strings(sorted)
	return e.Record.class.name + " invalid: " + strings.Join(sorted, ", ")
}
