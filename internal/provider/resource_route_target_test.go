package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRouteTargetResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccRouteTargetResourceConfig("65000:100", "Test route target"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_route_target.test", "name", "65000:100"),
					resource.TestCheckResourceAttr("netbox_route_target.test", "description", "Test route target"),
					resource.TestCheckResourceAttrSet("netbox_route_target.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccRouteTargetResourceConfig("65000:100", "Updated route target description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_route_target.test", "description", "Updated route target description"),
				),
			},
		},
	})
}

func testAccRouteTargetResourceConfig(name, description string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tag" "rt1" {
  name = "test-rt-bgp"
  slug = "test-rt-bgp"
  color = "ff5733"
  upsert = true
}

resource "netbox_tag" "rt2" {
  name = "test-rt-mpls"
  slug = "test-rt-mpls"
  color = "0099cc"
  upsert = true
}

resource "netbox_route_target" "test" {
  name        = %[1]q
  description = %[2]q
  tags = [
    {
      name = netbox_tag.rt1.name
      slug = netbox_tag.rt1.slug
    },
    {
      name = netbox_tag.rt2.name
      slug = netbox_tag.rt2.slug
    }
  ]
}
`, name, description)
}
