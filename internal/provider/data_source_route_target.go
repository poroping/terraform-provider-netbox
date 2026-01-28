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

var _ datasource.DataSource = &RouteTargetDataSource{}

func NewRouteTargetDataSource() datasource.DataSource {
	return &RouteTargetDataSource{}
}

type RouteTargetDataSource struct {
	client *client.Client
}

type RouteTargetDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Tenant      types.Int64  `tfsdk:"tenant"`
	Description types.String `tfsdk:"description"`
}

func (d *RouteTargetDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_route_target"
}

func (d *RouteTargetDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a route target from NetBox by name.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the route target.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the route target to look up.",
				Required:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "The tenant ID associated with this route target.",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the route target.",
				Computed:    true,
			},
		},
	}
}

func (d *RouteTargetDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RouteTargetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RouteTargetDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := url.Values{}
	params.Set("name", data.Name.ValueString())

	results, err := d.client.GetList(ctx, "/api/ipam/route-targets/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read route target: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No route target found with name: %s", data.Name.ValueString()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d route targets matching the criteria. Please be more specific.", len(results)),
		)
		return
	}

	var routeTarget RouteTargetAPIModel
	if err := json.Unmarshal(results[0], &routeTarget); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse route target response: %s", err))
		return
	}

	data.ID = types.Int64Value(int64(routeTarget.ID))
	data.Name = types.StringValue(routeTarget.Name)

	if routeTarget.Tenant != nil {
		data.Tenant = types.Int64Value(int64(routeTarget.Tenant.ID))
	} else {
		data.Tenant = types.Int64Null()
	}

	if routeTarget.Description != "" {
		data.Description = types.StringValue(routeTarget.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
