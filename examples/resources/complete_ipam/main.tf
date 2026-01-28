terraform {
  required_providers {
    netbox = {
      source = "local/justinr/netbox"
    }
  }
}

provider "netbox" {
  url      = "https://demo.netbox.dev"
  token    = "eBmNSu3cFxCZnY9qwoX8xi93BJniL6qjvbE5P70j"
  insecure = false
}

# Organization tenant
resource "netbox_tenant" "example" {
  name        = "Example Corp"
  slug        = "example-corp"
  description = "Example organization for testing"
  upsert      = true
}

# Regional Internet Registry
resource "netbox_rir" "rfc1918" {
  name        = "RFC1918"
  slug        = "rfc1918"
  is_private  = true
  description = "Private address space"
  upsert      = true
}

# ASN Range for allocation
resource "netbox_asn_range" "private_asns" {
  name        = "Private ASN Range"
  slug        = "private-asn-range"
  start       = 64512
  end         = 65534
  rir         = netbox_rir.rfc1918.id
  tenant      = netbox_tenant.example.id
  description = "Private ASN range for internal use"
}

# Individual ASN
resource "netbox_asn" "example" {
  asn         = 65001
  rir         = netbox_rir.rfc1918.id
  tenant      = netbox_tenant.example.id
  description = "Example ASN for testing"
  upsert      = true
}

# VRF
resource "netbox_vrf" "example" {
  name        = "EXAMPLE-VRF"
  rd          = "65001:100"
  tenant      = netbox_tenant.example.id
  description = "Example VRF for testing"
  upsert      = true
}

# Route Targets for VRF
resource "netbox_route_target" "import" {
  name        = "65001:100"
  tenant      = netbox_tenant.example.id
  description = "Import RT for example VRF"
}

resource "netbox_route_target" "export" {
  name        = "65001:101"
  tenant      = netbox_tenant.example.id
  description = "Export RT for example VRF"
}

# VLAN Group
resource "netbox_vlan_group" "prod" {
  name        = "Production"
  slug        = "production"
  description = "Production VLANs"
  min_vid     = 100
  max_vid     = 199
}

# VLAN
resource "netbox_vlan" "web" {
  vid         = 101
  name        = "Web-Servers"
  status      = "active"
  group       = netbox_vlan_group.prod.id
  tenant      = netbox_tenant.example.id
  description = "Web server VLAN"
  upsert      = true
}

# IP Prefix (allocation pool)
resource "netbox_prefix" "web_pool" {
  prefix        = "10.0.101.0/24"
  status        = "active"
  is_pool       = true
  mark_utilized = false
  vrf           = netbox_vrf.example.id
  tenant        = netbox_tenant.example.id
  description   = "Web server subnet pool"
  upsert        = true
}

# IP Range for allocation
resource "netbox_ip_range" "web_servers" {
  start_address = "10.0.101.10"
  end_address   = "10.0.101.50"
  status        = "active"
  vrf           = netbox_vrf.example.id
  tenant        = netbox_tenant.example.id
  description   = "Web server IP range"
}

# Individual IP Address
resource "netbox_ip_address" "web01" {
  address     = "10.0.101.10/24"
  status      = "active"
  role        = "anycast"
  dns_name    = "web01.example.com"
  vrf         = netbox_vrf.example.id
  tenant      = netbox_tenant.example.id
  description = "Web server 01"
}

# Outputs to show allocated resources
output "tenant_id" {
  value = netbox_tenant.example.id
}

output "vrf_id" {
  value = netbox_vrf.example.id
}

output "prefix_id" {
  value = netbox_prefix.web_pool.id
}

output "ip_address_id" {
  value = netbox_ip_address.web01.id
}
