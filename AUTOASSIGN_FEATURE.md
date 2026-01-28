# Prefix Auto-Assignment Feature

## Overview
Added `autoassign` functionality to the `netbox_prefix` resource, enabling automatic allocation of prefixes from a parent prefix using the NetBox API endpoint `/api/ipam/prefixes/{id}/available-prefixes/`.

## New Attributes

### `autoassign` (Boolean, Optional)
When set to `true`, automatically allocates a prefix from the specified parent prefix instead of requiring a manually specified CIDR.

**Requirements:**
- `parent_prefix_id` must be specified
- `prefix_length` must be specified
- `prefix` attribute becomes optional (computed)

### `parent_prefix_id` (Number, Optional)
The ID of the parent prefix from which to allocate a child prefix. Required when `autoassign = true`.

### `prefix_length` (Number, Optional)
The desired prefix length for the auto-allocated prefix (e.g., 24 for a /24). Required when `autoassign = true`.

### `prefix` (String, Optional/Computed)
- **Normal mode**: Required - specify the exact CIDR (e.g., "10.0.0.0/24")
- **Autoassign mode**: Optional/Computed - automatically set by NetBox during allocation

## Usage Modes

### Mode 1: Manual Prefix Creation (Original)
```hcl
resource "netbox_prefix" "manual" {
  prefix      = "10.0.0.0/24"
  status      = "active"
  description = "Manually specified prefix"
}
```

### Mode 2: Auto-Assignment
Automatically allocates an available prefix from a parent:

```hcl
resource "netbox_prefix" "parent" {
  prefix = "172.16.0.0/16"
  status = "container"
}

resource "netbox_prefix" "child" {
  autoassign       = true
  parent_prefix_id = netbox_prefix.parent.id
  prefix_length    = 24
  status           = "active"
  description      = "Auto-allocated /24"
}
```

### Mode 3: Auto-Assignment with Upsert
Combines `autoassign` with `upsert` to reuse existing prefixes:

```hcl
resource "netbox_prefix" "reusable" {
  autoassign       = true
  upsert     = true  # Enable upsert behavior
  parent_prefix_id = netbox_prefix.parent.id
  prefix_length    = 24
  tenant           = netbox_tenant.example.id
  
  tags = [
    { name = "production", slug = "production" }
  ]
}
```

**Upsert Logic:**
1. Search for existing prefixes under the parent
2. Match by `tenant` ID (or both null)
3. Match by `tags` (exact set match, order-independent)
4. If match found, update the existing prefix with new configuration
5. If no match found, allocate a new prefix from parent

## Implementation Details

### API Endpoint
Uses NetBox's prefix allocation endpoint:
```
POST /api/ipam/prefixes/{parent_id}/available-prefixes/
```

Request body includes:
- `prefix_length`: Required prefix length
- `status`, `vrf`, `tenant`, `description`, `comments`: Optional attributes
- `tags`: Optional tag associations

Response: Array of allocated prefix objects (typically single element)

### Validation
- If `autoassign = true`:
  - `parent_prefix_id` must be provided
  - `prefix_length` must be provided
  - `prefix` is optional (will be computed)
  
- If `autoassign = false` or omitted:
  - `prefix` is required
  - Normal creation/upsert behavior

### Tag Matching for Upsert
When `autoassign = true` and `upsert = true`:
- Compares tags by slug (case-sensitive)
- Requires exact set match (all tags must match, no extras)
- Order-independent comparison (uses map for efficiency)

## Benefits

1. **Dynamic Allocation**: NetBox selects the next available prefix automatically
2. **Reproducible Infrastructure**: Terraform manages lifecycle, NetBox handles allocation
3. **Idempotent Operations**: With `upsert`, multiple applies won't create duplicates
4. **Parent/Child Relationships**: Maintains NetBox's hierarchical prefix structure
5. **Tag-Based Selection**: Upsert can find prefixes by semantic meaning (tags) not just IDs

## Use Cases

