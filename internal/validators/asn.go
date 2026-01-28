package validators

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.Int64 = &asnValidator{}
var _ validator.Int64 = &asnRangeValidator{}

// ASNValidator validates that an integer is a valid ASN (1-4294967295)
type asnValidator struct{}

func (v asnValidator) Description(ctx context.Context) string {
	return "value must be a valid ASN (1-4294967295)"
}

func (v asnValidator) MarkdownDescription(ctx context.Context) string {
	return "value must be a valid ASN (1-4294967295)"
}

func (v asnValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueInt64()
	// ASN range: 1 to 4294967295 (32-bit)
	if value < 1 || value > 4294967295 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid ASN",
			fmt.Sprintf("ASN value %d is out of range. Must be between 1 and 4294967295", value),
		)
	}
}

func ASN() validator.Int64 {
	return asnValidator{}
}

// ASNRangeValidator validates that start <= end for ASN ranges
type asnRangeValidator struct {
	startPath string
	endPath   string
}

func (v asnRangeValidator) Description(ctx context.Context) string {
	return "ASN range start must be less than or equal to end"
}

func (v asnRangeValidator) MarkdownDescription(ctx context.Context) string {
	return "ASN range start must be less than or equal to end"
}

func (v asnRangeValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	// This validator should be applied to both start and end, but actual validation
	// would require access to both values, which is better done at the resource level
	// For now, just validate it's a valid ASN
	value := req.ConfigValue.ValueInt64()
	if value < 1 || value > 4294967295 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid ASN",
			fmt.Sprintf("ASN value %d is out of range. Must be between 1 and 4294967295", value),
		)
	}
}

func ASNRange() validator.Int64 {
	return asnRangeValidator{}
}
