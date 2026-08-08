// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import "testing"

// assocFixture builds a Person and Group class on a shared directory with a
// group referencing alice by DN and by gidNumber.
func assocFixture(t *testing.T) (person, group *Class, dir *MockDirectory) {
	t.Helper()
	dir = NewMockDirectory()
	conn := NewConnection(dir, "dc=example,dc=com")
	person = NewClass("Person", personMapping(), conn)
	group = NewClass("Group", &Mapping{
		DNAttribute: "cn", Prefix: "ou=Groups", Classes: []string{"groupOfNames"}, Scope: ScopeSub,
	}, conn)

	dir.Seed("uid=alice,ou=Users,dc=example,dc=com", map[string][]string{
		"uid": {"alice"}, "cn": {"Alice"}, "gidNumber": {"100"},
		"objectClass": {"top", "person", "inetOrgPerson"},
	})
	dir.Seed("cn=admins,ou=Groups,dc=example,dc=com", map[string][]string{
		"cn": {"admins"}, "gidNumber": {"100"},
		"member":      {"uid=alice,ou=Users,dc=example,dc=com"},
		"objectClass": {"groupOfNames"},
	})

	// has_many members: group.member DNs -> Person entries by DN.
	group.HasMany("members", person, DNKey, "member")
	// has_many groups: person.DN -> Groups whose member contains it.
	person.HasMany("groups", group, "member", DNKey)
	// belongs_to primaryGroup: person.gidNumber -> Group with matching gidNumber.
	person.BelongsTo("primaryGroup", group, "gidNumber", "gidNumber")
	return
}

func TestAssociationUnknown(t *testing.T) {
	p, _, _ := assocFixture(t)
	alice, _ := p.Find("alice")
	if _, err := alice.Association("bogus"); err == nil {
		t.Fatal("expected unknown association error")
	}
}

func TestHasManyByDN(t *testing.T) {
	_, g, _ := assocFixture(t)
	admins, err := g.Find("admins")
	if err != nil {
		t.Fatal(err)
	}
	members, err := admins.Association("members")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].ID() != "alice" {
		t.Fatalf("members: %#v", members)
	}
}

func TestHasManyByAttribute(t *testing.T) {
	p, _, _ := assocFixture(t)
	alice, _ := p.Find("alice")
	groups, err := alice.Association("groups")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].ID() != "admins" {
		t.Fatalf("groups: %#v", groups)
	}
}

func TestBelongsToOne(t *testing.T) {
	p, _, _ := assocFixture(t)
	alice, _ := p.Find("alice")
	grp, err := alice.AssociationOne("primaryGroup")
	if err != nil {
		t.Fatal(err)
	}
	if grp == nil || grp.ID() != "admins" {
		t.Fatalf("primaryGroup: %#v", grp)
	}
}

func TestAssociationOneNone(t *testing.T) {
	p, g, dir := assocFixture(t)
	// A person with no matching group.
	dir.Seed("uid=bob,ou=Users,dc=example,dc=com", map[string][]string{
		"uid": {"bob"}, "gidNumber": {"999"}, "objectClass": {"top", "person", "inetOrgPerson"},
	})
	bob, _ := p.Find("bob")
	grp, err := bob.AssociationOne("primaryGroup")
	if err != nil || grp != nil {
		t.Fatalf("expected nil group: %#v %v", grp, err)
	}
	_ = g
}

func TestKeyValuesAndEmpty(t *testing.T) {
	p, _, _ := assocFixture(t)
	// New (unsaved) record: DN empty -> association resolves to nothing.
	fresh := p.New()
	recs, err := fresh.Association("groups")
	if err != nil || recs != nil {
		t.Fatalf("empty keyValues: %#v %v", recs, err)
	}
	if fresh.keyValues("gidNumber") != nil {
		t.Fatal("absent attr keyValues nil")
	}
}

func TestAssociationErrorPaths(t *testing.T) {
	p, g, dir := assocFixture(t)
	admins, _ := g.Find("admins")
	alice, _ := p.Find("alice")
	dir.FailOn["search"] = "boom"
	// DNKey path: findByDN issues a search that fails.
	if _, err := admins.Association("members"); err == nil {
		t.Fatal("expected DNKey search error")
	}
	// attribute path: Search fails.
	if _, err := alice.Association("groups"); err == nil {
		t.Fatal("expected attr search error")
	}
	if _, err := alice.AssociationOne("primaryGroup"); err == nil {
		t.Fatal("expected belongs_to search error")
	}
}

func TestResolveByValuesEmpty(t *testing.T) {
	_, g, _ := assocFixture(t)
	got, err := resolveByValues(g, "cn", nil)
	if err != nil || got != nil {
		t.Fatalf("empty values: %#v %v", got, err)
	}
}
