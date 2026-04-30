# Basic IP address - just an address with prefix length
resource "netbox_ip_address" "basic" {
  address = "10.0.0.1/24"
}

# Full IP address with all optional attributes
resource "netbox_ip_address" "full" {
  address     = "10.0.0.2/24"
  vrf         = netbox_vrf.mgmt.id
  tenant      = netbox_tenant.acme.id
  dns_name    = "web01.example.com"
  description = "Primary interface - web01"
  comments    = "Allocated to web cluster on 2026-01-15"

  tags = [
    {
      name = "production"
      slug = "production"
    },
    {
      name = "web"
      slug = "web"
    }
  ]
}

# Upsert: adopt an IP address that already exists in NetBox.
# The provider searches by address; if found it updates the record
# to match desired state rather than failing with a duplicate error.
resource "netbox_ip_address" "loopback" {
  address     = "192.0.2.10/32"
  upsert      = true
  dns_name    = "lo0.router1.example.com"
  description = "Router loopback - adopted from existing record"
}

# VRF-scoped address with upsert.
# Useful when the same RFC1918 address appears in multiple VRFs and you
# want Terraform to own a specific one without recreating it.
resource "netbox_ip_address" "vrf_addr" {
  address = "172.16.0.1/24"
  vrf     = netbox_vrf.customer_a.id
  tenant  = netbox_tenant.customer_a.id
  upsert  = true
}

# ── Autoassign ──────────────────────────────────────────────────────────────
# The prefix to allocate from. It must already exist in NetBox.
resource "netbox_prefix" "server_pool" {
  prefix      = "10.10.0.0/24"
  description = "Server address pool"
}

# Autoassign: the provider calls POST /api/ipam/prefixes/{id}/available-ips/
# and NetBox returns the first unallocated IP in the prefix.
# The 'address' attribute is computed and available in state after apply.
resource "netbox_ip_address" "auto_web" {
  autoassign       = true
  parent_prefix_id = netbox_prefix.server_pool.id
  dns_name         = "web02.example.com"
  description      = "Auto-allocated address for web02"
  tenant           = netbox_tenant.acme.id
}

resource "netbox_ip_address" "auto_db" {
  autoassign       = true
  parent_prefix_id = netbox_prefix.server_pool.id
  dns_name         = "db01.example.com"
  description      = "Auto-allocated address for db01"
  tenant           = netbox_tenant.acme.id
}

# Expose computed addresses for use by other resources
output "web02_ip" {
  value = netbox_ip_address.auto_web.address
}

output "db01_ip" {
  value = netbox_ip_address.auto_db.address
}

# Autoassign + upsert: idempotent allocation.
# Primary match key: dns_name within the parent prefix.
# On first apply: a new IP is allocated from the prefix.
# On re-apply (e.g. after accidental destroy): the provider finds the existing
# record by dns_name within the prefix and re-uses it instead of allocating a
# second address.
resource "netbox_ip_address" "auto_upsert" {
  autoassign       = true
  upsert           = true
  parent_prefix_id = netbox_prefix.server_pool.id
  dns_name         = "mgmt01.example.com"
  description      = "Management interface - idempotently allocated"
  tenant           = netbox_tenant.acme.id
}
