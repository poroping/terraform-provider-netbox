package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTenantResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTenantResourceConfig("test-sensa", "Test tenant description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_tenant.test", "name", "test-sensa"),
					resource.TestCheckResourceAttrSet("netbox_tenant.test", "slug"),
					resource.TestCheckResourceAttr("netbox_tenant.test", "description", "Test tenant description"),
					resource.TestCheckResourceAttrSet("netbox_tenant.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccTenantResourceConfig("test-sensa", "Updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_tenant.test", "description", "Updated description"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccTenantResource_Upsert(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantResourceConfigUpsert("test-sensa-upsert"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_tenant.test", "name", "test-sensa-upsert"),
					resource.TestCheckResourceAttr("netbox_tenant.test", "upsert", "true"),
					resource.TestCheckResourceAttr("netbox_tenant.test", "description", "Created with upsert"),
					resource.TestCheckResourceAttrSet("netbox_tenant.test", "id"),
				),
			},
		},
	})
}

func testAccTenantResourceConfig(name, description string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tenant" "test" {
  name        = %[1]q
  description = %[2]q
}
`, name, description)
}

func testAccTenantResourceConfigUpsert(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tenant" "first" {
  name = %[1]q

  lifecycle {
    ignore_changes = [
        description,
      ]
    }
}

resource "netbox_tenant" "test" {
  name         = netbox_tenant.first.name
  description  = "Created with upsert"
  upsert       = true
}
`, name)
}
