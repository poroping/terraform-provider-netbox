package validators

import (
	"context"
	"fmt"
	"net"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = &ipAddressValidator{}
var _ validator.String = &cidrValidator{}
var _ validator.String = &ipv4Validator{}
var _ validator.String = &ipv6Validator{}

// IPAddressValidator validates that a string is a valid IP address (IPv4 or IPv6)
type ipAddressValidator struct{}

func (v ipAddressValidator) Description(ctx context.Context) string {
	return "value must be a valid IP address (IPv4 or IPv6)"
}

func (v ipAddressValidator) MarkdownDescription(ctx context.Context) string {
	return "value must be a valid IP address (IPv4 or IPv6)"
}

func (v ipAddressValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	if net.ParseIP(value) == nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IP Address",
			fmt.Sprintf("Value %q is not a valid IP address", value),
		)
	}
}

func IPAddress() validator.String {
	return ipAddressValidator{}
}

// CIDRValidator validates that a string is in valid CIDR notation
type cidrValidator struct{}

func (v cidrValidator) Description(ctx context.Context) string {
	return "value must be in valid CIDR notation (e.g., 192.168.1.0/24 or 2001:db8::/32)"
}

func (v cidrValidator) MarkdownDescription(ctx context.Context) string {
	return "value must be in valid CIDR notation (e.g., `192.168.1.0/24` or `2001:db8::/32`)"
}

func (v cidrValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	_, _, err := net.ParseCIDR(value)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid CIDR Notation",
			fmt.Sprintf("Value %q is not valid CIDR notation: %s", value, err),
		)
	}
}

func CIDR() validator.String {
	return cidrValidator{}
}

// IPv4Validator validates that a string is a valid IPv4 address
type ipv4Validator struct{}

func (v ipv4Validator) Description(ctx context.Context) string {
	return "value must be a valid IPv4 address"
}

func (v ipv4Validator) MarkdownDescription(ctx context.Context) string {
	return "value must be a valid IPv4 address"
}

func (v ipv4Validator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	ip := net.ParseIP(value)
	if ip == nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IPv4 Address",
			fmt.Sprintf("Value %q is not a valid IPv4 address", value),
		)
		return
	}

	if ip.To4() == nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IPv4 Address",
			fmt.Sprintf("Value %q is not an IPv4 address", value),
		)
	}
}

func IPv4() validator.String {
	return ipv4Validator{}
}

// IPv6Validator validates that a string is a valid IPv6 address
type ipv6Validator struct{}

func (v ipv6Validator) Description(ctx context.Context) string {
	return "value must be a valid IPv6 address"
}

func (v ipv6Validator) MarkdownDescription(ctx context.Context) string {
	return "value must be a valid IPv6 address"
}

func (v ipv6Validator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	ip := net.ParseIP(value)
	if ip == nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IPv6 Address",
			fmt.Sprintf("Value %q is not a valid IPv6 address", value),
		)
		return
	}

	if ip.To4() != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IPv6 Address",
			fmt.Sprintf("Value %q is not an IPv6 address", value),
		)
	}
}

func IPv6() validator.String {
	return ipv6Validator{}
}

// IPAddressWithCIDRValidator validates that a string is a valid IP address with CIDR suffix
type ipAddressWithCIDRValidator struct{}

func (v ipAddressWithCIDRValidator) Description(ctx context.Context) string {
	return "value must be an IP address with CIDR suffix (e.g., 192.168.1.1/24)"
}

func (v ipAddressWithCIDRValidator) MarkdownDescription(ctx context.Context) string {
	return "value must be an IP address with CIDR suffix (e.g., `192.168.1.1/24`)"
}

func (v ipAddressWithCIDRValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	ip, _, err := net.ParseCIDR(value)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IP Address with CIDR",
			fmt.Sprintf("Value %q is not a valid IP address with CIDR suffix: %s", value, err),
		)
		return
	}

	// Validate that it's an actual IP address, not a network address
	if ip == nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IP Address",
			fmt.Sprintf("Value %q does not contain a valid IP address", value),
		)
	}
}

func IPAddressWithCIDR() validator.String {
	return ipAddressWithCIDRValidator{}
}

// RouteTargetValidator validates route target format (ASN:NN or IP:NN)
type routeTargetValidator struct{}

func (v routeTargetValidator) Description(ctx context.Context) string {
	return "value must be a valid route target in format ASN:NN or IP:NN"
}

func (v routeTargetValidator) MarkdownDescription(ctx context.Context) string {
	return "value must be a valid route target in format `ASN:NN` or `IP:NN`"
}

func (v routeTargetValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueString()
	// Route target format: ASN:NN (e.g., 65000:100) or IP:NN (e.g., 192.168.1.1:100)
	routeTargetRegex := regexp.MustCompile(`^(\d+:\d+|[0-9.]+:\d+)$`)

	if !routeTargetRegex.MatchString(value) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Route Target Format",
			fmt.Sprintf("Value %q is not a valid route target. Must be in format ASN:NN (e.g., 65000:100) or IP:NN (e.g., 192.168.1.1:100)", value),
		)
	}
}

func RouteTarget() validator.String {
	return routeTargetValidator{}
}
