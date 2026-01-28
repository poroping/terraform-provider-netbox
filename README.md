# terraform-provider-netbox

Terraform provider for NetBox IPAM (IP Address Management) and organization resources, enabling infrastructure-as-code management of NetBox objects with automatic allocation support.

## Features

- **Full IPAM Resource Management**: Manage tenants, VRFs, RIRs, ASNs, VLANs, prefixes, IP ranges, and IP addresses
- **Upsert Behavior**: `upsert` flag enables idempotent resource creation
- **VLAN Auto-Assignment**: Automatically allocate available VLAN IDs from a VLAN group
- **Data Sources**: Query existing NetBox objects by name, slug, or CIDR
- **NetBox Demo Compatible**: Tested against demo.netbox.dev

## Resources

### Organization
- `netbox_tenant` - Organization/tenancy management

### IPAM (IP Address Management)
- `netbox_vrf` - Virtual Routing and Forwarding instances
- `netbox_rir` - Regional Internet Registries  
- `netbox_asn` - Individual AS numbers
- `netbox_asn_range` - ASN allocation pools
- `netbox_vlan_group` - VLAN grouping with VID constraints
- `netbox_vlan` - Individual VLANs
- `netbox_route_target` - BGP route targets for VRF import/export
- `netbox_prefix` - IP prefixes/subnets with allocation pool support
- `netbox_ip_range` - IP address ranges for allocation
- `netbox_ip_address` - Individual IP addresses with DNS and role

## Data Sources

- `netbox_tenant` - Look up tenants by name or slug
- `netbox_vrf` - Look up VRFs by name
- `netbox_prefix` - Look up prefixes by CIDR notation

## Installation

### From Terraform Registry

```hcl
terraform {
  required_providers {
    netbox = {
      source  = "poroping/netbox"
      version = "~> 0.1"
    }
  }
}

provider "netbox" {
  url      = "https://demo.netbox.dev"
  token    = "your-api-token-here"
  insecure = false
}
```

### Local Development

```bash
# Build and install locally
make build
make install
```

For local development, use:
```hcl
terraform {
  required_providers {
    netbox = {
      source = "local/poroping/netbox"
    }
  }
}
```

## Usage Examples

See [examples/resources/complete_ipam/main.tf](examples/resources/complete_ipam/main.tf) for a comprehensive workflow demonstration.

### Quick Example

```hcl
# Organization
resource "netbox_tenant" "example" {
  name         = "Example Corp"
  slug         = "example-corp"
  description  = "Example organization"
  upsert = true
}

# VRF
resource "netbox_vrf" "example" {
  name         = "EXAMPLE-VRF"
  rd           = "65001:100"
  tenant       = netbox_tenant.example.id
  upsert = true
}

# IP Prefix (allocation pool)
resource "netbox_prefix" "web_pool" {
  prefix        = "10.0.101.0/24"
  status        = "active"
  is_pool       = true
  vrf           = netbox_vrf.example.id
  tenant        = netbox_tenant.example.id
  upsert  = true
}

# Individual IP Address
resource "netbox_ip_address" "web01" {
  address      = "10.0.101.10/24"
  status       = "active"
  dns_name     = "web01.example.com"
  vrf          = netbox_vrf.example.id
  upsert = true
}
```

## Key Features

### Upsert Behavior with upsert

```hcl
resource "netbox_tenant" "example" {
  name         = "Example Corp"
  upsert = true  # Finds and updates existing, or creates new
}
```

When `upsert = true`, the provider:
- Searches for existing resources by key fields (name, ASN, CIDR, etc.)
- Updates existing resources to match desired state
- Creates new resources if not found

### Auto-generated Slugs

```hcl
resource "netbox_rir" "example" {
  name = "My RIR"
  # slug automatically generated as "my-rir"
}
```

### VLAN Auto-Assignment

Automatically allocate available VIDs from a VLAN group:

```hcl
resource "netbox_vlan_group" "prod" {
  name    = "Production"
  min_vid = 100
  max_vid = 199
}

resource "netbox_vlan" "auto" {
  name        = "Web-Servers"
  autoassign  = true  # Automatically assigns available VID
  group       = netbox_vlan_group.prod.id
  status      = "active"
}

# The VID is computed automatically
output "assigned_vid" {
  value = netbox_vlan.auto.vid
}
```

**How it works:**
- Checks for existing VLAN with the same name in the group (prevents duplicates)
- If not found, calls `/api/ipam/vlan-groups/{id}/available-vlans/` to allocate
- Idempotent: multiple runs won't create duplicate VLANs

See [examples/resources/vlan_autoassign/](examples/resources/vlan_autoassign/) for complete examples.

## Provider Configuration

### Arguments

- `url` - (Required) NetBox API URL. Env: `NETBOX_URL`
- `token` - (Required) NetBox API token. Env: `NETBOX_TOKEN`
- `insecure` - (Optional) Skip TLS verification

### Environment Variables

```bash
export NETBOX_URL="https://netbox.example.com"
export NETBOX_TOKEN="your-api-token-here"
```

## Development

### Requirements

- Go 1.24+
- Terraform 1.x
- NetBox API v3.x+

### Commands

```bash
make deps      # Install dependencies
make build     # Build provider
make install   # Install locally
make test      # Run tests
make testacc   # Run acceptance tests (TF_ACC=1 required)
make generate  # Generate documentation
make lint      # Run linter
```

### Project Structure

```
.
├── internal/
│   ├── client/          # NetBox API client
│   └── provider/        # Provider, resources, data sources
├── examples/            # Usage examples
├── Makefile
└── main.go
```

## Testing Against demo.netbox.dev

```hcl
provider "netbox" {
  url   = "https://demo.netbox.dev"
  token = "eBmNSu3cFxCZnY9qwoX8xi93BJniL6qjvbE5P70j"
}
```

**Note:** Demo instance is shared. Use `upsert = true` for idempotent testing.

## License

MPL-2.0

## Support

- **NetBox API Docs:** https://docs.netbox.dev/en/stable/integrations/rest-api/
- **Terraform Plugin Framework:** https://developer.hashicorp.com/terraform/plugin/framework
- **NetBox Demo:** https://demo.netbox.dev/

---

**Version:** 0.0.1 (Development)