### Use Case 1: Per-Environment VPCs
```hcl
locals {
  environments = ["dev", "staging", "prod"]
}

resource "netbox_prefix" "vpc_parent" {
  prefix = "10.0.0.0/8"
  status = "container"
}

resource "netbox_prefix" "vpc" {
  for_each = toset(local.environments)
  
  autoassign       = true
  parent_prefix_id = netbox_prefix.vpc_parent.id
  prefix_length    = 16
  description      = "${each.key} VPC"
  
  tags = [
    { name = each.key, slug = each.key }
  ]
}
```

### Use Case 2: Reusable Subnet Pools
```hcl
# First apply: creates new prefix
# Subsequent applies: reuses same prefix if tenant/tags match
resource "netbox_prefix" "app_subnet" {
  autoassign       = true
  upsert     = true
  parent_prefix_id = var.parent_prefix_id
  prefix_length    = 24
  tenant           = var.tenant_id
  
  tags = [
    { name = "application", slug = "application" },
    { name = var.app_name, slug = var.app_name }
  ]
}
```

### Use Case 3: Multi-Region Allocation
```hcl
variable "regions" {
  default = ["us-east", "us-west", "eu-west"]
}

resource "netbox_prefix" "region" {
  for_each = toset(var.regions)
  
  autoassign       = true
  parent_prefix_id = netbox_prefix.global.id
  prefix_length    = 12
  description      = "${each.key} region prefix"
  
  tags = [
    { name = "region", slug = "region" },
    { name = each.key, slug = each.key }
  ]
}

resource "netbox_prefix" "az" {
  for_each = {
    for pair in setproduct(var.regions, ["a", "b", "c"]) :
    "${pair[0]}-${pair[1]}" => { region = pair[0], az = pair[1] }
  }
  
  autoassign       = true
  parent_prefix_id = netbox_prefix.region[each.value.region].id
  prefix_length    = 16
  description      = "${each.value.region}-${each.value.az} AZ"
}
```

## Testing

Build and install:
```bash
make build
make install
```

Test configuration:
```hcl
terraform {
  required_providers {
    netbox = {
      source = "local/justinr/netbox"
    }
  }
}

provider "netbox" {
  url   = "https://netbox.example.com"
  token = var.netbox_token
}

# Create parent container
resource "netbox_prefix" "test_parent" {
  prefix = "192.168.0.0/16"
  status = "container"
}

# Auto-allocate child
resource "netbox_prefix" "test_child" {
  autoassign       = true
  parent_prefix_id = netbox_prefix.test_parent.id
  prefix_length    = 24
  status           = "active"
}

output "allocated_prefix" {
  value = netbox_prefix.test_child.prefix
}
```

Expected behavior:
- First apply: Allocates 192.168.0.0/24 (or next available)
- Output shows the allocated CIDR
- Subsequent applies: No changes (idempotent)

## Files Modified

- `internal/provider/resource_prefix.go`
  - Added `ParentPrefixID` and `PrefixLength` fields to model
  - Updated schema to make `prefix` optional/computed
  - Added `autoassign`, `parent_prefix_id`, `prefix_length` attributes
  - Rewrote `Create()` method with three paths:
    1. Autoassign with optional upsert
    2. Manual with optional upsert (existing behavior)
    3. Normal create (existing behavior)
  - Added validation for autoassign requirements

- `examples/resources/netbox_prefix/resource.tf`
  - Added examples for all three usage modes

- `docs/resources/prefix.md`
  - Auto-generated documentation with new attributes

## Future Enhancements

Potential improvements for future versions:

1. **Multiple Prefix Allocation**: Support requesting multiple prefixes at once
2. **Prefix Family Selection**: Add IPv4/IPv6 family preference
3. **Custom Allocation Strategy**: Support NetBox's allocation strategies (first-available, etc.)
4. **Conflict Resolution**: Advanced upsert logic for partial matches
5. **Import Support**: Enhanced import for auto-assigned prefixes
6. **Validation**: Add validators for prefix_length ranges (0-32 for IPv4, 0-128 for IPv6)
