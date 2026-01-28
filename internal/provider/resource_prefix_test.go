package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPrefixResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccPrefixResourceConfig("10.0.0.0/24", "active"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_prefix.test", "prefix", "10.0.0.0/24"),
					resource.TestCheckResourceAttrSet("netbox_prefix.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccPrefixResourceConfig("10.0.0.0/24", "reserved"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_prefix.test", "prefix", "10.0.0.0/24"),
				),
			},
		},
	})
}

func testAccPrefixResourceConfig(prefix, status string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tag" "prefix1" {
  name = "test-pfx-network"
  slug = "test-pfx-network"
  color = "ff5733"
  upsert = true
}

resource "netbox_tag" "prefix2" {
  name = "test-pfx-ipam"
  slug = "test-pfx-ipam"
  color = "0099cc"
  upsert = true
}

resource "netbox_prefix" "test" {
  prefix = %[1]q
  tags = [
    {
      name = netbox_tag.prefix1.name
      slug = netbox_tag.prefix1.slug
    },
    {
      name = netbox_tag.prefix2.name
      slug = netbox_tag.prefix2.slug
    }
  ]
}
`, prefix)
}

func TestAccPrefixResource_Autoassign(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create parent prefix and auto-allocate child
			{
				Config: testAccPrefixResourceAutoassignConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_prefix.parent", "prefix", "10.100.0.0/16"),
					resource.TestCheckResourceAttrSet("netbox_prefix.child", "id"),
					resource.TestCheckResourceAttrSet("netbox_prefix.child", "prefix"),
				),
			},
		},
	})
}

func TestAccPrefixResource_AutoassignUpsert(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create prefix with autoassign (no upsert)
			{
				Config: testAccPrefixResourceAutoassignUpsertConfig("Initial autoassigned prefix", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_prefix.child", "description", "Initial autoassigned prefix"),
					resource.TestCheckResourceAttr("netbox_prefix.child", "autoassign", "true"),
					resource.TestCheckResourceAttrSet("netbox_prefix.child", "id"),
					resource.TestCheckResourceAttrSet("netbox_prefix.child", "prefix"),
				),
			},
			// Apply again with upsert=true - should find existing prefix instead of allocating new one
			{
				Config: testAccPrefixResourceAutoassignUpsertConfig("Updated via upsert", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_prefix.child", "description", "Updated via upsert"),
					resource.TestCheckResourceAttr("netbox_prefix.child", "upsert", "true"),
					resource.TestCheckResourceAttr("netbox_prefix.child", "autoassign", "true"),
					// ID and prefix should remain the same (reused existing prefix)
					resource.TestCheckResourceAttrSet("netbox_prefix.child", "id"),
					resource.TestCheckResourceAttrSet("netbox_prefix.child", "prefix"),
				),
			},
		},
	})
}

func testAccPrefixResourceAutoassignConfig() string {
	return testAccProviderConfig() + `
resource "netbox_tag" "pfx_auto1" {
  name = "test-pfx-allocated"
  slug = "test-pfx-allocated"
  color = "9e9e9e"
  upsert = true
}

resource "netbox_tag" "pfx_auto2" {
  name = "test-pfx-subnet"
  slug = "test-pfx-subnet"
  color = "00ff00"
  upsert = true
}

resource "netbox_prefix" "parent" {
  prefix      = "10.100.0.0/16"
  description = "Parent prefix for autoassign test"
}

resource "netbox_prefix" "child" {
  autoassign       = true
  parent_prefix_id = netbox_prefix.parent.id
  prefix_length    = 24
  description      = "Auto-allocated child prefix"
  tags = [
    {
      name = netbox_tag.pfx_auto1.name
      slug = netbox_tag.pfx_auto1.slug
    },
    {
      name = netbox_tag.pfx_auto2.name
      slug = netbox_tag.pfx_auto2.slug
    }
  ]
}
`
}

func testAccPrefixResourceAutoassignUpsertConfig(description string, upsert bool) string {
	upsertStr := "false"
	if upsert {
		upsertStr = "true"
	}
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tag" "pfx_upsert1" {
  name = "test-pfx-upsert-tag"
  slug = "test-pfx-upsert-tag"
  color = "ff0000"
  upsert = true
}

resource "netbox_tag" "pfx_upsert2" {
  name = "test-pfx-reuse"
  slug = "test-pfx-reuse"
  color = "00ffff"
  upsert = true
}

resource "netbox_prefix" "parent_upsert" {
  prefix      = "10.200.0.0/16"
  description = "Parent prefix for autoassign upsert test"
}

resource "netbox_prefix" "child" {
  autoassign       = true
  upsert           = %[2]s
  parent_prefix_id = netbox_prefix.parent_upsert.id
  prefix_length    = 24
  description      = %[1]q
  tags = [
    {
      name = netbox_tag.pfx_upsert2.name
      slug = netbox_tag.pfx_upsert2.slug
    },
    {
      name = netbox_tag.pfx_upsert1.name
      slug = netbox_tag.pfx_upsert1.slug
    }
  ]
}
`, description, upsertStr)
}
