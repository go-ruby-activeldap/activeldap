// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"reflect"
	"strings"
	"testing"
)

func TestErrors(t *testing.T) {
	e := newErrors()
	if !e.Empty() {
		t.Fatal("empty")
	}
	e.Add("cn", "can't be blank")
	e.Add("cn", "too short")
	e.Add(ErrorsBase, "record invalid")
	if e.Empty() || e.Count() != 3 {
		t.Fatalf("count: %d", e.Count())
	}
	if got := e.On("CN"); !reflect.DeepEqual(got, []string{"can't be blank", "too short"}) {
		t.Fatalf("on: %#v", got)
	}
	fm := e.FullMessages()
	if fm[0] != "cn can't be blank" || fm[2] != "record invalid" {
		t.Fatalf("full: %#v", fm)
	}
	e.clear()
	if !e.Empty() {
		t.Fatal("clear")
	}
}

func TestPresenceOfAndRequiredClasses(t *testing.T) {
	c, _ := newPersonClass(t)
	b := c.New()
	PresenceOf("cn", "sn")(b)
	if b.errors.Count() != 2 {
		t.Fatalf("presence: %d", b.errors.Count())
	}
	b.errors.clear()
	b.Set("cn", "Alice")
	PresenceOf("cn")(b)
	if b.errors.Count() != 0 {
		t.Fatal("presence present")
	}
	b.errors.clear()
	RequiredClasses("posixAccount")(b)
	if b.errors.Count() != 1 {
		t.Fatal("required class missing")
	}
}

func TestBlank(t *testing.T) {
	if !blank(nil) || !blank([]string{"  "}) {
		t.Fatal("blank true")
	}
	if blank([]string{"x"}) {
		t.Fatal("blank false")
	}
}

func TestValid(t *testing.T) {
	c, _ := newPersonClass(t)
	b := c.New()
	// missing uid -> invalid
	if b.Valid() {
		t.Fatal("should be invalid without uid")
	}
	if len(b.Errors().On("uid")) == 0 {
		t.Fatal("uid error")
	}
	b.SetID("alice")
	if !b.Valid() {
		t.Fatalf("should be valid: %#v", b.Errors().FullMessages())
	}
	// custom validator
	c.Validate(func(rec *Base) {
		if rec.One("cn") == "" {
			rec.Errors().Add("cn", "required by policy")
		}
	})
	if b.Valid() {
		t.Fatal("custom validator should fail")
	}
	if !strings.Contains(strings.Join(b.Errors().FullMessages(), " "), "policy") {
		t.Fatal("custom message")
	}
}

func TestValidRequiredClassBranch(t *testing.T) {
	// A record missing a mapped objectClass reports it.
	c, _ := newPersonClass(t)
	b := c.New()
	b.SetID("alice")
	b.Set("objectClass", "top") // drop person/inetOrgPerson
	if b.Valid() {
		t.Fatal("missing mapped classes -> invalid")
	}
	msgs := strings.Join(b.Errors().FullMessages(), " ")
	if !strings.Contains(msgs, "person") {
		t.Fatalf("expected class error: %v", msgs)
	}
}

func TestValidationErrorMessage(t *testing.T) {
	c, _ := newPersonClass(t)
	b := c.New()
	ve := &ValidationError{Record: b, Messages: []string{"b msg", "a msg"}}
	if ve.Error() != "Person invalid: a msg, b msg" {
		t.Fatalf("error: %q", ve.Error())
	}
}
