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

var _ datasource.DataSource = &TagDataSource{}

func NewTagDataSource() datasource.DataSource {
	return &TagDataSource{}
}

type TagDataSource struct {
	client *client.Client
}

type TagDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	Color       types.String `tfsdk:"color"`
	Description types.String `tfsdk:"description"`
}

func (d *TagDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag"
}

func (d *TagDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a tag from NetBox by name or slug.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Description: "The ID of the tag.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the tag to look up.",
				Optional:    true,
			},
			"slug": schema.StringAttribute{
				Description: "The slug of the tag to look up.",
				Optional:    true,
			},
			"color": schema.StringAttribute{
				Description: "The color of the tag (hex format).",
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the tag.",
				Computed:    true,
			},
		},
	}
}

func (d *TagDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TagDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TagDataSourceModel
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
			"Either name or slug must be provided to look up a tag.",
		)
		return
	}

	results, err := d.client.GetList(ctx, "/api/extras/tags/", params)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read tag: %s", err))
		return
	}

	if len(results) == 0 {
		resp.Diagnostics.AddError(
			"Not Found",
			fmt.Sprintf("No tag found matching the criteria: %s", params.Encode()),
		)
		return
	}

	if len(results) > 1 {
		resp.Diagnostics.AddError(
			"Multiple Results",
			fmt.Sprintf("Found %d tags matching the criteria. Please be more specific.", len(results)),
		)
		return
	}

	var tag TagAPIModel
	if err := json.Unmarshal(results[0], &tag); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse tag response: %s", err))
		return
	}

	data.ID = types.Int64Value(int64(tag.ID))
	data.Name = types.StringValue(tag.Name)
	data.Slug = types.StringValue(tag.Slug)

	if tag.Color != "" {
		data.Color = types.StringValue(tag.Color)
	} else {
		data.Color = types.StringNull()
	}

	if tag.Description != "" {
		data.Description = types.StringValue(tag.Description)
	} else {
		data.Description = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
