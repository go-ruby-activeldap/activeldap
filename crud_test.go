// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"reflect"
	"strings"
	"testing"
)

func seedPeople(c *Class, dir *MockDirectory) {
	dir.Seed("uid=alice,ou=Users,dc=example,dc=com", map[string][]string{
		"uid": {"alice"}, "cn": {"Alice"}, "mail": {"alice@x"}, "objectClass": {"top", "person", "inetOrgPerson"},
	})
	dir.Seed("uid=bob,ou=Users,dc=example,dc=com", map[string][]string{
		"uid": {"bob"}, "cn": {"Bob"}, "objectClass": {"top", "person", "inetOrgPerson"},
	})
}

func TestConditions(t *testing.T) {
	if Conditions(nil) != nil {
		t.Fatal("nil conds")
	}
	if Conditions(map[string][]string{"uid": {"alice"}}).String() != "(uid=alice)" {
		t.Fatal("conds")
	}
}

func TestSearchRequest(t *testing.T) {
	c, _ := newPersonClass(t)
	one := ScopeOne
	req := c.searchRequest(FindOptions{Scope: &one, Base: "ou=Other", Filter: Equal("mail", "x")})
	if req.Scope != ScopeOne || req.Base != "ou=Other" {
		t.Fatal("overrides")
	}
	if req.Filter != "(&(&(objectClass=top)(objectClass=person)(objectClass=inetOrgPerson))(mail=x))" {
		t.Fatalf("filter: %q", req.Filter)
	}
	// defaults
	req2 := c.searchRequest(FindOptions{})
	if req2.Base != c.BaseDN() || req2.Scope != ScopeSub {
		t.Fatal("defaults")
	}
}

func TestSearchFindAllFirst(t *testing.T) {
	c, dir := newPersonClass(t)
	seedPeople(c, dir)
	all, err := c.FindAll(FindOptions{})
	if err != nil || len(all) != 2 {
		t.Fatalf("all: %d %v", len(all), err)
	}
	limited, _ := c.Search(FindOptions{Limit: 1})
	if len(limited) != 1 {
		t.Fatalf("limit: %d", len(limited))
	}
	first, _ := c.FindFirst(FindOptions{Filter: Equal("uid", "bob")})
	if first == nil || first.ID() != "bob" {
		t.Fatal("first")
	}
	none, _ := c.FindFirst(FindOptions{Filter: Equal("uid", "ghost")})
	if none != nil {
		t.Fatal("none")
	}
	// search error propagation
	dir.FailOn["search"] = "boom"
	if _, err := c.Search(FindOptions{}); err == nil {
		t.Fatal("expected search error")
	}
	if _, err := c.FindFirst(FindOptions{}); err == nil {
		t.Fatal("expected first error")
	}
}

