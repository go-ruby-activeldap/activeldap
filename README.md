<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-activeldap/brand/main/social/go-ruby-activeldap-activeldap.png" alt="go-ruby-activeldap/activeldap" width="720"></p>

# activeldap — go-ruby-activeldap

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-activeldap.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the ActiveRecord-style LDAP
object-relational mapper of Ruby's [ActiveLdap](https://github.com/activeldap/activeldap)
gem** — the object↔entry mapping (`ldap_mapping`), the finder/search query
builder, the dirty-tracking record with single- and multi-valued attributes,
validations, `belongs_to`/`has_many` associations over distinguished names, and
LDIF import/export. It mirrors ActiveLdap's semantics **without any Ruby
runtime**.

It is the ORM backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) (rbgo), layered on
the `Net::LDAP` surface provided by
[go-ruby-ldap](https://github.com/go-ruby-ldap/ldap) exactly as the real
`activeldap` gem builds on `net-ldap` — but it is a **standalone, reusable**
module, a sibling of [go-ruby-sequel](https://github.com/go-ruby-sequel/sequel)
and [go-ruby-activerecord](https://github.com/go-ruby-activerecord/activerecord).

> **What it is — and isn't.** Turning a mapping plus a set of conditions into a
> distinguished name and an RFC 4515 search filter, tracking which attributes
> changed, deciding add-vs-modify on save, diffing dirty attributes into modify
> operations, validating, and serialising to LDIF is fully deterministic and
> needs **no directory server and no interpreter**, so it lives here as pure Go.
> Talking to a directory is the host's job: a `Base` is bound to a `Directory`
> seam — the four Net::LDAP operations ActiveLdap uses (`search` / `add` /
> `modify` / `delete`). The host (rbgo) wires that seam to the bound `Net::LDAP`
> connection; tests wire it to the in-memory `MockDirectory`. The mapping, DN,
> filter, dirty-diff and LDIF logic is what this library owns and tests; the
> network is the seam.

## Features

Faithful port of ActiveLdap's ORM, validated against ActiveLdap 7.x semantics
and (differentially) against the `net-ldap` gem's own filter escaping and Ruby's
Base64:

- **`ldap_mapping`** — `Mapping{DNAttribute, Prefix, Classes, Scope}` plus
  attribute **aliases** (case-insensitive) and **single-valued** declarations,
  compiled by `NewClass` and bound to a `Connection`.
- **Finders** — `Find(id)`, `FindFirst` / `FindAll` / `Search(FindOptions{…})`
  with per-call `Filter` / `Base` / `Scope` / `Attributes` / `Limit` overrides,
  and `Exist(id)`. Every find is guarded by the mapping's `objectClass` filter.
- **RFC 4515 filters** — a composable builder (`Equal`, `Present`, `Substring`,
  `And` / `Or` / `Not`, `RawFilter`) with metacharacter escaping, a `Conditions`
  hash-to-AND helper, plus a parser/evaluator so `MockDirectory` answers the very
  filters the builder emits.
- **DN handling** — `ParseDN` / `BuildDN` / `Normalized` / `Equal` / `Parent`
  with RFC 4514 value escaping.
- **Record** — case-insensitive attribute access (`Get` / `One` / `Set` / `Add`
  / `Delete`), **dirty tracking** (`Changed` / `ChangedAttributes` / `Changes`),
  `New`/`Persisted`, computed `DN` and `ID`.
- **Persistence** — `Create`, `Save` (INSERT for a new record, minimal
  diff-based `:replace`/`:delete` UPDATE for an existing one),
  `UpdateAttributes`, `Destroy`, `Reload`.
- **Validations** — `PresenceOf`, `RequiredClasses`, custom `Validate`
  validators, an `Errors` object with `FullMessages`, and a `ValidationError`
  that blocks a save without touching the directory.
- **Associations** — `BelongsTo` / `HasMany` over an attribute foreign key *or*
  the entry DN (`DNKey`), the classic `groupOfNames` `member` pattern included.
- **LDIF** — `ToLDIF` (RFC 2849, safe-string base64), `ParseLDIF` (line folding,
  comments, base64), and `LoadLDIF` import through the connection.

## Install

```sh
go get github.com/go-ruby-activeldap/activeldap
```

## Quick start (Go)

```go
dir := activeldap.NewMockDirectory() // or a Net::LDAP-backed Directory
conn := activeldap.NewConnection(dir, "dc=example,dc=com")

person := activeldap.NewClass("Person", &activeldap.Mapping{
    DNAttribute: "uid",
    Prefix:      "ou=Users",
    Classes:     []string{"top", "person", "inetOrgPerson"},
    Scope:       activeldap.ScopeSub,
}, conn)

alice, _ := person.Create(map[string][]string{
    "uid": {"alice"}, "cn": {"Alice"}, "sn": {"Adams"},
})
alice.Set("mail", "alice@example.com")
alice.Save() // diff-based modify: only mail is replaced

people, _ := person.FindAll(activeldap.FindOptions{Filter: activeldap.Present("mail")})
```

The equivalent Ruby (running on rbgo, once the gem is registered) is in
[`examples/activeldap_usage.rb`](examples/activeldap_usage.rb).

## Tests & coverage

```sh
GOWORK=off go test -race -coverpkg=$(go list ./... | paste -sd, -) -coverprofile=cover.out ./...
GOWORK=off go tool cover -func=cover.out | tail -1   # total: 100.0%
```

100% statement coverage including every error branch. CI additionally builds on
the six supported 64-bit targets (amd64/arm64 native, riscv64/loong64/ppc64le/s390x
under qemu) and for `js`/`wasip1` wasm. The differential oracle (`oracle_test.go`)
checks filter escaping and LDIF base64 against the real `net-ldap` gem and Ruby's
Base64, and skips where they are absent.

## License

BSD-3-Clause © the go-ruby-activeldap/activeldap authors.
