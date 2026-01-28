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

var _ datasource.DataSource = &TenantDataSource{}

func NewTenantDataSource() datasource.DataSource {
	return &TenantDataSource{}
}

type TenantDataSource struct {
	client *client.Client
}

type TenantDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	Description types.String `tfsdk:"description"`
	Comments    types.String `tfsdk:"comments"`
}

func (d *TenantDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant"
}

func (d *TenantDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a tenant from NetBox by name or slug.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the tenant.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the tenant to look up.",
				Optional:    true,
			},
			"slug": schema.StringAttribute{
				Description: "The slug of the tenant to look up.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the tenant.",
				Computed:    true,
			},
			"comments": schema.StringAttribute{
				Description: "Comments about the tenant.",
				Computed:    true,
			},
		},
	}
}

func (d *TenantDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TenantDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TenantDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build query parameters
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
			"Either name or slug must be provided to look up a tenant.",
		)
		return
	}

	// Query NetBox API
	results, err := d.client.GetList(ctx, "/api/tenancy/tenants/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read tenant: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No tenant found matching the criteria: %s", params.Encode()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d tenants matching the criteria. Please be more specific.", len(results)),
		)
		return
	}

	// Map API response to data source model
	var tenant TenantAPIModel
	if err := json.Unmarshal(results[0], &tenant); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse tenant response: %s", err))
		return
	}

	data.ID = types.Int64Value(int64(int64(tenant.ID)))
	data.Name = types.StringValue(tenant.Name)
	data.Slug = types.StringValue(tenant.Slug)

	if tenant.Description != "" {
		data.Description = types.StringValue(tenant.Description)
	} else {
		data.Description = types.StringNull()
	}

	if tenant.Comments != "" {
		data.Comments = types.StringValue(tenant.Comments)
	} else {
		data.Comments = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
