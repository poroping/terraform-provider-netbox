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

var _ datasource.DataSource = &PrefixDataSource{}

func NewPrefixDataSource() datasource.DataSource {
	return &PrefixDataSource{}
}

type PrefixDataSource struct {
	client *client.Client
}

type PrefixDataSourceModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Prefix       types.String `tfsdk:"prefix"`
	IsPool       types.Bool   `tfsdk:"is_pool"`
	MarkUtilized types.Bool   `tfsdk:"mark_utilized"`
	VRF          types.Int64  `tfsdk:"vrf"`
	Tenant       types.Int64  `tfsdk:"tenant"`
	Description  types.String `tfsdk:"description"`
}

func (d *PrefixDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prefix"
}

func (d *PrefixDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a prefix from NetBox by CIDR.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the prefix.",
				Computed:    true,
			},
			"prefix": schema.StringAttribute{
				Description: "The CIDR notation of the prefix to look up (e.g., 10.0.0.0/24).",
				Required:    true,
			},
			"is_pool": schema.BoolAttribute{
				Description: "Whether this prefix is used as an allocation pool.",
				Computed:    true,
			},
			"mark_utilized": schema.BoolAttribute{
				Description: "Whether to mark this prefix as fully utilized.",
				Computed:    true,
			},
			"vrf": schema.Int64Attribute{
				Description: "The VRF ID associated with the prefix.",
				Computed:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "The tenant ID associated with the prefix.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the prefix.",
				Computed:    true,
			},
		},
	}
}

func (d *PrefixDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *PrefixDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PrefixDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Query NetBox API by prefix
	params := url.Values{}
	params.Set("prefix", data.Prefix.ValueString())

	results, err := d.client.GetList(ctx, "/api/ipam/prefixes/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read prefix: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No prefix found with CIDR: %s", data.Prefix.ValueString()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d prefixes with CIDR %s. This should not happen.", len(results), data.Prefix.ValueString()),
		)
		return
	}

	// Map API response to data source model
	var prefix PrefixAPIModel
	if err := json.Unmarshal(results[0], &prefix); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse prefix response: %s", err))
		return
	}
	data.ID = types.Int64Value(int64(prefix.ID))
	data.Prefix = types.StringValue(prefix.Prefix)

	// IsPool and MarkUtilized are not in our PrefixAPIModel, set to null
	data.IsPool = types.BoolNull()
	data.MarkUtilized = types.BoolNull()

	if prefix.VRF != nil {
		data.VRF = types.Int64Value(int64(prefix.VRF.ID))
	} else {
		data.VRF = types.Int64Null()
	}

	if prefix.Tenant != nil {
		data.Tenant = types.Int64Value(int64(prefix.Tenant.ID))
	} else {
		data.Tenant = types.Int64Null()
	}

	if prefix.Description != "" {
		data.Description = types.StringValue(prefix.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
