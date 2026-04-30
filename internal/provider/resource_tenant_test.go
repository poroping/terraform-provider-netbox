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

func TestAccTenantResource_UpsertBySlug(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Step 1: create a tenant normally so it exists in NetBox.
			{
				Config: testAccTenantResourceConfigUpsertBySlugSetup("test-sensa-byslug"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_tenant.seed", "name", "test-sensa-byslug"),
					resource.TestCheckResourceAttrSet("netbox_tenant.seed", "slug"),
				),
			},
			// Step 2: a second resource with upsert_by_slug should adopt the
			// existing tenant (matching by slug) instead of creating a duplicate.
			{
				Config: testAccTenantResourceConfigUpsertBySlug("test-sensa-byslug"),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The upserted resource must share the same id as the seed.
					resource.TestCheckResourceAttrPair(
						"netbox_tenant.test", "id",
						"netbox_tenant.seed", "id",
					),
					resource.TestCheckResourceAttr("netbox_tenant.test", "upsert_by_slug", "true"),
					resource.TestCheckResourceAttr("netbox_tenant.test", "description", "Adopted via slug"),
					resource.TestCheckResourceAttrSet("netbox_tenant.test", "slug"),
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

// testAccTenantResourceConfigUpsertBySlugSetup creates the seed tenant only.
func testAccTenantResourceConfigUpsertBySlugSetup(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tenant" "seed" {
  name = %[1]q
}
`, name)
}

// testAccTenantResourceConfigUpsertBySlug keeps the seed and adds the
// upsert_by_slug tenant alongside it.
// seed uses ignore_changes on description because netbox_tenant.test will
// update the shared underlying object; without this Terraform would plan a
// drift-correction update on seed after the refresh, failing the empty-plan
// check.
func testAccTenantResourceConfigUpsertBySlug(name string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tenant" "seed" {
  name = %[1]q

  lifecycle {
    ignore_changes = [description]
  }
}

resource "netbox_tenant" "test" {
  name           = %[1]q
  description    = "Adopted via slug"
  upsert_by_slug = true

  depends_on = [netbox_tenant.seed]
}
`, name)
}
