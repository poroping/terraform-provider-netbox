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

var _ datasource.DataSource = &ASNDataSource{}

func NewASNDataSource() datasource.DataSource {
	return &ASNDataSource{}
}

type ASNDataSource struct {
	client *client.Client
}

type ASNDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	ASN         types.Int64  `tfsdk:"asn"`
	RIR         types.Int64  `tfsdk:"rir"`
	Tenant      types.Int64  `tfsdk:"tenant"`
	Description types.String `tfsdk:"description"`
}

func (d *ASNDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asn"
}

func (d *ASNDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches an Autonomous System Number (ASN) from NetBox.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the ASN.",
				Computed:    true,
			},
			"asn": schema.Int64Attribute{
				Description: "The ASN value to look up.",
				Required:    true,
			},
			"rir": schema.Int64Attribute{
				Description: "The RIR ID associated with this ASN.",
				Computed:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "The tenant ID associated with this ASN.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the ASN.",
				Computed:    true,
			},
		},
	}
}

func (d *ASNDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ASNDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ASNDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("asn", strconv.FormatInt(data.ASN.ValueInt64(), 10))

	results, err := d.client.GetList(ctx, "/api/ipam/asns/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read ASN: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No ASN found with value: %d", data.ASN.ValueInt64()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d ASNs matching the criteria. Please be more specific.", len(results)),
		)
		return
	}

	var asn ASNAPIModel
	if err := json.Unmarshal(results[0], &asn); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN response: %s", err))
		return
	}

	data.ID = types.Int64Value(int64(asn.ID))
	data.ASN = types.Int64Value(int64(asn.ASN))

	if asn.RIR != nil {
		data.RIR = types.Int64Value(int64(asn.RIR.ID))
	} else {
		data.RIR = types.Int64Null()
	}

	if asn.Tenant != nil {
		data.Tenant = types.Int64Value(int64(asn.Tenant.ID))
	} else {
		data.Tenant = types.Int64Null()
	}

	if asn.Description != "" {
		data.Description = types.StringValue(asn.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
