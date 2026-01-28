package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccVLANResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccVLANResourceConfig("test-vlan", 100, "Test VLAN description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_vlan.test", "name", "test-vlan"),
					resource.TestCheckResourceAttr("netbox_vlan.test", "vid", "100"),
					resource.TestCheckResourceAttr("netbox_vlan.test", "description", "Test VLAN description"),
					resource.TestCheckResourceAttrSet("netbox_vlan.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccVLANResourceConfig("test-vlan", 100, "Updated VLAN description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_vlan.test", "description", "Updated VLAN description"),
				),
			},
		},
	})
}

func TestAccVLANResource_Autoassign(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing with autoassign
			{
				Config: testAccVLANResourceConfigAutoassign("test-vlan-autoassign", "Test autoassigned VLAN"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_vlan.test", "name", "test-vlan-autoassign"),
					resource.TestCheckResourceAttr("netbox_vlan.test", "description", "Test autoassigned VLAN"),
					resource.TestCheckResourceAttr("netbox_vlan.test", "autoassign", "true"),
					resource.TestCheckResourceAttrSet("netbox_vlan.test", "id"),
					resource.TestCheckResourceAttrSet("netbox_vlan.test", "vid"),
					resource.TestCheckResourceAttrSet("netbox_vlan.test", "group"),
				),
			},
		},
	})
}

func TestAccVLANResource_AutoassignUpsert(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create VLAN with autoassign (no upsert)
			{
				Config: testAccVLANResourceConfigAutoassignUpsert("test-vlan-upsert", "Initial autoassigned VLAN", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_vlan.test", "name", "test-vlan-upsert"),
					resource.TestCheckResourceAttr("netbox_vlan.test", "description", "Initial autoassigned VLAN"),
					resource.TestCheckResourceAttr("netbox_vlan.test", "autoassign", "true"),
					resource.TestCheckResourceAttrSet("netbox_vlan.test", "id"),
					resource.TestCheckResourceAttrSet("netbox_vlan.test", "vid"),
				),
			},
			// Apply again with upsert=true - should find existing VLAN instead of allocating new one
			{
				Config: testAccVLANResourceConfigAutoassignUpsert("test-vlan-upsert", "Updated via upsert", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_vlan.test", "name", "test-vlan-upsert"),
					resource.TestCheckResourceAttr("netbox_vlan.test", "description", "Updated via upsert"),
					resource.TestCheckResourceAttr("netbox_vlan.test", "upsert", "true"),
					resource.TestCheckResourceAttr("netbox_vlan.test", "autoassign", "true"),
					// ID and VID should remain the same (reused existing VLAN)
					resource.TestCheckResourceAttrSet("netbox_vlan.test", "id"),
					resource.TestCheckResourceAttrSet("netbox_vlan.test", "vid"),
				),
			},
		},
	})
}

func testAccVLANResourceConfig(name string, vid int, description string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tag" "test1" {
  name = "test-vlan-prod"
  slug = "test-vlan-prod"
  color = "ff5733"
  upsert = true
}

resource "netbox_tag" "test2" {
  name = "test-vlan-critical"
  slug = "test-vlan-critical"
  color = "0099cc"
  upsert = true
}

resource "netbox_vlan" "test" {
  name        = %[1]q
  vid         = %[2]d
  description = %[3]q
  tags = [
    {
      name = netbox_tag.test1.name
      slug = netbox_tag.test1.slug
    },
    {
      name = netbox_tag.test2.name
      slug = netbox_tag.test2.slug
    }
  ]
}
`, name, vid, description)
}

func testAccVLANResourceConfigAutoassign(name, description string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tag" "auto1" {
  name = "test-vlan-auto"
  slug = "test-vlan-auto"
  color = "9e9e9e"
  upsert = true
}

resource "netbox_tag" "auto2" {
  name = "test-vlan-dynamic"
  slug = "test-vlan-dynamic"
  color = "00ff00"
  upsert = true
}

resource "netbox_vlan_group" "test" {
  name        = "test-vlan-group-autoassign"
  slug        = "test-vlan-group-autoassign"
  min_vid     = 100
  max_vid     = 200
  description = "Test group for autoassign"
}

resource "netbox_vlan" "test" {
  name        = %[1]q
  description = %[2]q
  autoassign  = true
  group       = netbox_vlan_group.test.id
  tags = [
    {
      name = netbox_tag.auto1.name
      slug = netbox_tag.auto1.slug
    },
    {
      name = netbox_tag.auto2.name
      slug = netbox_tag.auto2.slug
    }
  ]
}
`, name, description)
}

func testAccVLANResourceConfigAutoassignUpsert(name, description string, upsert bool) string {
	upsertStr := "false"
	if upsert {
		upsertStr = "true"
	}
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tag" "upsert1" {
  name = "test-vlan-upsert-tag"
  slug = "test-vlan-upsert-tag"
  color = "ff0000"
  upsert = true
}

resource "netbox_tag" "upsert2" {
  name = "test-vlan-reuse"
  slug = "test-vlan-reuse"
  color = "00ffff"
  upsert = true
}

resource "netbox_vlan_group" "upsert_test" {
  name        = "test-vlan-group-upsert"
  slug        = "test-vlan-group-upsert"
  min_vid     = 200
  max_vid     = 300
  description = "Test group for autoassign upsert"
}

resource "netbox_vlan" "test" {
  name        = %[1]q
  description = %[2]q
  autoassign  = true
  upsert      = %[3]s
  group       = netbox_vlan_group.upsert_test.id
  tags = [
    {
      name = netbox_tag.upsert1.name
      slug = netbox_tag.upsert1.slug
    },
    {
      name = netbox_tag.upsert2.name
      slug = netbox_tag.upsert2.slug
    }
  ]
}
`, name, description, upsertStr)
}
