# Basic tenant - slug is computed automatically by NetBox from the name
resource "netbox_tenant" "basic" {
  name = "Acme Corp"
}

# Full tenant with all optional attributes
resource "netbox_tenant" "full" {
  name        = "Globex Corporation"
  description = "Primary customer tenant for Globex"
  comments    = "Onboarded 2026-01-01. Primary contact: ops@globex.example.com"

  tags = [
    {
      name = "customer"
      slug = "customer"
    },
    {
      name = "production"
      slug = "production"
    }
  ]
}

# Upsert: adopt a tenant that was created outside of Terraform.
# The provider searches by name; if a match is found it updates the record
# and brings it into Terraform state without recreating it.
resource "netbox_tenant" "legacy" {
  name        = "Legacy-Org"
  upsert      = true
  description = "Pre-existing tenant adopted by Terraform"
}

# Upsert by slug: adopt a tenant whose slug matches the one NetBox would
# derive from the given name. Useful when the tenant was created with a
# different display name but you know its slug.
# Takes precedence over upsert (name-based) when both are set.
resource "netbox_tenant" "by_slug" {
  name           = "Initech"
  upsert_by_slug = true
  description    = "Adopted by slug match"
}

# Reference the computed slug in other resources
output "acme_slug" {
  value = netbox_tenant.basic.slug
}
