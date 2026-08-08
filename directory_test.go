// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"reflect"
	"testing"
)

func TestScopeStringAndParse(t *testing.T) {
	if ScopeBase.String() != "base" || ScopeOne.String() != "one" || ScopeSub.String() != "sub" {
		t.Fatal("scope strings")
	}
	if Scope(99).String() != "Scope(99)" {
		t.Fatalf("unknown scope: %q", Scope(99).String())
	}
	for name, want := range map[string]Scope{
		"base": ScopeBase, "baseObject": ScopeBase,
		"one": ScopeOne, "singleLevel": ScopeOne,
		"sub": ScopeSub, "subtree": ScopeSub, "": ScopeSub,
	} {
		if got, ok := ParseScope(name); !ok || got != want {
			t.Errorf("ParseScope(%q)=%v ok=%v", name, got, ok)
		}
	}
	if _, ok := ParseScope("bogus"); ok {
		t.Fatal("bogus scope should fail")
	}
}

func TestModOpString(t *testing.T) {
	if ModAdd.String() != "add" || ModReplace.String() != "replace" || ModDelete.String() != "delete" {
		t.Fatal("modop strings")
	}
	if ModOp(42).String() != "ModOp(42)" {
		t.Fatalf("unknown modop: %q", ModOp(42).String())
	}
}

func TestEntryGet(t *testing.T) {
	e := &Entry{Attributes: map[string][]string{"CN": {"x"}}}
	if got := e.Get("cn"); !reflect.DeepEqual(got, []string{"x"}) {
		t.Fatalf("get: %#v", got)
	}
	if e.Get("missing") != nil {
		t.Fatal("absent nil")
	}
}

func TestNormDNAndParent(t *testing.T) {
	if normDN("UID=Alice, DC=Com") != "uid=alice,dc=com" {
		t.Fatalf("normDN: %q", normDN("UID=Alice, DC=Com"))
	}
	// unparseable falls back to fold.
	if normDN("bogus") != "bogus" {
		t.Fatalf("bad normDN: %q", normDN("bogus"))
	}
	if ParseDNParent("uid=alice,ou=users") != "ou=users" {
		t.Fatal("parent")
	}
	if ParseDNParent("dc=com") != "" {
		t.Fatal("no-comma parent")
	}
}

func newTestDir(t *testing.T) *MockDirectory {
	t.Helper()
	d := NewMockDirectory()
	d.Seed("uid=alice,ou=Users,dc=com", map[string][]string{
		"uid": {"alice"}, "cn": {"Alice"}, "objectClass": {"person"},
	})
	d.Seed("uid=bob,ou=Users,dc=com", map[string][]string{
		"uid": {"bob"}, "cn": {"Bob"}, "objectClass": {"person"},
	})
	d.Seed("ou=Users,dc=com", map[string][]string{
		"ou": {"Users"}, "objectClass": {"organizationalUnit"},
	})
	return d
}

func TestMockSearchScopes(t *testing.T) {
	d := newTestDir(t)
	sub, _ := d.Search(SearchRequest{Base: "ou=Users,dc=com", Scope: ScopeSub, Filter: "(objectClass=person)"})
	if len(sub) != 2 {
		t.Fatalf("sub: %d", len(sub))
	}
	one, _ := d.Search(SearchRequest{Base: "ou=Users,dc=com", Scope: ScopeOne, Filter: "(objectClass=*)"})
	if len(one) != 2 { // alice, bob are children; ou itself excluded
		t.Fatalf("one: %d", len(one))
	}
	base, _ := d.Search(SearchRequest{Base: "uid=alice,ou=Users,dc=com", Scope: ScopeBase, Filter: "(objectClass=*)"})
	if len(base) != 1 || base[0].Get("uid")[0] != "alice" {
		t.Fatalf("base: %#v", base)
	}
	// empty base with sub matches everything.
	all, _ := d.Search(SearchRequest{Base: "", Scope: ScopeSub, Filter: "(objectClass=*)"})
	if len(all) != 3 {
		t.Fatalf("empty base sub: %d", len(all))
	}
}

