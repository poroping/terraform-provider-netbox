package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTenantDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTenantDataSourceConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.netbox_tenant.test", "id"),
					resource.TestCheckResourceAttr("data.netbox_tenant.test", "name", "test-tenant-ds"),
				),
			},
		},
	})
}

func testAccTenantDataSourceConfig() string {
	return testAccProviderConfig() + `
resource "netbox_tenant" "test" {
  name = "test-tenant-ds"
}

data "netbox_tenant" "test" {
  name = netbox_tenant.test.name
}
`
}
