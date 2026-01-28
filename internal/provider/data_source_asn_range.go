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

var _ datasource.DataSource = &ASNRangeDataSource{}

func NewASNRangeDataSource() datasource.DataSource {
	return &ASNRangeDataSource{}
}

type ASNRangeDataSource struct {
	client *client.Client
}

type ASNRangeDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	Start       types.Int64  `tfsdk:"start"`
	End         types.Int64  `tfsdk:"end"`
	RIR         types.Int64  `tfsdk:"rir"`
	Tenant      types.Int64  `tfsdk:"tenant"`
	Description types.String `tfsdk:"description"`
}

func (d *ASNRangeDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asn_range"
}

func (d *ASNRangeDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches an ASN range from NetBox by name or slug.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the ASN range.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the ASN range to look up.",
				Optional:    true,
			},
			"slug": schema.StringAttribute{
				Description: "The slug of the ASN range to look up.",
				Optional:    true,
			},
			"start": schema.Int64Attribute{
				Description: "The starting ASN of the range.",
				Computed:    true,
			},
			"end": schema.Int64Attribute{
				Description: "The ending ASN of the range.",
				Computed:    true,
			},
			"rir": schema.Int64Attribute{
				Description: "The RIR ID associated with this ASN range.",
				Computed:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "The tenant ID associated with this ASN range.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the ASN range.",
				Computed:    true,
			},
		},
	}
}

func (d *ASNRangeDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ASNRangeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ASNRangeDataSourceModel
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
			"Either name or slug must be provided to look up an ASN range.",
		)
		return
	}

	results, err := d.client.GetList(ctx, "/api/ipam/asn-ranges/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read ASN range: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No ASN range found matching the criteria: %s", params.Encode()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d ASN ranges matching the criteria. Please be more specific.", len(results)),
		)
		return
	}

	var asnRange ASNRangeAPIModel
	if err := json.Unmarshal(results[0], &asnRange); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN range response: %s", err))
		return
	}

	data.ID = types.Int64Value(int64(asnRange.ID))
	data.Name = types.StringValue(asnRange.Name)
	data.Slug = types.StringValue(asnRange.Slug)
	data.Start = types.Int64Value(int64(asnRange.Start))
	data.End = types.Int64Value(int64(asnRange.End))

	if asnRange.RIR != nil {
		data.RIR = types.Int64Value(int64(asnRange.RIR.ID))
	} else {
		data.RIR = types.Int64Null()
	}

	if asnRange.Tenant != nil {
		data.Tenant = types.Int64Value(int64(asnRange.Tenant.ID))
	} else {
		data.Tenant = types.Int64Null()
	}

	if asnRange.Description != "" {
		data.Description = types.StringValue(asnRange.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
