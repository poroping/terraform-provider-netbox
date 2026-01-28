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
var _ resource.Resource = &RIRResource{}
var _ resource.ResourceWithImportState = &RIRResource{}

func NewRIRResource() resource.Resource {
	return &RIRResource{}
}

// RIRResource defines the resource implementation.
type RIRResource struct {
	client *client.Client
}

// RIRResourceModel describes the resource data model.
type RIRResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	IsPrivate   types.Bool   `tfsdk:"is_private"`
	Description types.String `tfsdk:"description"`
	Tags        []TagRef     `tfsdk:"tags"`
	Upsert      types.Bool   `tfsdk:"upsert"`
}

// RIRAPIModel represents the NetBox API response for an RIR
type RIRAPIModel struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	IsPrivate   bool        `json:"is_private"`
	Description string      `json:"description,omitempty"`
	Tags        []TagAPIRef `json:"tags,omitempty"`
}

func (r *RIRResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rir"
}

func (r *RIRResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NetBox RIR (Regional Internet Registry).",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NetBox internal ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the RIR.",
				Required:    true,
			},
			"slug": schema.StringAttribute{
				Description: "URL-friendly slug for the RIR. Auto-generated from name if not provided.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"is_private": schema.BoolAttribute{
				Description: "Whether this RIR represents private address space.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the RIR.",
				Optional:    true,
			},
			"upsert": schema.BoolAttribute{
				Description: "If true, will find and use existing RIR with matching name instead of creating a new one.",
				Optional:    true,
			},
			"tags": schema.ListNestedAttribute{
				Description: "Tags associated with this RIR.",
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

func (r *RIRResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RIRResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RIRResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if we should search for existing RIR
	if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
		// Search for existing RIR by name
		params := url.Values{}
		params.Add("name", data.Name.ValueString())

		results, err := r.client.GetList(ctx, "/api/ipam/rirs/", params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for RIR, got error: %s", err))
			return
		}

		if len(results) > 0 {
			// Found existing RIR - update it to match desired state
			var existing RIRAPIModel
			if err := json.Unmarshal(results[0], &existing); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse RIR response: %s", err))
				return
			}

			data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))

			// Update the existing RIR with desired configuration
			updateData := RIRAPIModel{
				Name: data.Name.ValueString(),
			}

			if !data.Slug.IsNull() {
				updateData.Slug = data.Slug.ValueString()
			} else {
				updateData.Slug = existing.Slug
			}
			if !data.IsPrivate.IsNull() {
				updateData.IsPrivate = data.IsPrivate.ValueBool()
			}
			if !data.Description.IsNull() {
				updateData.Description = data.Description.ValueString()
			}
			if len(data.Tags) > 0 {
				updateData.Tags = ConvertTagsToAPI(data.Tags)
			}

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/rirs/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing RIR, got error: %s", err))
				return
			}

			var updated RIRAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse RIR response: %s", err))
				return
			}

			data.Slug = types.StringValue(updated.Slug)

			// Only set is_private if it was explicitly set in config
			if !data.IsPrivate.IsNull() {
				data.IsPrivate = types.BoolValue(updated.IsPrivate)
			}

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// Create new RIR
	createData := RIRAPIModel{
		Name: data.Name.ValueString(),
	}

	if !data.Slug.IsNull() {
		createData.Slug = data.Slug.ValueString()
	}
	if !data.IsPrivate.IsNull() {
		createData.IsPrivate = data.IsPrivate.ValueBool()
	}
	if !data.Description.IsNull() {
		createData.Description = data.Description.ValueString()
	}
	if len(data.Tags) > 0 {
		createData.Tags = ConvertTagsToAPI(data.Tags)
	}

	apiResp, err := r.client.Create(ctx, "/api/ipam/rirs/", createData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create RIR, got error: %s", err))
		return
	}

	var created RIRAPIModel
	if err := json.Unmarshal(apiResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse RIR response: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
	data.Slug = types.StringValue(created.Slug)

	// Only set is_private if it was explicitly set in config
	if !data.IsPrivate.IsNull() {
		data.IsPrivate = types.BoolValue(created.IsPrivate)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RIRResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RIRResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/rirs/%s/", data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read RIR, got error: %s", err))
		return
	}

	var rir RIRAPIModel
	if err := json.Unmarshal(apiResp.Body, &rir); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse RIR response: %s", err))
		return
	}

	data.Name = types.StringValue(rir.Name)
	data.Slug = types.StringValue(rir.Slug)

	// Only set is_private if it was originally set in the config
	if !data.IsPrivate.IsNull() {
		data.IsPrivate = types.BoolValue(rir.IsPrivate)
	}

	if rir.Description == "" {
		data.Description = types.StringNull()
	} else {
		data.Description = types.StringValue(rir.Description)
	}
	data.Tags = ConvertTagsFromAPI(rir.Tags)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RIRResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RIRResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateData := RIRAPIModel{
		Name: data.Name.ValueString(),
	}

	if !data.Slug.IsNull() {
		updateData.Slug = data.Slug.ValueString()
	}
	if !data.IsPrivate.IsNull() {
		updateData.IsPrivate = data.IsPrivate.ValueBool()
	}
	if !data.Description.IsNull() {
		updateData.Description = data.Description.ValueString()
	}
	if len(data.Tags) > 0 {
		updateData.Tags = ConvertTagsToAPI(data.Tags)
	}

	apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/rirs/%s/", data.ID.ValueString()), updateData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update RIR, got error: %s", err))
		return
	}

	var updated RIRAPIModel
	if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse RIR response: %s", err))
		return
	}

	data.Slug = types.StringValue(updated.Slug)
	data.IsPrivate = types.BoolValue(updated.IsPrivate)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RIRResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RIRResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, fmt.Sprintf("/api/ipam/rirs/%s/", data.ID.ValueString()))
	if err != nil {
		// Treat 404 as success since resource is already gone
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete RIR, got error: %s", err))
		return
	}
}

func (r *RIRResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
