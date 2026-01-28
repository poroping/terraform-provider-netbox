# Create a VRF with route distinguisher
resource "netbox_vrf" "example" {
  name        = "example-vrf"
  rd          = "65000:100"
  description = "Example VRF"
  tenant      = netbox_tenant.example.id

  tags = [
    {
      name = "production"
      slug = "production"
    }
  ]
}

# VRF with enforce_unique - prevents duplicate IP addresses within this VRF
resource "netbox_vrf" "unique" {
  name           = "unique-vrf"
  rd             = "65000:200"
  enforce_unique = true
  description    = "VRF with IP uniqueness enforcement"
}
