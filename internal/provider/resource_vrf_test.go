package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVRFResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccVRFResourceConfig("test-vrf", "65420:666"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_vrf.test", "name", "test-vrf"),
					resource.TestCheckResourceAttr("netbox_vrf.test", "rd", "65420:666"),
					resource.TestCheckResourceAttrSet("netbox_vrf.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccVRFResourceConfig("test-vrf", "64420:666"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_vrf.test", "rd", "64420:666"),
				),
			},
		},
	})
}

func testAccVRFResourceConfig(name, rd string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_vrf" "test" {
  name = %[1]q
  rd   = %[2]q
}
`, name, rd)
}
