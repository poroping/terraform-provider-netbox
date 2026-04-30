package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccIPAddressResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccIPAddressResourceConfig("10.0.0.1/24", "active"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_ip_address.test", "address", "10.0.0.1/24"),
					resource.TestCheckResourceAttrSet("netbox_ip_address.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccIPAddressResourceConfigWithDNS("10.0.0.1/24", "active", "test.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_ip_address.test", "dns_name", "test.example.com"),
				),
			},
		},
	})
}

func TestAccIPAddressResource_Autoassign(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIPAddressResourceConfigAutoassign(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Address is computed - just verify it is set and non-empty
					resource.TestCheckResourceAttrSet("netbox_ip_address.auto", "address"),
					resource.TestCheckResourceAttrSet("netbox_ip_address.auto", "id"),
					resource.TestCheckResourceAttr("netbox_ip_address.auto", "dns_name", "auto.example.com"),
					resource.TestCheckResourceAttr("netbox_ip_address.auto", "autoassign", "true"),
				),
			},
		},
	})
}

func TestAccIPAddressResource_AutoassignUpsert(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: create a prefix and allocate an IP from it (seed).
			{
				Config: testAccIPAddressResourceConfigAutoassignUpsertSetup(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("netbox_ip_address.seed", "address"),
					resource.TestCheckResourceAttrSet("netbox_ip_address.seed", "id"),
				),
			},
			// Step 2: a second resource with autoassign+upsert should adopt the
			// existing IP (found by dns_name within the parent prefix) instead of
			// allocating a new one.
			{
				Config: testAccIPAddressResourceConfigAutoassignUpsert(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// Both resources must share the same NetBox ID.
					resource.TestCheckResourceAttrPair(
						"netbox_ip_address.test", "id",
						"netbox_ip_address.seed", "id",
					),
					resource.TestCheckResourceAttr("netbox_ip_address.test", "autoassign", "true"),
					resource.TestCheckResourceAttr("netbox_ip_address.test", "upsert", "true"),
					resource.TestCheckResourceAttr("netbox_ip_address.test", "description", "Adopted via autoassign+upsert"),
				),
			},
		},
	})
}

func testAccIPAddressResourceConfig(address, status string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_ip_address" "test" {
  address = %[1]q
}
`, address)
}

func testAccIPAddressResourceConfigWithDNS(address, status, dnsName string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_ip_address" "test" {
  address  = %[1]q
  dns_name = %[3]q
}
`, address, status, dnsName)
}

func testAccIPAddressResourceConfigAutoassign() string {
	return testAccProviderConfig() + `
resource "netbox_prefix" "pool" {
  prefix      = "203.0.113.0/28"
  description = "Test pool for IP autoassign"
}

resource "netbox_ip_address" "auto" {
  autoassign       = true
  parent_prefix_id = netbox_prefix.pool.id
  dns_name         = "auto.example.com"
  description      = "Auto-allocated IP"
}
`
}

// testAccIPAddressResourceConfigAutoassignUpsertSetup creates the prefix and
// allocates the seed IP via autoassign.
func testAccIPAddressResourceConfigAutoassignUpsertSetup() string {
	return testAccProviderConfig() + `
resource "netbox_prefix" "pool" {
  prefix      = "203.0.113.32/28"
  description = "Test pool for IP autoassign+upsert"
}

resource "netbox_ip_address" "seed" {
  autoassign       = true
  parent_prefix_id = netbox_prefix.pool.id
  dns_name         = "upsert-test.example.com"
  description      = "Original allocation"
}
`
}

// testAccIPAddressResourceConfigAutoassignUpsert keeps the seed resource and
// adds the adopting resource. seed ignores description changes because
// netbox_ip_address.test will update the shared object.
func testAccIPAddressResourceConfigAutoassignUpsert() string {
	return testAccProviderConfig() + `
resource "netbox_prefix" "pool" {
  prefix      = "203.0.113.32/28"
  description = "Test pool for IP autoassign+upsert"
}

resource "netbox_ip_address" "seed" {
  autoassign       = true
  parent_prefix_id = netbox_prefix.pool.id
  dns_name         = "upsert-test.example.com"
  description      = "Original allocation"

  lifecycle {
    ignore_changes = [description]
  }
}

resource "netbox_ip_address" "test" {
  autoassign       = true
  upsert           = true
  parent_prefix_id = netbox_prefix.pool.id
  dns_name         = "upsert-test.example.com"
  description      = "Adopted via autoassign+upsert"

  depends_on = [netbox_ip_address.seed]
}
`
}
