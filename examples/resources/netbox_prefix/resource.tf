# Create a prefix manually
resource "netbox_prefix" "example" {
  prefix      = "10.0.0.0/24"
  status      = "active"
  vrf         = netbox_vrf.example.id
  tenant      = netbox_tenant.example.id
  description = "Example network prefix"

  tags = [
    {
      name = "production"
      slug = "production"
    }
  ]
}

# Create a parent prefix
resource "netbox_prefix" "parent" {
  prefix      = "172.16.0.0/16"
  status      = "container"
  description = "Parent prefix for auto-allocation"
}

# Auto-assign prefix from parent
resource "netbox_prefix" "auto_allocated" {
  autoassign       = true
  parent_prefix_id = netbox_prefix.parent.id
  prefix_length    = 24
  status           = "active"
  tenant           = netbox_tenant.example.id
  description      = "Auto-allocated from parent"

  tags = [
    {
      name = "auto-assigned"
      slug = "auto-assigned"
    }
  ]
}

# Auto-assign with upsert: reuse existing prefix with same tenant/tags if found
resource "netbox_prefix" "auto_with_upsert" {
  autoassign       = true
  upsert           = true
  parent_prefix_id = netbox_prefix.parent.id
  prefix_length    = 24
  status           = "active"
  tenant           = netbox_tenant.example.id
  description      = "Will reuse existing prefix if found"

  tags = [
    {
      name = "reusable"
      slug = "reusable"
    }
  ]
}

