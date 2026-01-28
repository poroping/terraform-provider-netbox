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

var _ datasource.DataSource = &IPRangeDataSource{}

func NewIPRangeDataSource() datasource.DataSource {
	return &IPRangeDataSource{}
}

type IPRangeDataSource struct {
	client *client.Client
}

type IPRangeDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	StartAddr   types.String `tfsdk:"start_address"`
	EndAddr     types.String `tfsdk:"end_address"`
	Status      types.String `tfsdk:"status"`
	VRF         types.Int64  `tfsdk:"vrf"`
	Tenant      types.Int64  `tfsdk:"tenant"`
	Description types.String `tfsdk:"description"`
}

func (d *IPRangeDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_range"
}

func (d *IPRangeDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches an IP range from NetBox by start and end addresses.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the IP range.",
				Computed:    true,
			},
			"start_address": schema.StringAttribute{
				Description: "The starting IP address of the range.",
				Required:    true,
			},
			"end_address": schema.StringAttribute{
				Description: "The ending IP address of the range.",
				Required:    true,
			},
			"status": schema.StringAttribute{
				Description: "The status of the IP range.",
				Computed:    true,
			},
			"vrf": schema.Int64Attribute{
				Description: "The VRF ID associated with this IP range.",
				Computed:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "The tenant ID associated with this IP range.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the IP range.",
				Computed:    true,
			},
		},
	}
}

func (d *IPRangeDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *IPRangeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IPRangeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("start_address", data.StartAddr.ValueString())
	params.Set("end_address", data.EndAddr.ValueString())

	results, err := d.client.GetList(ctx, "/api/ipam/ip-ranges/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read IP range: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No IP range found with start: %s, end: %s", data.StartAddr.ValueString(), data.EndAddr.ValueString()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d IP ranges matching the criteria. Please be more specific.", len(results)),
		)
		return
	}

	var ipRange IPRangeAPIModel
	if err := json.Unmarshal(results[0], &ipRange); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP range response: %s", err))
		return
	}

	data.ID = types.Int64Value(int64(ipRange.ID))
	data.StartAddr = types.StringValue(ipRange.StartAddress)
	data.EndAddr = types.StringValue(ipRange.EndAddress)

	if ipRange.Status != "" {
		data.Status = types.StringValue(ipRange.Status)
	} else {
		data.Status = types.StringNull()
	}

	if ipRange.VRF != nil {
		data.VRF = types.Int64Value(int64(ipRange.VRF.ID))
	} else {
		data.VRF = types.Int64Null()
	}

	if ipRange.Tenant != nil {
		data.Tenant = types.Int64Value(int64(ipRange.Tenant.ID))
	} else {
		data.Tenant = types.Int64Null()
	}

	if ipRange.Description != "" {
		data.Description = types.StringValue(ipRange.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
