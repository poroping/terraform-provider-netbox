# Create an ASN manually
resource "netbox_asn" "example" {
  asn         = 64512
  rir         = netbox_rir.example.id
  tenant      = netbox_tenant.example.id
  description = "Example ASN"

  tags = [
    {
      name = "production"
      slug = "production"
    }
  ]
}

# Create an ASN range for auto-allocation
resource "netbox_asn_range" "private" {
  name        = "Private ASN Range"
  slug        = "private-asn-range"
  start       = 64512
  end         = 65534
  rir         = netbox_rir.example.id
  description = "Private ASN range for auto-allocation"
}

# Auto-assign ASN from range
resource "netbox_asn" "auto_allocated" {
  autoassign          = true
  parent_asn_range_id = netbox_asn_range.private.id
  rir                 = netbox_rir.example.id
  tenant              = netbox_tenant.example.id
  description         = "Auto-allocated from range"

  tags = [
    {
      name = "auto-assigned"
      slug = "auto-assigned"
    }
  ]
}

# Auto-assign with upsert: reuse existing ASN with same tenant/tags if found
resource "netbox_asn" "auto_with_upsert" {
  autoassign          = true
  upsert              = true
  parent_asn_range_id = netbox_asn_range.private.id
  rir                 = netbox_rir.example.id
  tenant              = netbox_tenant.example.id
  description         = "Will reuse existing ASN if found"

  tags = [
    {
      name = "reusable"
      slug = "reusable"
    }
  ]
}
