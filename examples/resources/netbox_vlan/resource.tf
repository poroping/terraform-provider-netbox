# Basic VLAN with an explicit VID
resource "netbox_vlan" "basic" {
  vid    = 100
  name   = "Management"
  status = "active"
}

# VLAN with all optional attributes
resource "netbox_vlan" "full" {
  vid         = 200
  name        = "Web-Servers"
  status      = "active"
  group       = netbox_vlan_group.datacenter.id
  tenant      = netbox_tenant.acme.id
  description = "VLAN for public-facing web servers"
  comments    = "Reviewed and approved by network team"

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

# Auto-assign: let NetBox pick the next available VID from the group's range.
# The group must have min_vid/max_vid set. The provider calls
# /api/ipam/vlan-groups/{id}/available-vlans/ and uses the first free VID.
resource "netbox_vlan_group" "app_vlans" {
  name    = "App VLANs"
  slug    = "app-vlans"
  min_vid = 300
  max_vid = 399
}

resource "netbox_vlan" "auto_app" {
  name        = "App-Tier-1"
  autoassign  = true
  group       = netbox_vlan_group.app_vlans.id
  status      = "active"
  description = "Auto-assigned VLAN for the application tier"
}

resource "netbox_vlan" "auto_db" {
  name        = "DB-Tier-1"
  autoassign  = true
  group       = netbox_vlan_group.app_vlans.id
  status      = "active"
  description = "Auto-assigned VLAN for the database tier"
}

# Export the computed VIDs so they can be consumed by other resources
output "app_vlan_vid" {
  value = netbox_vlan.auto_app.vid
}

output "db_vlan_vid" {
  value = netbox_vlan.auto_db.vid
}

# Upsert: adopt an existing VLAN that was created outside of Terraform.
# If a VLAN named "Legacy-DMZ" already exists, Terraform will take ownership
# of it instead of failing with a duplicate-name error.
resource "netbox_vlan" "legacy_dmz" {
  vid    = 999
  name   = "Legacy-DMZ"
  upsert = true
  status = "active"
}

# Combined autoassign + upsert: idempotent VLAN allocation.
# On the first apply a new VID is chosen from the group.
# On subsequent applies (or after an accidental destroy) the provider
# finds the existing VLAN by name inside the group and re-uses it,
# rather than allocating a second VID.
resource "netbox_vlan" "idempotent_storage" {
  name       = "Storage-Network"
  autoassign = true
  upsert     = true
  group      = netbox_vlan_group.app_vlans.id
  status     = "active"
  description = "Storage VLAN - idempotently allocated"
}