func TestFindAndExist(t *testing.T) {
	c, dir := newPersonClass(t)
	seedPeople(c, dir)
	b, err := c.Find("alice")
	if err != nil || b.One("cn") != "Alice" {
		t.Fatalf("find: %v", err)
	}
	_, err = c.Find("ghost")
	if _, ok := err.(*EntryNotFoundError); !ok {
		t.Fatalf("expected EntryNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Fatal("error message")
	}
	ex, _ := c.Exist("alice")
	if !ex {
		t.Fatal("exist true")
	}
	nx, _ := c.Exist("ghost")
	if nx {
		t.Fatal("exist false")
	}
	// Exist surfaces non-notfound errors.
	dir.FailOn["search"] = "boom"
	if _, err := c.Exist("alice"); err == nil {
		t.Fatal("exist error")
	}
	if _, err := c.Find("alice"); err == nil {
		t.Fatal("find error")
	}
}

func TestCreateAndSaveAdd(t *testing.T) {
	c, dir := newPersonClass(t)
	b, err := c.Create(map[string][]string{"uid": {"carol"}, "cn": {"Carol"}, "sn": {"C"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !b.Persisted() {
		t.Fatal("persisted after create")
	}
	if !containsLog(dir.Log, "add uid=carol,ou=Users,dc=example,dc=com") {
		t.Fatalf("add log: %#v", dir.Log)
	}
	// invalid create -> ValidationError, not persisted
	inv, err := c.Create(map[string][]string{"cn": {"NoUID"}})
	if err == nil || inv.Persisted() {
		t.Fatal("invalid create should fail")
	}
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError: %v", err)
	}
}

func TestSaveModifyDiff(t *testing.T) {
	c, dir := newPersonClass(t)
	seedPeople(c, dir)
	b, _ := c.Find("alice")
	dir.Log = nil
	// no changes -> save is a no-op (no modify op).
	if err := b.Save(); err != nil {
		t.Fatal(err)
	}
	if len(dir.Log) != 0 {
		t.Fatalf("no-op save logged: %#v", dir.Log)
	}
	// change + remove -> replace + delete ops.
	b.Set("cn", "Alice A")
	b.Delete("mail")
	if !b.SaveOK() {
		t.Fatal("save ok")
	}
	if !containsLog(dir.Log, "modify replace uid=alice,ou=Users,dc=example,dc=com cn") ||
		!containsLog(dir.Log, "modify delete uid=alice,ou=Users,dc=example,dc=com mail") {
		t.Fatalf("modify log: %#v", dir.Log)
	}
	if b.Changed() {
		t.Fatal("baseline reset after save")
	}
}

func TestModifyOpsSkipDNAttr(t *testing.T) {
	c, dir := newPersonClass(t)
	seedPeople(c, dir)
	b, _ := c.Find("alice")
	b.Set("uid", "alice2") // dn attribute change is not emitted as a modify
	ops := b.modifyOps()
	for _, op := range ops {
		if op.Attribute == "uid" {
			t.Fatal("dn attribute should be skipped")
		}
	}
}

func TestSaveErrorPaths(t *testing.T) {
	c, dir := newPersonClass(t)
	// invalid save
	nb := c.New()
	if err := nb.Save(); err == nil {
		t.Fatal("invalid save")
	}
	// add error
	dir.FailOn["add"] = "boom"
	valid := c.New()
	valid.SetID("z")
	if err := valid.Save(); err == nil {
		t.Fatal("add error")
	}
	delete(dir.FailOn, "add")
	// modify error
	seedPeople(c, dir)
	b, _ := c.Find("alice")
	b.Set("cn", "changed")
	dir.FailOn["modify"] = "boom"
	if err := b.Save(); err == nil {
		t.Fatal("modify error")
	}
}

func TestUpdateAttributes(t *testing.T) {
	c, dir := newPersonClass(t)
	seedPeople(c, dir)
	b, _ := c.Find("alice")
	if err := b.UpdateAttributes(map[string][]string{"cn": {"Alice Updated"}}); err != nil {
		t.Fatal(err)
	}
	if b.One("cn") != "Alice Updated" {
		t.Fatal("update")
	}
}

func TestDestroyAndReload(t *testing.T) {
	c, dir := newPersonClass(t)
	seedPeople(c, dir)
	// new record cannot be destroyed
	nb := c.New()
	if err := nb.Destroy(); err == nil {
		t.Fatal("new destroy")
	}
	b, _ := c.Find("alice")
	// reload found
	b.Set("cn", "temp")
	if err := b.Reload(); err != nil {
		t.Fatal(err)
	}
	if b.One("cn") != "Alice" {
		t.Fatal("reload discards changes")
	}
	// destroy error
	dir.FailOn["delete"] = "boom"
	if err := b.Destroy(); err == nil {
		t.Fatal("destroy error")
	}
	delete(dir.FailOn, "delete")
	if err := b.Destroy(); err != nil {
		t.Fatal(err)
	}
	if b.Persisted() {
		t.Fatal("not persisted after destroy")
	}
	// reload of a gone entry errors
	if err := b.Reload(); err == nil {
		t.Fatal("reload gone")
	}
}

func containsLog(log []string, want string) bool {
	for _, l := range log {
		if l == want {
			return true
		}
	}
	return false
}

func TestEntryNotFoundError(t *testing.T) {
	e := &EntryNotFoundError{Class: "Person", ID: "x"}
	if !reflect.DeepEqual(e.Class, "Person") || e.ID != "x" {
		t.Fatal("fields")
	}
}
