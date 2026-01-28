# Plan Modifiers for Unordered Tag Comparison

## Overview
Implemented a custom plan modifier to handle unordered comparison of tags in all resources. This ensures that reordering tags in Terraform configuration doesn't trigger unnecessary resource updates when the actual tag content hasn't changed.

## Implementation

### Plan Modifier Package
Created `internal/planmodifiers/list.go` with two modifiers:

1. **UnorderedList()** - The main modifier for unordered list comparison
   - Converts both plan and state list elements to string representations
   - Sorts both lists alphabetically
   - Compares sorted lists element by element
   - If lists match (ignoring order), uses state value to prevent unnecessary updates
   - This is the key feature that solves the tag ordering problem

2. **UseStateForUnknownUnlessItemsChange()** - Auxiliary modifier
   - Preserves state value when plan is unknown but config/state items haven't changed
   - Useful for computed list attributes

### Resources Updated
Added the `UnorderedList()` plan modifier to the `tags` attribute in all 11 resources that support tags:

1. resource_tenant.go
2. resource_vrf.go
3. resource_rir.go
4. resource_asn.go
5. resource_asn_range.go
6. resource_vlan_group.go
7. resource_vlan.go
8. resource_route_target.go
9. resource_prefix.go (also added missing tags schema attribute)
10. resource_ip_range.go
11. resource_ip_address.go (also added missing tags schema attribute)

Note: resource_tag.go doesn't have a tags field since it IS a tag resource.

### Schema Pattern
Each resource's tags attribute now includes the plan modifier:

```go
"tags": schema.ListNestedAttribute{
    Description: "Tags associated with this resource.",
    Optional:    true,
    PlanModifiers: []planmodifier.List{
        planmodifiers.UnorderedList(),
    },
    NestedObject: schema.NestedAttributeObject{
        Attributes: map[string]schema.Attribute{
            "name": schema.StringAttribute{
                Description: "Tag name.",
                Required:    true,
            },
            "slug": schema.StringAttribute{
                Description: "Tag slug.",
                Required:    true,
            },
        },
    },
},
```

## Benefits

1. **Better User Experience**: Users can reorder tags in their Terraform configuration without triggering resource updates
2. **Correct Terraform Behavior**: Tag order is semantically meaningless, so it shouldn't affect resource state
3. **Prevents Drift**: Avoids false positives in `terraform plan` when only tag order has changed
4. **Compatible with Formatters**: Works correctly with `terraform fmt` and other tools that might reorder configuration

## Testing

To verify the implementation works correctly:

1. Create a resource with tags in a specific order
2. Apply the configuration
3. Reorder the tags (same content, different order)
4. Run `terraform plan` - should show "No changes"
5. Change tag content (add/remove/modify tags)
6. Run `terraform plan` - should show changes correctly

Example:
```hcl
# Initial configuration
resource "netbox_tenant" "example" {
  name = "Example"
  tags = [
    { name = "Production", slug = "production" },
    { name = "Critical", slug = "critical" },
  ]
}

# After reordering (should show no changes)
resource "netbox_tenant" "example" {
  name = "Example"
  tags = [
    { name = "Critical", slug = "critical" },
    { name = "Production", slug = "production" },
  ]
}

# After changing content (should show changes)
resource "netbox_tenant" "example" {
  name = "Example"
  tags = [
    { name = "Critical", slug = "critical" },
    { name = "Production", slug = "production" },
    { name = "New", slug = "new" },
  ]
}
```

## Build Status
✅ Provider builds successfully with plan modifiers
✅ No compilation errors
✅ All imports resolved correctly
✅ Installed to local Terraform plugins directory

## Files Modified
- Created: `internal/planmodifiers/list.go` (129 lines)
- Modified: 11 resource files to add import and plan modifier
- Fixed: Added missing tags schema attributes to resource_prefix.go and resource_ip_address.go
