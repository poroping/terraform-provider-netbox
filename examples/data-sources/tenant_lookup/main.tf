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

# Create a tenant
resource "netbox_tenant" "test" {
  name        = "Test Tenant"
  slug        = "test-tenant"
  description = "Test tenant for data source"
}

# Look up the tenant by name
data "netbox_tenant" "lookup" {
  name = netbox_tenant.test.name
}

# Output the looked-up tenant ID
output "tenant_id_from_data_source" {
  value = data.netbox_tenant.lookup.id
}

output "tenant_id_from_resource" {
  value = netbox_tenant.test.id
}
