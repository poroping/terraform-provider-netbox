package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"netbox": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	// Verify required environment variables are set
	if v := os.Getenv("NETBOX_URL"); v == "" {
		t.Fatal("NETBOX_URL must be set for acceptance tests")
	}
	if v := os.Getenv("NETBOX_TOKEN"); v == "" {
		t.Fatal("NETBOX_TOKEN must be set for acceptance tests")
	}
}

// testAccProviderConfig returns the provider configuration for acceptance tests
func testAccProviderConfig() string {
	return `
provider "netbox" {
  url      = "` + os.Getenv("NETBOX_URL") + `"
  token    = "` + os.Getenv("NETBOX_TOKEN") + `"
  insecure = true
}
`
}
