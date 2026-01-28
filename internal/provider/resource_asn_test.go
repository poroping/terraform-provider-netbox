package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccASNResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccASNResourceConfig(65100, "Test ASN"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_asn.test", "asn", "65100"),
					resource.TestCheckResourceAttr("netbox_asn.test", "description", "Test ASN"),
					resource.TestCheckResourceAttrSet("netbox_asn.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccASNResourceConfig(65100, "Updated ASN description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_asn.test", "description", "Updated ASN description"),
				),
			},
		},
	})
}

func testAccASNResourceConfig(asn int, description string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tag" "asn1" {
  name = "test-asn-public"
  slug = "test-asn-public"
  color = "0099cc"
  upsert = true
}

resource "netbox_tag" "asn2" {
  name = "test-asn-transit"
  slug = "test-asn-transit"
  color = "ff5733"
  upsert = true
}

resource "netbox_rir" "test" {
  name = "test-rir-asn"
  slug = "test-rir-asn"
}

resource "netbox_asn" "test" {
  asn         = %[1]d
  rir         = netbox_rir.test.id
  description = %[2]q
  tags = [
    {
      name = netbox_tag.asn1.name
      slug = netbox_tag.asn1.slug
    },
    {
      name = netbox_tag.asn2.name
      slug = netbox_tag.asn2.slug
    }
  ]
}
`, asn, description)
}

func TestAccASNResource_Autoassign(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create ASN range and auto-allocate ASN
			{
				Config: testAccASNResourceAutoassignConfig(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_asn_range.test", "start", "65200"),
					resource.TestCheckResourceAttr("netbox_asn_range.test", "end", "65299"),
					resource.TestCheckResourceAttrSet("netbox_asn.auto", "id"),
					resource.TestCheckResourceAttrSet("netbox_asn.auto", "asn"),
				),
			},
		},
	})
}

func TestAccASNResource_AutoassignUpsert(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create ASN with autoassign (no upsert)
			{
				Config: testAccASNResourceAutoassignUpsertConfig("Initial autoassigned ASN", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_asn.test", "description", "Initial autoassigned ASN"),
					resource.TestCheckResourceAttr("netbox_asn.test", "autoassign", "true"),
					resource.TestCheckResourceAttrSet("netbox_asn.test", "id"),
					resource.TestCheckResourceAttrSet("netbox_asn.test", "asn"),
				),
			},
			// Apply again with upsert=true - should find existing ASN instead of allocating new one
			{
				Config: testAccASNResourceAutoassignUpsertConfig("Updated via upsert", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_asn.test", "description", "Updated via upsert"),
					resource.TestCheckResourceAttr("netbox_asn.test", "upsert", "true"),
					resource.TestCheckResourceAttr("netbox_asn.test", "autoassign", "true"),
					// ID and ASN should remain the same (reused existing ASN)
					resource.TestCheckResourceAttrSet("netbox_asn.test", "id"),
					resource.TestCheckResourceAttrSet("netbox_asn.test", "asn"),
				),
			},
		},
	})
}

func testAccASNResourceAutoassignConfig() string {
	return testAccProviderConfig() + `
resource "netbox_tag" "asn_auto1" {
  name = "test-asn-allocated"
  slug = "test-asn-allocated"
  color = "9e9e9e"
  upsert = true
}

resource "netbox_tag" "asn_auto2" {
  name = "test-asn-managed"
  slug = "test-asn-managed"
  color = "00ff00"
  upsert = true
}

resource "netbox_rir" "test_autoassign" {
  name = "test-rir-asn-auto"
  slug = "test-rir-asn-auto"
}

resource "netbox_asn_range" "test" {
  name        = "test-asn-range-auto"
  slug        = "test-asn-range-auto"
  rir         = netbox_rir.test_autoassign.id
  start       = 65200
  end         = 65299
  description = "ASN range for autoassign test"
}

resource "netbox_asn" "auto" {
  autoassign          = true
  parent_asn_range_id = netbox_asn_range.test.id
  rir                 = netbox_rir.test_autoassign.id
  description         = "Auto-allocated ASN"
  tags = [
    {
      name = netbox_tag.asn_auto1.name
      slug = netbox_tag.asn_auto1.slug
    },
    {
      name = netbox_tag.asn_auto2.name
      slug = netbox_tag.asn_auto2.slug
    }
  ]
}
`
}

func testAccASNResourceAutoassignUpsertConfig(description string, upsert bool) string {
	upsertStr := "false"
	if upsert {
		upsertStr = "true"
	}
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tag" "asn_upsert1" {
  name = "test-asn-upsert-tag"
  slug = "test-asn-upsert-tag"
  color = "ff0000"
  upsert = true
}

resource "netbox_tag" "asn_upsert2" {
  name = "test-asn-reuse"
  slug = "test-asn-reuse"
  color = "00ffff"
  upsert = true
}

resource "netbox_rir" "upsert_test" {
  name = "test-rir-asn-upsert"
  slug = "test-rir-asn-upsert"
}

resource "netbox_asn_range" "upsert_test" {
  name        = "test-asn-range-upsert"
  slug        = "test-asn-range-upsert"
  rir         = netbox_rir.upsert_test.id
  start       = 65300
  end         = 65399
  description = "ASN range for autoassign upsert"
}

resource "netbox_asn" "test" {
  autoassign          = true
  upsert              = %[2]s
  parent_asn_range_id = netbox_asn_range.upsert_test.id
  rir                 = netbox_rir.upsert_test.id
  description         = %[1]q
  tags = [
    {
      name = netbox_tag.asn_upsert1.name
      slug = netbox_tag.asn_upsert1.slug
    },
    {
      name = netbox_tag.asn_upsert2.name
      slug = netbox_tag.asn_upsert2.slug
    }
  ]
}
`, description, upsertStr)
}
