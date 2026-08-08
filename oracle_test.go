// Copyright (c) the go-ruby-activeldap/activeldap authors
//
// SPDX-License-Identifier: BSD-3-Clause

package activeldap

// Differential oracle tests that check this library's parity-sensitive string
// output against the *actual* Ruby gems ActiveLdap builds on — Net::LDAP's
// filter-value escaping and Ruby's Base64 for LDIF. They need the net-ldap gem
// installed, so they skip-gate on it being requireable (it is not on the stock
// CI lanes; run them locally with the gem on GEM_PATH). The deterministic,
// ruby-free tests already cover these surfaces to 100%, so the no-ruby lanes
// still pass; this is the strongest form of the parity claim.

import (
	"os/exec"
	"strings"
	"testing"
)

// runRuby runs a Ruby script, skipping the test unless ruby (and, when probe is
// non-empty, the probed require) is available.
func runRuby(t *testing.T, probe, script string) string {
	t.Helper()
	if _, err := exec.LookPath("ruby"); err != nil {
		t.Skip("ruby not available; skipping differential oracle")
	}
	if probe != "" {
		if out, err := exec.Command("ruby", "-e", probe).CombinedOutput(); err != nil {
			t.Skipf("ruby probe %q failed; skipping oracle\n%s", probe, out)
		}
	}
	out, err := exec.Command("ruby", "-e", script).CombinedOutput()
	if err != nil {
		t.Fatalf("ruby oracle failed: %v\n%s", err, out)
	}
	// Ruby's puts emits CRLF on Windows; normalise so line splits stay clean.
	return strings.TrimSpace(strings.ReplaceAll(string(out), "\r\n", "\n"))
}

// TestOracleFilterEscape proves EscapeFilterValue matches Net::LDAP::Filter's own
// value escaping for the RFC 4515 metacharacters.
func TestOracleFilterEscape(t *testing.T) {
	const script = `require 'net/ldap'
["*", "(", ")", "\\", "a*b(c)d\\e", "plain"].each do |v|
  puts Net::LDAP::Filter.escape(v)
end`
	got := runRuby(t, "require 'net/ldap'", script)
	wantInputs := []string{"*", "(", ")", "\\", "a*b(c)d\\e", "plain"}
	lines := strings.Split(got, "\n")
	for i, in := range wantInputs {
		if EscapeFilterValue(in) != strings.ToLower(lines[i]) && EscapeFilterValue(in) != lines[i] {
			t.Errorf("escape %q: ours=%q ruby=%q", in, EscapeFilterValue(in), lines[i])
		}
	}
}

// TestOracleLDIFBase64 proves our LDIF base64 encoding matches Ruby's Base64
// strict encoding, so a value ActiveLdap would base64 in to_ldif round-trips.
func TestOracleLDIFBase64(t *testing.T) {
	const script = `require 'base64'
puts Base64.strict_encode64("café")
puts Base64.strict_encode64(" leading")`
	got := runRuby(t, "require 'base64'", script)
	lines := strings.Split(got, "\n")
	if ldifLine("cn", "café") != "cn:: "+lines[0] {
		t.Errorf("café: ours=%q ruby=%q", ldifLine("cn", "café"), "cn:: "+lines[0])
	}
	if ldifLine("cn", " leading") != "cn:: "+lines[1] {
		t.Errorf("leading: ours=%q ruby=%q", ldifLine("cn", " leading"), "cn:: "+lines[1])
	}
}
