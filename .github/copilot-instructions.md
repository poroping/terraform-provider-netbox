# Copilot Instructions for terraform-provider-netbox

## Project Overview
This is a Terraform provider for NetBox, enabling infrastructure-as-code management of NetBox resources. The provider follows the Terraform Plugin Framework SDK patterns. The intent is to be able to automatically allocate and assign asns, vlans, prefixes and ip-addresses from a pool to other Terraform-managed resources in a reproducible manner.

## Architecture & Structure

### Standard Terraform Provider Layout
- `internal/provider/` - Provider, data sources, and resource implementations
- `internal/client/` - NetBox API client wrapper (when implemented)
- `examples/` - Example Terraform configurations for each resource
- `docs/` - Auto-generated provider documentation
- `tools.go` - Build-time tool dependencies

### Key Components
- **Provider**: Configures NetBox API connection (URL, token, TLS settings)
- **Resources**: CRUD operations for NetBox IPAM (vrfs, vlans, vlan-groups, route-targets, rirs, prefixes, ip-ranges, ip-addresses, asns, asn-ranges) and Organization (api/tenancy) objects only
- **Data Sources**: Read-only queries for existing NetBox data
- **Schemas**: Define Terraform configuration structure using framework types

## Development Workflow

### Initial Setup
```bash
go mod init github.com/poroping/terraform-provider-netbox
go get github.com/hashicorp/terraform-plugin-framework
go get github.com/hashicorp/terraform-plugin-go
go get github.com/hashicorp/terraform-plugin-testing
```

### Build & Test
```bash
make build          # Build provider binary
make test           # Run unit tests
make testacc        # Run acceptance tests (requires NetBox instance)
make lint           # Run golangci-lint
make generate       # Generate docs
```

### Local Development Testing
```bash
# Build and install locally
make install
# Configure Terraform to use local build
terraform {
  required_providers {
    netbox = {
      source = "local/justinr/netbox"
    }
  }
}
```

## Coding Conventions

### Resource Implementation Pattern
1. Define schema with `schema.Schema` using framework types (`types.String`, `types.Int64`, etc.)
2. Implement `Create/Read/Update/Delete` methods on resource type
3. Use `diag.Diagnostics` for error reporting, not plain errors
4. Map between Terraform state (framework types) and API models explicitly
5. Handle API rate limiting and retries in client layer

### Schema Design
- Use `Required: true` or `Optional: true` + `Computed: true`
- Set `Sensitive: true` for credentials
- Add `Description` for every attribute (feeds documentation)
- Use `PlanModifiers` for computed IDs, timestamps
- Use `Validators` for input validation (e.g., IP format, enum values)
- Each resource should have an `upsert` boolean to control upsert behavior

### State Management
- Always set ID in `Create` and `Read` (typically NetBox object ID)
- Call `Read` at end of `Create` and `Update` to sync computed attributes
- Return `nil` diagnostics on successful `Delete`
- Handle 404 responses gracefully in `Read` (resource may be deleted outside Terraform) but still check the API is responsive.

### NetBox API Patterns
- Base URL: `/api/<app>/<endpoint>/` (e.g., `/api/dcim/devices/`)
- Authentication: `Authorization: Token <api_token>` header
- Pagination: Follow `next` links for list endpoints
- Custom fields: NetBox supports custom fields on most objects
- Change logging: NetBox tracks all changes with timestamps

### Testing
- Unit tests: Mock API responses, test schema logic
- Acceptance tests: Use `resource.Test` with real NetBox instance
  - Require `TF_ACC=1` environment variable
  - Check resource attributes with `resource.TestCheckResourceAttr`
  - Use `testAccCheckResourceDestroy` to verify cleanup

### Error Handling
- Use `resp.Diagnostics.AddError("Summary", "Detail")` not `fmt.Errorf`
- Include NetBox object type and ID in error messages
- Distinguish between user errors (invalid input) and API errors

## External Dependencies
- **NetBox API**: REST API v3.x+ required
- **Terraform Plugin Framework**: v1.x (modern framework, not legacy SDK)
- **Go HTTP client**: Use standard library or `github.com/go-resty/resty` for API calls

## Common Tasks

### Adding a New Resource
1. Create `internal/provider/resource_<name>.go`
2. Implement `schema.Resource` interface
3. Register in `provider.go` `Resources()` method
4. Add example in `examples/resources/<name>/resource.tf`
5. Run `make generate` to create docs

### Adding Custom Field Support
- Use `types.Map` with `types.String` elements for flexible key-value storage
- Document that keys must match NetBox custom field slugs

### Handling NetBox Object Relationships
- Use `types.Int64` for foreign key IDs (preferred for API efficiency)
- Consider adding nested object schemas for complex relationships
- Document when to use ID vs name for lookups

## Key Files to Reference
- Terraform Plugin Framework docs: https://developer.hashicorp.com/terraform/plugin/framework
- NetBox API docs: https://docs.netbox.dev/en/stable/integrations/rest-api/
- NetBox demo API schema: https://demo.netbox.dev/api/schema/
