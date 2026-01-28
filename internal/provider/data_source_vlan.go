package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/poroping/terraform-provider-netbox/internal/client"
)

var _ datasource.DataSource = &VLANDataSource{}

func NewVLANDataSource() datasource.DataSource {
	return &VLANDataSource{}
}

type VLANDataSource struct {
	client *client.Client
}

type VLANDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	VID         types.Int64  `tfsdk:"vid"`
	Name        types.String `tfsdk:"name"`
	Status      types.String `tfsdk:"status"`
	Group       types.Int64  `tfsdk:"group"`
	Tenant      types.Int64  `tfsdk:"tenant"`
	Description types.String `tfsdk:"description"`
}

func (d *VLANDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vlan"
}

func (d *VLANDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a VLAN from NetBox by VID and optionally name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the VLAN.",
				Computed:    true,
			},
			"vid": schema.Int64Attribute{
				Description: "The VLAN ID to look up.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the VLAN to look up (optional for disambiguation).",
				Optional:    true,
			},
			"status": schema.StringAttribute{
				Description: "The status of the VLAN.",
				Computed:    true,
			},
			"group": schema.Int64Attribute{
				Description: "The VLAN group ID.",
				Computed:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "The tenant ID associated with this VLAN.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the VLAN.",
				Computed:    true,
			},
		},
	}
}

func (d *VLANDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VLANDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VLANDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("vid", strconv.FormatInt(data.VID.ValueInt64(), 10))
	if !data.Name.IsNull() {
		params.Set("name", data.Name.ValueString())
	}

	results, err := d.client.GetList(ctx, "/api/ipam/vlans/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read VLAN: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No VLAN found with VID: %d", data.VID.ValueInt64()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d VLANs matching the criteria. Please specify name to disambiguate.", len(results)),
		)
		return
	}

	var vlan VLANAPIModel
	if err := json.Unmarshal(results[0], &vlan); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN response: %s", err))
		return
	}

	data.ID = types.Int64Value(int64(vlan.ID))
	data.VID = types.Int64Value(int64(vlan.VID))
	data.Name = types.StringValue(vlan.Name)

	if vlan.Group != nil {
		data.Group = types.Int64Value(int64(vlan.Group.ID))
	} else {
		data.Group = types.Int64Null()
	}

	if vlan.Tenant != nil {
		data.Tenant = types.Int64Value(int64(vlan.Tenant.ID))
	} else {
		data.Tenant = types.Int64Null()
	}

	if vlan.Description != "" {
		data.Description = types.StringValue(vlan.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
