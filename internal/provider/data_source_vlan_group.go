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

var _ datasource.DataSource = &VLANGroupDataSource{}

func NewVLANGroupDataSource() datasource.DataSource {
	return &VLANGroupDataSource{}
}

type VLANGroupDataSource struct {
	client *client.Client
}

type VLANGroupDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	Description types.String `tfsdk:"description"`
}

func (d *VLANGroupDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vlan_group"
}

func (d *VLANGroupDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a VLAN group from NetBox by name or slug.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the VLAN group.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the VLAN group to look up.",
				Optional:    true,
			},
			"slug": schema.StringAttribute{
				Description: "The slug of the VLAN group to look up.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the VLAN group.",
				Computed:    true,
			},
		},
	}
}

func (d *VLANGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VLANGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VLANGroupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	if !data.Name.IsNull() {
		params.Set("name", data.Name.ValueString())
	}
	if !data.Slug.IsNull() {
		params.Set("slug", data.Slug.ValueString())
	}

	if len(params) == 0 {
		resp.Diagnostics.AddError(
			"Missing Search Criteria",
			"Either name or slug must be provided to look up a VLAN group.",
		)
		return
	}

	results, err := d.client.GetList(ctx, "/api/ipam/vlan-groups/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read VLAN group: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No VLAN group found matching the criteria: %s", params.Encode()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d VLAN groups matching the criteria. Please be more specific.", len(results)),
		)
		return
	}

	var vlanGroup VLANGroupAPIModel
	if err := json.Unmarshal(results[0], &vlanGroup); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN group response: %s", err))
		return
	}

	data.ID = types.Int64Value(int64(vlanGroup.ID))
	data.Name = types.StringValue(vlanGroup.Name)
	data.Slug = types.StringValue(vlanGroup.Slug)

	if vlanGroup.Description != "" {
		data.Description = types.StringValue(vlanGroup.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
