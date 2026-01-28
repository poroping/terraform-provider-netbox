package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTagResourceConfig("sensa-tag", "ff5733", "Test tag description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_tag.test", "name", "sensa-tag"),
					resource.TestCheckResourceAttr("netbox_tag.test", "slug", "sensa-tag"),
					resource.TestCheckResourceAttr("netbox_tag.test", "color", "ff5733"),
					resource.TestCheckResourceAttr("netbox_tag.test", "description", "Test tag description"),
					resource.TestCheckResourceAttrSet("netbox_tag.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccTagResourceConfig("sensa-tag", "0099cc", "Updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_tag.test", "color", "0099cc"),
					resource.TestCheckResourceAttr("netbox_tag.test", "description", "Updated description"),
				),
			},
		},
	})
}

func testAccTagResourceConfig(name, color, description string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tag" "test" {
  name        = %[1]q
  slug        = %[1]q
  color       = %[2]q
  description = %[3]q
}
`, name, color, description)
}
