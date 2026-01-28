package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/poroping/terraform-provider-netbox/internal/client"
	"github.com/poroping/terraform-provider-netbox/internal/planmodifiers"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RouteTargetResource{}
var _ resource.ResourceWithImportState = &RouteTargetResource{}

func NewRouteTargetResource() resource.Resource {
	return &RouteTargetResource{}
}

// RouteTargetResource defines the resource implementation.
type RouteTargetResource struct {
	client *client.Client
}

// RouteTargetResourceModel describes the resource data model.
type RouteTargetResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Tenant      types.Int64  `tfsdk:"tenant"`
	Description types.String `tfsdk:"description"`
	Comments    types.String `tfsdk:"comments"`
	Tags        []TagRef     `tfsdk:"tags"`
	Upsert      types.Bool   `tfsdk:"upsert"`
}

// RouteTargetAPIModel represents the NetBox API response for a route target
type RouteTargetAPIModel struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	Tenant      *TenantIDOrObject `json:"tenant,omitempty"`
	Description string            `json:"description,omitempty"`
	Comments    string            `json:"comments,omitempty"`
	Tags        []TagAPIRef       `json:"tags,omitempty"`
}

func (r *RouteTargetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_route_target"
}

func (r *RouteTargetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NetBox route target.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NetBox internal ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Route target name (e.g., '65000:100').",
				Required:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "Tenant ID that owns this route target.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the route target.",
				Optional:    true,
			},
			"comments": schema.StringAttribute{
				Description: "Additional comments.",
				Optional:    true,
			},
			"upsert": schema.BoolAttribute{
				Description: "If true, will find and use existing route target with matching name instead of creating a new one.",
				Optional:    true,
			},
			"tags": schema.ListNestedAttribute{
				Description: "Tags associated with this route target.",
				Optional:    true,
				PlanModifiers: []planmodifier.List{
					planmodifiers.UnorderedList(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "Tag name.",
							Required:    true,
						},
						"slug": schema.StringAttribute{
							Description: "Tag slug.",
							Required:    true,
						},
					},
				},
			},
		},
	}
}

func (r *RouteTargetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	apiClient, ok := req.ProviderData.(*client.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = apiClient
}

func (r *RouteTargetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RouteTargetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if we should search for existing route target
	if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
		// Search for existing route target by name
		params := url.Values{}
		params.Add("name", data.Name.ValueString())

		results, err := r.client.GetList(ctx, "/api/ipam/route-targets/", params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for route target, got error: %s", err))
			return
		}

		if len(results) > 0 {
			// Found existing route target - update it to match desired state
			var existing RouteTargetAPIModel
			if err := json.Unmarshal(results[0], &existing); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse route target response: %s", err))
				return
			}

			data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))

			// Update the existing route target with desired configuration
			updateData := RouteTargetAPIModel{
				Name: data.Name.ValueString(),
			}

			if !data.Tenant.IsNull() {
				updateData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
			}
			if !data.Description.IsNull() {
				updateData.Description = data.Description.ValueString()
			}
			if !data.Comments.IsNull() {
				updateData.Comments = data.Comments.ValueString()
			}
			if len(data.Tags) > 0 {
				updateData.Tags = ConvertTagsToAPI(data.Tags)
			}

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/route-targets/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing route target, got error: %s", err))
				return
			}

			var updated RouteTargetAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse route target response: %s", err))
				return
			}

			if updated.Tenant != nil {
				data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
			}

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// Create new route target
	createData := RouteTargetAPIModel{
		Name: data.Name.ValueString(),
	}

	if !data.Tenant.IsNull() {
		createData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
	}
	if !data.Description.IsNull() {
		createData.Description = data.Description.ValueString()
	}
	if !data.Comments.IsNull() {
		createData.Comments = data.Comments.ValueString()
	}
	if len(data.Tags) > 0 {
		createData.Tags = ConvertTagsToAPI(data.Tags)
	}

	apiResp, err := r.client.Create(ctx, "/api/ipam/route-targets/", createData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create route target, got error: %s", err))
		return
	}

	var created RouteTargetAPIModel
	if err := json.Unmarshal(apiResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse route target response: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
	if created.Tenant != nil {
		data.Tenant = types.Int64Value(int64(created.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RouteTargetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RouteTargetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/route-targets/%s/", data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read route target, got error: %s", err))
		return
	}

	var routeTarget RouteTargetAPIModel
	if err := json.Unmarshal(apiResp.Body, &routeTarget); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse route target response: %s", err))
		return
	}

	data.Name = types.StringValue(routeTarget.Name)
	if routeTarget.Tenant != nil {
		data.Tenant = types.Int64Value(int64(routeTarget.Tenant.ID))
	}

	if routeTarget.Description == "" {
		data.Description = types.StringNull()
	} else {
		data.Description = types.StringValue(routeTarget.Description)
	}

	if routeTarget.Comments == "" {
		data.Comments = types.StringNull()
	} else {
		data.Comments = types.StringValue(routeTarget.Comments)
	}

	data.Tags = ConvertTagsFromAPI(routeTarget.Tags)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RouteTargetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RouteTargetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateData := RouteTargetAPIModel{
		Name: data.Name.ValueString(),
	}

	if !data.Tenant.IsNull() {
		updateData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
	}
	if !data.Description.IsNull() {
		updateData.Description = data.Description.ValueString()
	}
	if !data.Comments.IsNull() {
		updateData.Comments = data.Comments.ValueString()
	}
	if len(data.Tags) > 0 {
		updateData.Tags = ConvertTagsToAPI(data.Tags)
	}

	apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/route-targets/%s/", data.ID.ValueString()), updateData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update route target, got error: %s", err))
		return
	}

	var updated RouteTargetAPIModel
	if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse route target response: %s", err))
		return
	}

	if updated.Tenant != nil {
		data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RouteTargetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RouteTargetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, fmt.Sprintf("/api/ipam/route-targets/%s/", data.ID.ValueString()))
	if err != nil {
		// Treat 404 as success since resource is already gone
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete route target, got error: %s", err))
		return
	}
}

func (r *RouteTargetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
