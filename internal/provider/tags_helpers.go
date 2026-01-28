package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TagRef represents a tag reference in Terraform state
type TagRef struct {
	Name types.String `tfsdk:"name"`
	Slug types.String `tfsdk:"slug"`
}

// TagAPIRef represents a tag reference in NetBox API responses
type TagAPIRef struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ConvertTagsToAPI converts Terraform tag references to NetBox API format
func ConvertTagsToAPI(tags []TagRef) []TagAPIRef {
	if len(tags) == 0 {
		return nil
	}

	apiTags := make([]TagAPIRef, len(tags))
	for i, tag := range tags {
		apiTags[i] = TagAPIRef{
			Name: tag.Name.ValueString(),
			Slug: tag.Slug.ValueString(),
		}
	}
	return apiTags
}

// ConvertTagsFromAPI converts NetBox API tag references to Terraform format
func ConvertTagsFromAPI(apiTags []TagAPIRef) []TagRef {
	if len(apiTags) == 0 {
		return nil
	}

	tags := make([]TagRef, len(apiTags))
	for i, apiTag := range apiTags {
		tags[i] = TagRef{
			Name: types.StringValue(apiTag.Name),
			Slug: types.StringValue(apiTag.Slug),
		}
	}
	return tags
}
