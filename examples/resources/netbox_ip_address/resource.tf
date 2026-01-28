# Create an IP address
resource "netbox_ip_address" "example" {
  address     = "10.0.0.1/24"
  status      = "active"
  vrf         = netbox_vrf.example.id
  tenant      = netbox_tenant.example.id
  dns_name    = "server1.example.com"
  description = "Example server IP"

  tags = [
    {
      name = "webserver"
      slug = "webserver"
    }
  ]
}

# Auto-assign IP address from prefix
resource "netbox_ip_address" "auto" {
  address    = "" # Will be auto-assigned
  vrf        = netbox_vrf.example.id
  autoassign = true
  status     = "active"
  dns_name   = "auto-server.example.com"
}
