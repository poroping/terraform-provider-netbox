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

# Create a VLAN group
resource "netbox_vlan_group" "auto_vlans" {
  name        = "Auto-Assigned VLANs"
  slug        = "auto-assigned-vlans"
  min_vid     = 100
  max_vid     = 199
  description = "VLANs with auto-assigned VIDs"
  upsert      = true
}

# Auto-assign VLAN from the group
# The VID will be automatically selected from available VIDs in the group
resource "netbox_vlan" "auto_web" {
  name        = "Web-Servers-Auto"
  autoassign  = true
  group       = netbox_vlan_group.auto_vlans.id
  status      = "active"
  description = "Auto-assigned VLAN for web servers"
}

# Another auto-assigned VLAN in the same group
resource "netbox_vlan" "auto_db" {
  name        = "DB-Servers-Auto"
  autoassign  = true
  group       = netbox_vlan_group.auto_vlans.id
  status      = "active"
  description = "Auto-assigned VLAN for database servers"
}

# Standard VLAN with explicit VID (for comparison)
resource "netbox_vlan" "explicit" {
  vid         = 200
  name        = "Explicit-VLAN"
  status      = "active"
  description = "VLAN with explicitly specified VID"
}

# Output the auto-assigned VIDs
output "auto_web_vid" {
  value       = netbox_vlan.auto_web.vid
  description = "Auto-assigned VID for web servers VLAN"
}

output "auto_db_vid" {
  value       = netbox_vlan.auto_db.vid
  description = "Auto-assigned VID for database servers VLAN"
}

output "explicit_vid" {
  value       = netbox_vlan.explicit.vid
  description = "Explicitly specified VID"
}
