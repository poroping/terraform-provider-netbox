package provider

import (
	"regexp"
	"strings"
)

// Slugify converts a string to a URL-friendly slug
// Converts to lowercase, replaces spaces with hyphens, removes invalid characters
func Slugify(s string) string {
	// Convert to lowercase
	slug := strings.ToLower(s)

	// Replace spaces with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")

	// Replace underscores with hyphens for consistency
	slug = strings.ReplaceAll(slug, "_", "-")

	// Remove any character that's not alphanumeric, hyphen, or underscore
	// NetBox allows: letters, numbers, underscores, hyphens
	reg := regexp.MustCompile(`[^a-z0-9\-_]+`)
	slug = reg.ReplaceAllString(slug, "")

	// Remove leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	// Replace multiple consecutive hyphens with single hyphen
	reg = regexp.MustCompile(`-+`)
	slug = reg.ReplaceAllString(slug, "-")

	// If slug is empty after cleaning, return a default
	if slug == "" {
		slug = "unnamed"
	}

	return slug
}
