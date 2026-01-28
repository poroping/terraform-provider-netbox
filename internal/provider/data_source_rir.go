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

var _ datasource.DataSource = &RIRDataSource{}

func NewRIRDataSource() datasource.DataSource {
	return &RIRDataSource{}
}

type RIRDataSource struct {
	client *client.Client
}

type RIRDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	Description types.String `tfsdk:"description"`
}

func (d *RIRDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rir"
}

func (d *RIRDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Regional Internet Registry (RIR) from NetBox by name or slug.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the RIR.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the RIR to look up.",
				Optional:    true,
			},
			"slug": schema.StringAttribute{
				Description: "The slug of the RIR to look up.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the RIR.",
				Computed:    true,
			},
		},
	}
}

func (d *RIRDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RIRDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RIRDataSourceModel
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
			"Either name or slug must be provided to look up a RIR.",
		)
		return
	}

	results, err := d.client.GetList(ctx, "/api/ipam/rirs/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read RIR: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No RIR found matching the criteria: %s", params.Encode()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d RIRs matching the criteria. Please be more specific.", len(results)),
		)
		return
	}

	var rir RIRAPIModel
	if err := json.Unmarshal(results[0], &rir); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse RIR response: %s", err))
		return
	}

	data.ID = types.Int64Value(int64(rir.ID))
	data.Name = types.StringValue(rir.Name)
	data.Slug = types.StringValue(rir.Slug)

	if rir.Description != "" {
		data.Description = types.StringValue(rir.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