func TestMockSearchProjectionAndErrors(t *testing.T) {
	d := newTestDir(t)
	proj, _ := d.Search(SearchRequest{Base: "ou=Users,dc=com", Scope: ScopeSub, Filter: "(uid=alice)", Attributes: []string{"cn"}})
	if len(proj) != 1 || proj[0].Get("uid") != nil || proj[0].Get("cn")[0] != "Alice" {
		t.Fatalf("projection: %#v", proj[0].Attributes)
	}
	if _, err := d.Search(SearchRequest{Filter: "(("}); err == nil {
		t.Fatal("expected filter parse error")
	}
	d.FailOn["search"] = "boom"
	if _, err := d.Search(SearchRequest{Filter: "(objectClass=*)"}); err == nil {
		t.Fatal("expected search fail")
	}
}

func TestMockAdd(t *testing.T) {
	d := NewMockDirectory()
	if err := d.Add("uid=x,dc=com", map[string][]string{"uid": {"x"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.Add("uid=x,dc=com", nil); err == nil {
		t.Fatal("expected exists error")
	}
	d.FailOn["add"] = "boom"
	if err := d.Add("uid=y,dc=com", nil); err == nil {
		t.Fatal("expected add fail")
	}
}

func TestMockModify(t *testing.T) {
	d := newTestDir(t)
	dn := "uid=alice,ou=Users,dc=com"
	err := d.Modify(dn, []ModifyOp{
		{Op: ModReplace, Attribute: "cn", Values: []string{"Alice A"}},
		{Op: ModAdd, Attribute: "mail", Values: []string{"a@x"}},
		{Op: ModDelete, Attribute: "cn", Values: []string{"nomatch"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := d.Search(SearchRequest{Base: dn, Scope: ScopeBase, Filter: "(objectClass=*)"})
	if got[0].Get("cn")[0] != "Alice A" || got[0].Get("mail")[0] != "a@x" {
		t.Fatalf("modify result: %#v", got[0].Attributes)
	}
	// replace with empty deletes; delete whole attr.
	if err := d.Modify(dn, []ModifyOp{{Op: ModReplace, Attribute: "mail"}, {Op: ModDelete, Attribute: "cn"}}); err != nil {
		t.Fatal(err)
	}
	got2, _ := d.Search(SearchRequest{Base: dn, Scope: ScopeBase, Filter: "(objectClass=*)"})
	if got2[0].Get("mail") != nil || got2[0].Get("cn") != nil {
		t.Fatalf("delete result: %#v", got2[0].Attributes)
	}
	if err := d.Modify("uid=ghost,dc=com", nil); err == nil {
		t.Fatal("expected no-entry error")
	}
	d.FailOn["modify"] = "boom"
	if err := d.Modify(dn, nil); err == nil {
		t.Fatal("expected modify fail")
	}
}

func TestMockDelete(t *testing.T) {
	d := newTestDir(t)
	if err := d.Delete("uid=alice,ou=Users,dc=com"); err != nil {
		t.Fatal(err)
	}
	if err := d.Delete("uid=alice,ou=Users,dc=com"); err == nil {
		t.Fatal("expected no-entry error")
	}
	d.FailOn["delete"] = "boom"
	if err := d.Delete("uid=bob,ou=Users,dc=com"); err == nil {
		t.Fatal("expected delete fail")
	}
}

func TestRemoveValuesAndFindKey(t *testing.T) {
	if got := removeValues([]string{"a", "b", "c"}, []string{"b"}); !reflect.DeepEqual(got, []string{"a", "c"}) {
		t.Fatalf("removeValues: %#v", got)
	}
	e := &Entry{Attributes: map[string][]string{"CN": {"x"}}}
	if findAttrKey(e, "cn") != "CN" {
		t.Fatal("findAttrKey existing")
	}
	if findAttrKey(e, "new") != "new" {
		t.Fatal("findAttrKey new")
	}
}
