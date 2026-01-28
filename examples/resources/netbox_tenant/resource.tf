# Create a new tenant
resource "netbox_tenant" "example" {
  name        = "example-tenant"
  slug        = "example-tenant"
  description = "Example tenant for demonstration"
  comments    = "Created by Terraform"

  tags = [
    {
      name = "terraform"
      slug = "terraform"
    }
  ]
}

# Use existing tenant with upsert
resource "netbox_tenant" "existing" {
  name   = "existing-tenant"
  slug   = "existing-tenant"
  upsert = true
}
