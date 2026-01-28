package validators

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = &hexColorValidator{}

// HexColorValidator validates that a string is a valid hex color code
type hexColorValidator struct{}

func (v hexColorValidator) Description(ctx context.Context) string {
	return "value must be a valid hex color code (e.g., #FF5733 or ff5733)"
}

func (v hexColorValidator) MarkdownDescription(ctx context.Context) string {
	return "value must be a valid hex color code (e.g., `#FF5733` or `ff5733`)"
}

func (v hexColorValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	// Hex color: optional # followed by 6 hex digits, or 3 hex digits
	hexColorRegex := regexp.MustCompile(`^#?([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

	if !hexColorRegex.MatchString(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Hex Color",
			fmt.Sprintf("Value %q is not a valid hex color code. Must be in format #RRGGBB or RRGGBB", value),
		)
	}
}

func HexColor() validator.String {
	return hexColorValidator{}
}
