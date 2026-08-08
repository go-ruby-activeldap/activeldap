# Copyright (c) the go-ruby-activeldap/activeldap authors
#
# SPDX-License-Identifier: BSD-3-Clause
#
# ActiveLdap usage on rbgo. This is the Ruby surface the go-embedded-ruby binding
# exposes over this library, layered on the Net::LDAP connection from
# go-ruby-ldap — the same API as the upstream `activeldap` gem. Run it with:
#
#   rbgo examples/activeldap_usage.rb
#
# (once the `active_ldap` gem is registered in rbgo; see the go-embedded-ruby PR).

require "active_ldap"

# Connection — the Go form wires Net::LDAP under the hood.
ActiveLdap::Base.setup_connection(
  host:     "ldap.example.com",
  port:     389,
  base:     "dc=example,dc=com",
  bind_dn:  "cn=admin,dc=example,dc=com",
  password: "secret"
)

class User < ActiveLdap::Base
  ldap_mapping dn_attribute: "uid",
               prefix:       "ou=Users",
               classes:      ["top", "person", "inetOrgPerson"],
               scope:        :sub
  validates_presence_of :cn, :sn
  belongs_to :primary_group, class: "Group",
             foreign_key: "gidNumber", primary_key: "gidNumber"
end

class Group < ActiveLdap::Base
  ldap_mapping dn_attribute: "cn", prefix: "ou=Groups", classes: ["groupOfNames"]
  has_many :members, class: "User", foreign_key: "dn", primary_key: "member"
end

# Create + save (INSERT).
alice = User.new("alice")
alice.cn = "Alice"
alice.sn = "Adams"
alice.mail = ["alice@example.com"]
alice.save

# Find + update (diff-based modify).
u = User.find("alice")
u.mail << "alice.adams@example.com"
u.update_attributes(title: "Engineer")

# Search with a filter.
engineers = User.find(:all, filter: "(title=Engineer)")
puts engineers.map(&:dn)

# Associations.
admins = Group.find("admins")
admins.members.each { |m| puts m.uid }

# LDIF export.
puts alice.to_ldif

# Validation.
bad = User.new("bob") # missing cn/sn
puts bad.valid?            # => false
puts bad.errors.full_messages
