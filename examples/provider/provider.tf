terraform {
  required_providers {
    netbox = {
      source = "local/justinr/netbox"
    }
  }
}

provider "netbox" {
  url   = "https://demo.netbox.dev"
  token = "eBmNSu3cFxCZnY9qwoX8xi93BJniL6qjvbE5P70j"
}

# Create a tenant
resource "netbox_tenant" "example" {
  name        = "tf-example-tenant"
  slug        = "tf-example"
  description = "Example tenant created by Terraform"
}

# Create a RIR
resource "netbox_rir" "example" {
  name        = "tf-example-rir"
  description = "Example RIR for testing"
  is_private  = true
}

# Create an ASN range
resource "netbox_asn_range" "example" {
  name        = "tf-example-asn-range"
  start       = 64512
  end         = 64522
  rir         = netbox_rir.example.id
  description = "Private ASN range for testing"
}

# Create a VRF
resource "netbox_vrf" "example" {
  name        = "tf-example-vrf"
  rd          = "64512:100"
  description = "Example VRF"
  tenant      = netbox_tenant.example.id
}

# Create a VLAN group
resource "netbox_vlan_group" "example" {
  name        = "tf-example-vlan-group"
  description = "Example VLAN group"
  min_vid     = 100
  max_vid     = 199
}
