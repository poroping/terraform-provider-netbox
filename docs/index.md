---
page_title: "netbox Provider"
subcategory: ""
description: |-
  Terraform provider for managing NetBox IPAM and DCIM resources.
---

# netbox Provider

The NetBox provider allows you to manage [NetBox](https://netbox.dev/) resources using Terraform. NetBox is an infrastructure resource modeling (IRM) application designed to empower network automation.

This provider focuses on IPAM (IP Address Management) and organizational resources, enabling automated allocation and management of:

- IP addresses, prefixes, and ranges
- VLANs and VLAN groups
- VRFs (Virtual Routing and Forwarding instances)
- ASNs (Autonomous System Numbers) and ASN ranges
- Route targets
- Tenants and tags

## Key Features

- **Automatic Resource Allocation**: Resources support `autoassign` for automatic allocation of IPs, prefixes, VLANs, and ASNs from pools
- **Idempotent Operations**: Resources support `allow_append` to find and use existing resources instead of creating duplicates
- **Full CRUD Support**: Complete Create, Read, Update, Delete operations for all resources
- **Data Sources**: Query existing NetBox resources for use in Terraform configurations
- **Tag Support**: All resources support tagging for organization and categorization

## Example Usage

```terraform
terraform {
  required_providers {
    netbox = {
      source = "local/justinr/netbox"
    }
  }
}

provider "netbox" {
  url   = "https://demo.netbox.dev"
  token = "eBmNSu3cFxCZnY9qwoX8xi93BJniL6qjvbE5P70j"
}

# Create a tenant
resource "netbox_tenant" "example" {
  name        = "tf-example-tenant"
  slug        = "tf-example"
  description = "Example tenant created by Terraform"
}

# Create a RIR
resource "netbox_rir" "example" {
  name        = "tf-example-rir"
  description = "Example RIR for testing"
  is_private  = true
}

# Create an ASN range
resource "netbox_asn_range" "example" {
  name        = "tf-example-asn-range"
  start       = 64512
  end         = 64522
  rir         = netbox_rir.example.id
  description = "Private ASN range for testing"
}

# Create a VRF
resource "netbox_vrf" "example" {
  name        = "tf-example-vrf"
  rd          = "64512:100"
  description = "Example VRF"
  tenant      = netbox_tenant.example.id
}

# Create a VLAN group
resource "netbox_vlan_group" "example" {
  name        = "tf-example-vlan-group"
  description = "Example VLAN group"
  min_vid     = 100
  max_vid     = 199
}
```

## Authentication

The provider requires a NetBox API token for authentication. You can configure authentication in three ways:

1. **Provider Configuration** (shown above)
2. **Environment Variables**:
   ```bash
   export NETBOX_URL="https://netbox.example.com"
   export NETBOX_TOKEN="your-api-token-here"
   ```
3. **Terraform Variables** (recommended for production)

## Schema

### Optional

- `url` (String) NetBox API URL. Can also be set via `NETBOX_URL` environment variable.
- `token` (String, Sensitive) NetBox API authentication token. Can also be set via `NETBOX_TOKEN` environment variable.
- `insecure` (Boolean) Skip TLS certificate verification. Not recommended for production use. Defaults to `false`.
