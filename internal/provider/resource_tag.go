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
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TagResource{}
var _ resource.ResourceWithImportState = &TagResource{}

func NewTagResource() resource.Resource {
	return &TagResource{}
}

// TagResource defines the resource implementation.
type TagResource struct {
	client *client.Client
}

// TagResourceModel describes the resource data model.
type TagResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	Color       types.String `tfsdk:"color"`
	Description types.String `tfsdk:"description"`
	Upsert      types.Bool   `tfsdk:"upsert"`
}

// TagAPIModel represents the NetBox API response for a tag
type TagAPIModel struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Color       string `json:"color,omitempty"`
	Description string `json:"description,omitempty"`
}

func (r *TagResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag"
}

func (r *TagResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NetBox tag.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NetBox internal ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the tag.",
				Required:    true,
			},
			"slug": schema.StringAttribute{
				Description: "URL-friendly slug for the tag. Auto-generated from name if not provided.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"color": schema.StringAttribute{
				Description: "Color for the tag in hex format (e.g., '9e9e9e').",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the tag.",
				Optional:    true,
			},
			"upsert": schema.BoolAttribute{
				Description: "If true, will find and use existing tag with matching slug instead of creating a new one.",
				Optional:    true,
			},
		},
	}
}

func (r *TagResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TagResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if we should search for existing tag
	if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
		// Search for existing tag by slug (unique identifier)
		params := url.Values{}
		if !data.Slug.IsNull() {
			params.Add("slug", data.Slug.ValueString())
		} else {
			// If no slug provided, search by name (will be slugified by NetBox)
			params.Add("name", data.Name.ValueString())
		}

		results, err := r.client.GetList(ctx, "/api/extras/tags/", params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for tag, got error: %s", err))
			return
		}

		if len(results) > 0 {
			// Found existing tag - update it to match desired state
			var existing TagAPIModel
			if err := json.Unmarshal(results[0], &existing); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse tag response: %s", err))
				return
			}

			data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))

			// Update the existing tag with desired configuration
			updateData := TagAPIModel{
				Name: data.Name.ValueString(),
			}

			if !data.Slug.IsNull() {
				updateData.Slug = data.Slug.ValueString()
			} else {
				updateData.Slug = existing.Slug
			}
			if !data.Color.IsNull() {
				updateData.Color = data.Color.ValueString()
			}
			if !data.Description.IsNull() {
				updateData.Description = data.Description.ValueString()
			}

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/extras/tags/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing tag, got error: %s", err))
				return
			}

			var updated TagAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse tag response: %s", err))
				return
			}

			data.Slug = types.StringValue(updated.Slug)

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// Create new tag
	createData := TagAPIModel{
		Name: data.Name.ValueString(),
	}

	if !data.Slug.IsNull() {
		createData.Slug = data.Slug.ValueString()
	}
	if !data.Color.IsNull() {
		createData.Color = data.Color.ValueString()
	}
	if !data.Description.IsNull() {
		createData.Description = data.Description.ValueString()
	}

	apiResp, err := r.client.Create(ctx, "/api/extras/tags/", createData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create tag, got error: %s", err))
		return
	}

	var created TagAPIModel
	if err := json.Unmarshal(apiResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse tag response: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
	data.Slug = types.StringValue(created.Slug)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TagResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/extras/tags/%s/", data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read tag, got error: %s", err))
		return
	}

	var tag TagAPIModel
	if err := json.Unmarshal(apiResp.Body, &tag); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse tag response: %s", err))
		return
	}

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

func (r *TagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TagResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateData := TagAPIModel{
		Name: data.Name.ValueString(),
	}

	if !data.Slug.IsNull() {
		updateData.Slug = data.Slug.ValueString()
	}
	if !data.Color.IsNull() {
		updateData.Color = data.Color.ValueString()
	}
	if !data.Description.IsNull() {
		updateData.Description = data.Description.ValueString()
	}

	apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/extras/tags/%s/", data.ID.ValueString()), updateData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update tag, got error: %s", err))
		return
	}

	var updated TagAPIModel
	if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse tag response: %s", err))
		return
	}

	data.Slug = types.StringValue(updated.Slug)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TagResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, fmt.Sprintf("/api/extras/tags/%s/", data.ID.ValueString()))
	if err != nil {
		// Treat 404 as success since resource is already gone
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete tag, got error: %s", err))
		return
	}
}

func (r *TagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
