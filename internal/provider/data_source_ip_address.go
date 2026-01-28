package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/poroping/terraform-provider-netbox/internal/client"
)

var _ datasource.DataSource = &IPAddressDataSource{}

func NewIPAddressDataSource() datasource.DataSource {
	return &IPAddressDataSource{}
}

type IPAddressDataSource struct {
	client *client.Client
}

type IPAddressDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Address     types.String `tfsdk:"address"`
	VRF         types.Int64  `tfsdk:"vrf"`
	Tenant      types.Int64  `tfsdk:"tenant"`
	DNSName     types.String `tfsdk:"dns_name"`
	Description types.String `tfsdk:"description"`
}

func (d *IPAddressDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_address"
}

func (d *IPAddressDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches an IP address from NetBox.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the IP address.",
				Computed:    true,
			},
			"address": schema.StringAttribute{
				Description: "The IP address to look up (in CIDR notation).",
				Required:    true,
			},
			"vrf": schema.Int64Attribute{
				Description: "The VRF ID associated with this IP address.",
				Computed:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "The tenant ID associated with this IP address.",
				Computed:    true,
			},
			"dns_name": schema.StringAttribute{
				Description: "DNS name for the IP address.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the IP address.",
				Computed:    true,
			},
		},
	}
}

func (d *IPAddressDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *IPAddressDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IPAddressDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("address", data.Address.ValueString())

	results, err := d.client.GetList(ctx, "/api/ipam/ip-addresses/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read IP address: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No IP address found: %s", data.Address.ValueString()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d IP addresses matching the criteria. Please be more specific.", len(results)),
		)
		return
	}

	var ipAddress IPAddressAPIModel
	if err := json.Unmarshal(results[0], &ipAddress); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP address response: %s", err))
		return
	}

	data.ID = types.Int64Value(int64(ipAddress.ID))
	data.Address = types.StringValue(ipAddress.Address)

	if ipAddress.VRF != nil {
		data.VRF = types.Int64Value(int64(ipAddress.VRF.ID))
	} else {
		data.VRF = types.Int64Null()
	}

	if ipAddress.Tenant != nil {
		data.Tenant = types.Int64Value(int64(ipAddress.Tenant.ID))
	} else {
		data.Tenant = types.Int64Null()
	}

	if ipAddress.DNSName != "" {
		data.DNSName = types.StringValue(ipAddress.DNSName)
	} else {
		data.DNSName = types.StringNull()
	}

	if ipAddress.Description != "" {
		data.Description = types.StringValue(ipAddress.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
