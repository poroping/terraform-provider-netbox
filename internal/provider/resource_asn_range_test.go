package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccASNRangeResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccASNRangeResourceConfig("test-asn-range", 65000, 65100, "Test ASN range"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_asn_range.test", "name", "test-asn-range"),
					resource.TestCheckResourceAttr("netbox_asn_range.test", "start", "65000"),
					resource.TestCheckResourceAttr("netbox_asn_range.test", "end", "65100"),
					resource.TestCheckResourceAttr("netbox_asn_range.test", "description", "Test ASN range"),
					resource.TestCheckResourceAttrSet("netbox_asn_range.test", "id"),
					resource.TestCheckResourceAttrSet("netbox_asn_range.test", "slug"),
				),
			},
			// Update and Read testing
			{
				Config: testAccASNRangeResourceConfig("test-asn-range", 65000, 65200, "Updated ASN range description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_asn_range.test", "end", "65200"),
					resource.TestCheckResourceAttr("netbox_asn_range.test", "description", "Updated ASN range description"),
				),
			},
		},
	})
}

func testAccASNRangeResourceConfig(name string, start, end int, description string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_tag" "range1" {
  name = "test-asnrng-pool"
  slug = "test-asnrng-pool"
  color = "ff5733"
  upsert = true
}

resource "netbox_tag" "range2" {
  name = "test-asnrng-reserved"
  slug = "test-asnrng-reserved"
  color = "0099cc"
  upsert = true
}

resource "netbox_rir" "test" {
  name = "test-rir-asn-range"
  slug = "test-rir-asn-range"
}

resource "netbox_asn_range" "test" {
  name        = %[1]q
  slug        = %[1]q
  start       = %[2]d
  end         = %[3]d
  rir         = netbox_rir.test.id
  description = %[4]q
  tags = [
    {
      name = netbox_tag.range1.name
      slug = netbox_tag.range1.slug
    },
    {
      name = netbox_tag.range2.name
      slug = netbox_tag.range2.slug
    }
  ]
}
`, name, start, end, description)
}
