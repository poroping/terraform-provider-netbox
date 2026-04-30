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
