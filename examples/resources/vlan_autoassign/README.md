# VLAN Auto-Assignment Example

This example demonstrates the `autoassign` feature of the `netbox_vlan` resource, which automatically assigns an available VLAN ID from a VLAN group.

## How Auto-Assignment Works

When `autoassign = true`:

1. **Group Required**: The `group` attribute must be specified to indicate which VLAN group to allocate from.

2. **VID Auto-Computed**: The `vid` attribute becomes computed and should not be manually set. NetBox will automatically select an available VID from the group's range.

3. **Duplicate Prevention**: Before allocating a new VID, the provider checks if a VLAN with the specified `name` already exists in the group:
   - If found, it updates the existing VLAN with the desired configuration (idempotent behavior)
   - If not found, it calls `/api/ipam/vlan-groups/{id}/available-vlans/` to allocate a new VID

4. **API Endpoint**: Uses NetBox's `/api/ipam/vlan-groups/{id}/available-vlans/` endpoint which returns the next available VID within the group's min/max range.

## Usage

```hcl
# Create a VLAN group with a defined VID range
resource "netbox_vlan_group" "auto_vlans" {
  name    = "Auto-Assigned VLANs"
  min_vid = 100
  max_vid = 199
}

# Let NetBox automatically assign an available VID
resource "netbox_vlan" "auto_web" {
  name        = "Web-Servers-Auto"
  autoassign  = true              # Enable auto-assignment
  group       = netbox_vlan_group.auto_vlans.id
  status      = "active"
  description = "Auto-assigned VLAN"
}

# The VID will be computed and available in state
output "assigned_vid" {
  value = netbox_vlan.auto_web.vid
}
```

## Benefits

- **Prevents Conflicts**: No need to manually track which VIDs are already in use
- **Idempotent**: Running `terraform apply` multiple times won't create duplicate VLANs
- **Scalable**: Easier to manage large VLAN deployments
- **Consistent**: Ensures VLANs stay within the group's defined range

## Comparison: Auto-Assign vs Explicit VID

### Auto-Assign
```hcl
resource "netbox_vlan" "auto" {
  name       = "My-VLAN"
  autoassign = true
  group      = netbox_vlan_group.example.id
  # vid is computed automatically
}
```

### Explicit VID
```hcl
resource "netbox_vlan" "explicit" {
  vid   = 200
  name  = "My-VLAN"
  # autoassign should not be set
}
```

## Important Notes

- **Mutual Exclusivity**: When `autoassign = true`, do not set the `vid` attribute (it will be computed)
- **Group Required**: The `group` attribute is mandatory when using `autoassign`
- **Name-Based Deduplication**: The provider checks for existing VLANs by name within the group to prevent duplicates
- **Re-runs Safe**: Running Terraform multiple times with the same configuration will not allocate additional VIDs

## Testing

```bash
# Initialize Terraform
terraform init

# See the execution plan (vid will show as "known after apply")
terraform plan

# Apply the configuration
terraform apply

# Check the assigned VIDs
terraform output
```
