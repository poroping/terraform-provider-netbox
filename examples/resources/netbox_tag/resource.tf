# Create a tag
resource "netbox_tag" "example" {
  name        = "production"
  slug        = "production"
  color       = "ff5733"
  description = "Production environment resources"
}

# Tag with hex color including #
resource "netbox_tag" "dev" {
  name  = "development"
  slug  = "development"
  color = "#0099cc"
}

# Use existing tag with upsert - will find by slug and update if exists
resource "netbox_tag" "reusable" {
  name        = "infrastructure"
  slug        = "infrastructure"
  color       = "9e9e9e"
  description = "Infrastructure resources"
  upsert      = true
}
