# Basic prefix with an explicit CIDR
resource "netbox_prefix" "basic" {
  prefix = "10.0.0.0/24"
}

# Full prefix with all optional attributes
resource "netbox_prefix" "full" {
  prefix      = "10.1.0.0/24"
  vrf         = netbox_vrf.mgmt.id
  tenant      = netbox_tenant.acme.id
  description = "Management network"
  comments    = "Allocated for the management plane"

  tags = [
    {
      name = "production"
      slug = "production"
    },
    {
      name = "management"
      slug = "management"
    }
  ]
}

# Upsert: adopt an existing prefix instead of failing on a duplicate CIDR.
# The provider searches by exact prefix string; if found it updates the
# record to match desired state and takes it into Terraform state.
resource "netbox_prefix" "legacy_dmz" {
  prefix      = "192.168.100.0/24"
  upsert      = true
  description = "Legacy DMZ - adopted from pre-existing NetBox record"
}

# ── Autoassign ──────────────────────────────────────────────────────────────
# A supernet ("container") that child prefixes are carved from.
# This should already exist in NetBox or be created once and left stable.
resource "netbox_prefix" "supernet" {
  prefix      = "172.16.0.0/16"
  description = "Supernet - container for auto-allocated /24s"
}

# Auto-assign: the provider calls /api/ipam/prefixes/{id}/available-prefixes/
# and allocates the first free prefix of the requested length.
resource "netbox_prefix" "auto_app" {
  autoassign       = true
  parent_prefix_id = netbox_prefix.supernet.id
  prefix_length    = 24
  tenant           = netbox_tenant.acme.id
  description      = "Auto-allocated for the application tier"

  tags = [
    {
      name = "app"
      slug = "app"
    }
  ]
}

resource "netbox_prefix" "auto_db" {
  autoassign       = true
  parent_prefix_id = netbox_prefix.supernet.id
  prefix_length    = 24
  tenant           = netbox_tenant.acme.id
  description      = "Auto-allocated for the database tier"

  tags = [
    {
      name = "db"
      slug = "db"
    }
  ]
}

# Expose the computed prefixes for use by other resources
output "app_prefix" {
  value = netbox_prefix.auto_app.prefix
}

output "db_prefix" {
  value = netbox_prefix.auto_db.prefix
}

# Combined autoassign + upsert: idempotent allocation.
# On first apply a new /24 is carved from the supernet.
# On re-apply (or after accidental destroy) the provider finds the child
# prefix that already matches the tenant + tags combination and re-uses it
# instead of allocating a second block.
resource "netbox_prefix" "auto_storage" {
  autoassign       = true
  upsert           = true
  parent_prefix_id = netbox_prefix.supernet.id
  prefix_length    = 24
  tenant           = netbox_tenant.acme.id
  description      = "Storage network - idempotently allocated"

  tags = [
    {
      name = "storage"
      slug = "storage"
    }
  ]
}