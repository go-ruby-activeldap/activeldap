// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

import (
	"reflect"
	"testing"
)

func TestAttributeSetBasics(t *testing.T) {
	a := newAttributeSet(map[string]string{"commonname": "cn"})
	a.Set("CN", []string{"Alice"})
	if !a.Has("cn") || !a.Has("commonName") {
		t.Fatal("case/alias Has")
	}
	if got := a.Get("commonName"); !reflect.DeepEqual(got, []string{"Alice"}) {
		t.Fatalf("alias Get: %#v", got)
	}
	// canonical spelling preserved from first Set.
	if names := a.Names(); len(names) != 1 || names[0] != "CN" {
		t.Fatalf("names: %#v", names)
	}
	// overwrite keeps single entry.
	a.Set("cn", []string{"Bob"})
	if len(a.Names()) != 1 {
		t.Fatal("overwrite added entry")
	}
	if a.Get("missing") != nil {
		t.Fatal("absent Get nil")
	}
}

func TestAttributeSetDelete(t *testing.T) {
	a := newAttributeSet(nil)
	a.Set("a", []string{"1"})
	a.Set("b", []string{"2"})
	a.Set("c", []string{"3"})
	a.Delete("b")
	if a.Has("b") {
		t.Fatal("b still present")
	}
	if got := a.Names(); !reflect.DeepEqual(got, []string{"a", "c"}) {
		t.Fatalf("order after delete: %#v", got)
	}
	a.Delete("missing") // no-op branch
}

func TestAttributeSetCloneAndSorted(t *testing.T) {
	a := newAttributeSet(nil)
	a.Set("b", []string{"2"})
	a.Set("a", []string{"1"})
	c := a.clone()
	c.Set("a", []string{"changed"})
	if reflect.DeepEqual(a.Get("a"), c.Get("a")) {
		t.Fatal("clone not deep")
	}
	if got := a.sortedFolded(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("sortedFolded: %#v", got)
	}
}
