package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccIPAddressResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccIPAddressResourceConfig("10.0.0.1/24", "active"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_ip_address.test", "address", "10.0.0.1/24"),
					resource.TestCheckResourceAttrSet("netbox_ip_address.test", "id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccIPAddressResourceConfigWithDNS("10.0.0.1/24", "active", "test.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("netbox_ip_address.test", "dns_name", "test.example.com"),
				),
			},
		},
	})
}

func testAccIPAddressResourceConfig(address, status string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_ip_address" "test" {
  address = %[1]q
}
`, address)
}

func testAccIPAddressResourceConfigWithDNS(address, status, dnsName string) string {
	return testAccProviderConfig() + fmt.Sprintf(`
resource "netbox_ip_address" "test" {
  address  = %[1]q
  dns_name = %[3]q
}
`, address, status, dnsName)
}
