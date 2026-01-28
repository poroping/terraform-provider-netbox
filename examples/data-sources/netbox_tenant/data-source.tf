# Look up tenant by name
data "netbox_tenant" "example" {
  name = "example-tenant"
}

# Look up tenant by slug
data "netbox_tenant" "by_slug" {
  slug = "example-tenant"
}

# Use data source output
resource "netbox_vrf" "example" {
  name   = "example-vrf"
  rd     = "65000:100"
  tenant = data.netbox_tenant.example.id
}
