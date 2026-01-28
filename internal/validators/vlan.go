package validators

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.Int64 = &vlanIDValidator{}

// VLANIDValidator validates that an integer is a valid VLAN ID (1-4094)
type vlanIDValidator struct{}

func (v vlanIDValidator) Description(ctx context.Context) string {
	return "value must be a valid VLAN ID (1-4094)"
}

func (v vlanIDValidator) MarkdownDescription(ctx context.Context) string {
	return "value must be a valid VLAN ID (1-4094)"
}

func (v vlanIDValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueInt64()
	// Valid VLAN IDs: 1-4094 (0 and 4095 are reserved)
	if value < 1 || value > 4094 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid VLAN ID",
			fmt.Sprintf("VLAN ID %d is out of range. Must be between 1 and 4094", value),
		)
	}
}

func VLANID() validator.Int64 {
	return vlanIDValidator{}
}
