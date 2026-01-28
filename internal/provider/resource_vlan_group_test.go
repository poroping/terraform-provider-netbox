package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVLANGroupResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccVLANGroupResourceConfig("test-vlan-group", 100, 200, "Test VLAN group"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_vlan_group.test", "name", "test-vlan-group"),
					resource.TestCheckResourceAttr("netbox_vlan_group.test", "min_vid", "100"),
					resource.TestCheckResourceAttr("netbox_vlan_group.test", "max_vid", "200"),
					resource.TestCheckResourceAttr("netbox_vlan_group.test", "description", "Test VLAN group"),
					resource.TestCheckResourceAttrSet("netbox_vlan_group.test", "id"),
					resource.TestCheckResourceAttrSet("netbox_vlan_group.test", "slug"),
				),
			},
			// Update and Read testing
			{
				Config: testAccVLANGroupResourceConfig("test-vlan-group", 100, 300, "Updated description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_vlan_group.test", "max_vid", "300"),
					resource.TestCheckResourceAttr("netbox_vlan_group.test", "description", "Updated description"),
				),
			},
		},
	})
}

func testAccVLANGroupResourceConfig(name string, minVid, maxVid int, description string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tag" "group1" {
  name = "test-vlangrp-infra"
  slug = "test-vlangrp-infra"
  color = "ff0000"
  upsert = true
}

resource "netbox_tag" "group2" {
  name = "test-vlangrp-mgmt"
  slug = "test-vlangrp-mgmt"
  color = "00ff00"
  upsert = true
}

resource "netbox_vlan_group" "test" {
  name        = %[1]q
  slug        = %[1]q
  min_vid     = %[2]d
  max_vid     = %[3]d
  description = %[4]q
  tags = [
    {
      name = netbox_tag.group1.name
      slug = netbox_tag.group1.slug
    },
    {
      name = netbox_tag.group2.name
      slug = netbox_tag.group2.slug
    }
  ]
}
`, name, minVid, maxVid, description)
}
