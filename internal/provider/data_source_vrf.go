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

var _ datasource.DataSource = &VRFDataSource{}

func NewVRFDataSource() datasource.DataSource {
	return &VRFDataSource{}
}

type VRFDataSource struct {
	client *client.Client
}

type VRFDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	RD          types.String `tfsdk:"rd"`
	Tenant      types.Int64  `tfsdk:"tenant"`
	Description types.String `tfsdk:"description"`
}

func (d *VRFDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vrf"
}

func (d *VRFDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a VRF from NetBox by name or RD.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the VRF.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the VRF to look up.",
				Optional:    true,
			},
			"rd": schema.StringAttribute{
				Description: "The route distinguisher of the VRF.",
				Computed:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "The tenant ID associated with the VRF.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the VRF.",
				Computed:    true,
			},
		},
	}
}

func (d *VRFDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VRFDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VRFDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters
	params := url.Values{}
	if !data.Name.IsNull() {
		params.Set("name", data.Name.ValueString())
	}

	if len(params) == 0 {
		resp.Diagnostics.AddError(
			"Missing Search Criteria",
			"Name must be provided to look up a VRF.",
		)
		return
	}

	// Query NetBox API
	results, err := d.client.GetList(ctx, "/api/ipam/vrfs/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read VRF: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No VRF found matching the criteria: %s", params.Encode()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d VRFs matching the criteria. Please be more specific.", len(results)),
		)
		return
	}

	// Map API response to data source model
	var vrf VRFAPIModel
	if err := json.Unmarshal(results[0], &vrf); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VRF response: %s", err))
		return
	}
	data.ID = types.Int64Value(int64(vrf.ID))
	data.Name = types.StringValue(vrf.Name)

	if vrf.RD != "" {
		data.RD = types.StringValue(vrf.RD)
	} else {
		data.RD = types.StringNull()
	}

	if vrf.Tenant != nil {
		data.Tenant = types.Int64Value(int64(vrf.Tenant.ID))
	} else {
		data.Tenant = types.Int64Null()
	}

	if vrf.Description != "" {
		data.Description = types.StringValue(vrf.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
