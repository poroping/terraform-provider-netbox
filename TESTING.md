# Testing Guide

This document describes how to run tests for the NetBox Terraform provider.

## Prerequisites

### For Unit Tests
- Go 1.21 or later
- No external dependencies required

### For Acceptance Tests
- Go 1.21 or later
- Access to a NetBox instance (can be demo.netbox.dev or local)
- NetBox API token with appropriate permissions

## Environment Variables

Set these environment variables for acceptance tests:

```bash
export NETBOX_URL="https://demo.netbox.dev"
export NETBOX_TOKEN="your-api-token-here"
```

For local testing with self-signed certificates:
```bash
export TF_ACC=1  # Required to run acceptance tests
```

## Running Tests

### Quick Start with Test Script

The repository includes a test runner script that makes it easy to run different types of tests:

```bash
# Run all unit tests
./test.sh unit

# Run all acceptance tests (interactive prompt)
./test.sh acceptance

# Run tests for a specific resource
./test.sh resource tenant

# Run tests for a specific data source
./test.sh datasource vrf

# Run validator tests
./test.sh validators

# Run everything (with prompts)
./test.sh all
```

### Using Make Commands

```bash
# Run unit tests
make test

# Run acceptance tests
make testacc

# Run linter
make lint

# Generate documentation
make generate
```

### Using Go Test Directly

#### Unit Tests
```bash
# All unit tests
go test -v -cover -timeout=120s -parallel=4 ./...

# Specific package
go test -v ./internal/validators/...
```

#### Acceptance Tests

**Important**: Acceptance tests will create, modify, and delete resources in your NetBox instance. Use a test instance, not production!

```bash
# All acceptance tests
TF_ACC=1 go test ./internal/provider/... -v -timeout 120m

# Specific resource
TF_ACC=1 go test ./internal/provider/... -v -run=TestAccTenantResource -timeout 120m

# Specific data source
TF_ACC=1 go test ./internal/provider/... -v -run=TestAccTenantDataSource -timeout 120m

# With verbose logging
TF_ACC=1 TF_LOG=DEBUG go test ./internal/provider/... -v -run=TestAccTenantResource -timeout 120m
```

## Test Structure

### Unit Tests
- Located in: `internal/validators/*_test.go`
- Fast, no external dependencies
- Test individual functions and validators

### Acceptance Tests
- Located in: `internal/provider/*_test.go`
- Require NetBox instance
- Test full resource lifecycle (Create, Read, Update, Delete)
- Use `resource.Test` from terraform-plugin-testing

## Writing New Tests

### Acceptance Test Example

```go
func TestAccMyResource(t *testing.T) {
    resource.Test(t, resource.TestCase{
        PreCheck:                 func() { testAccPreCheck(t) },
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        Steps: []resource.TestStep{
            {
                Config: testAccMyResourceConfig("test-value"),
                Check: resource.ComposeAggregateTestCheckFunc(
                    resource.TestCheckResourceAttr("netbox_myresource.test", "name", "test-value"),
                    resource.TestCheckResourceAttrSet("netbox_myresource.test", "id"),
                ),
            },
        },
    })
}

func testAccMyResourceConfig(name string) string {
    return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_myresource" "test" {
  name = %[1]q
}
`, name)
}
```

## Continuous Integration

The provider includes GitHub Actions workflows (when configured) that run:
- Unit tests on every PR
- Acceptance tests on specific branches (when NetBox credentials are available)
- Linting and formatting checks

## Test Coverage

View test coverage:

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out

# View coverage summary
go tool cover -func=coverage.out
```

## Debugging Tests

### Enable Terraform Logging
```bash
export TF_LOG=DEBUG
export TF_LOG_PATH=./terraform.log
```

### Run Single Test with Verbose Output
```bash
TF_ACC=1 go test -v ./internal/provider -run TestAccTenantResource -timeout 120m
```

### Use Delve for Debugging
```bash
TF_ACC=1 dlv test ./internal/provider -- -test.run TestAccTenantResource -test.v
```

## Common Issues

### "TF_ACC must be set for acceptance tests"
Set the environment variable: `export TF_ACC=1`

### "NETBOX_URL must be set"
Set your NetBox URL: `export NETBOX_URL=https://demo.netbox.dev`

### "NETBOX_TOKEN must be set"
Set your API token: `export NETBOX_TOKEN=your-token-here`

### Timeout Errors
Increase timeout: `-timeout 180m`

### Certificate Errors
For self-signed certificates, you may need to set `insecure = true` in the provider configuration or handle certificate verification in your test setup.

## Best Practices

1. **Use Test NetBox Instance**: Never run acceptance tests against production
2. **Clean Up Resources**: Tests should clean up after themselves (automatically handled by terraform-plugin-testing)
3. **Unique Names**: Use unique resource names to avoid conflicts
4. **Test All CRUD Operations**: Create, Read, Update, Delete
5. **Test Edge Cases**: Empty values, special characters, boundary conditions
6. **Fast Unit Tests**: Keep unit tests fast and deterministic
7. **Isolated Acceptance Tests**: Each test should be independent

## Test Organization

```
internal/
├── provider/
│   ├── provider_test.go              # Test helpers and config
│   ├── resource_tenant_test.go       # Tenant resource tests
│   ├── resource_vrf_test.go          # VRF resource tests
│   ├── data_source_tenant_test.go    # Tenant data source tests
│   └── ...
└── validators/
    └── (no tests yet - validators are simple)
```
