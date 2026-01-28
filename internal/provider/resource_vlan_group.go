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
var _ resource.Resource = &VLANGroupResource{}
var _ resource.ResourceWithImportState = &VLANGroupResource{}

func NewVLANGroupResource() resource.Resource {
	return &VLANGroupResource{}
}

// VLANGroupResource defines the resource implementation.
type VLANGroupResource struct {
	client *client.Client
}

// VLANGroupResourceModel describes the resource data model.
type VLANGroupResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	MinVID      types.Int64  `tfsdk:"min_vid"`
	MaxVID      types.Int64  `tfsdk:"max_vid"`
	Description types.String `tfsdk:"description"`
	Tags        []TagRef     `tfsdk:"tags"`
	Upsert      types.Bool   `tfsdk:"upsert"`
}

// VLANGroupAPIModel represents the NetBox API response for a VLAN group
type VLANGroupAPIModel struct {
	ID          int         `json:"id"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	MinVID      int         `json:"min_vid,omitempty"`
	MaxVID      int         `json:"max_vid,omitempty"`
	Description string      `json:"description,omitempty"`
	Tags        []TagAPIRef `json:"tags,omitempty"`
}

func (r *VLANGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vlan_group"
}

func (r *VLANGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NetBox VLAN group.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NetBox internal ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the VLAN group.",
				Required:    true,
			},
			"slug": schema.StringAttribute{
				Description: "URL-friendly slug for the VLAN group. Auto-generated from name if not provided.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"min_vid": schema.Int64Attribute{
				Description: "Minimum VLAN ID in the group.",
				Optional:    true,
			},
			"max_vid": schema.Int64Attribute{
				Description: "Maximum VLAN ID in the group.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the VLAN group.",
				Optional:    true,
			},
			"upsert": schema.BoolAttribute{
				Description: "If true, will find and use existing VLAN group with matching name instead of creating a new one.",
				Optional:    true,
			},
			"tags": schema.ListNestedAttribute{
				Description: "Tags associated with this VLAN group.",
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

func (r *VLANGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VLANGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VLANGroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if we should search for existing VLAN group
	if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
		// Search for existing VLAN group by name
		params := url.Values{}
		params.Add("name", data.Name.ValueString())

		results, err := r.client.GetList(ctx, "/api/ipam/vlan-groups/", params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for VLAN group, got error: %s", err))
			return
		}

		if len(results) > 0 {
			// Found existing VLAN group - update it to match desired state
			var existing VLANGroupAPIModel
			if err := json.Unmarshal(results[0], &existing); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN group response: %s", err))
				return
			}

			data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))

			// Update the existing VLAN group with desired configuration
			updateData := VLANGroupAPIModel{
				Name: data.Name.ValueString(),
			}

			if !data.Slug.IsNull() {
				updateData.Slug = data.Slug.ValueString()
			} else {
				updateData.Slug = existing.Slug
			}
			if !data.MinVID.IsNull() {
				updateData.MinVID = int(data.MinVID.ValueInt64())
			}
			if !data.MaxVID.IsNull() {
				updateData.MaxVID = int(data.MaxVID.ValueInt64())
			}
			if !data.Description.IsNull() {
				updateData.Description = data.Description.ValueString()
			}
			if len(data.Tags) > 0 {
				updateData.Tags = ConvertTagsToAPI(data.Tags)
			}

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/vlan-groups/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing VLAN group, got error: %s", err))
				return
			}

			var updated VLANGroupAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN group response: %s", err))
				return
			}

			data.Slug = types.StringValue(updated.Slug)
			if updated.MinVID > 0 {
				data.MinVID = types.Int64Value(int64(updated.MinVID))
			}
			if updated.MaxVID > 0 {
				data.MaxVID = types.Int64Value(int64(updated.MaxVID))
			}

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// Create new VLAN group
	createData := VLANGroupAPIModel{
		Name: data.Name.ValueString(),
	}

	if !data.Slug.IsNull() {
		createData.Slug = data.Slug.ValueString()
	}
	if !data.MinVID.IsNull() {
		createData.MinVID = int(data.MinVID.ValueInt64())
	}
	if !data.MaxVID.IsNull() {
		createData.MaxVID = int(data.MaxVID.ValueInt64())
	}
	if !data.Description.IsNull() {
		createData.Description = data.Description.ValueString()
	}
	if len(data.Tags) > 0 {
		createData.Tags = ConvertTagsToAPI(data.Tags)
	}

	apiResp, err := r.client.Create(ctx, "/api/ipam/vlan-groups/", createData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create VLAN group, got error: %s", err))
		return
	}

	var created VLANGroupAPIModel
	if err := json.Unmarshal(apiResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN group response: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
	data.Slug = types.StringValue(created.Slug)
	if created.MinVID > 0 {
		data.MinVID = types.Int64Value(int64(created.MinVID))
	}
	if created.MaxVID > 0 {
		data.MaxVID = types.Int64Value(int64(created.MaxVID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VLANGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VLANGroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/vlan-groups/%s/", data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read VLAN group, got error: %s", err))
		return
	}

	var vlanGroup VLANGroupAPIModel
	if err := json.Unmarshal(apiResp.Body, &vlanGroup); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN group response: %s", err))
		return
	}

	data.Name = types.StringValue(vlanGroup.Name)
	data.Slug = types.StringValue(vlanGroup.Slug)
	if vlanGroup.MinVID > 0 {
		data.MinVID = types.Int64Value(int64(vlanGroup.MinVID))
	}
	if vlanGroup.MaxVID > 0 {
		data.MaxVID = types.Int64Value(int64(vlanGroup.MaxVID))
	}
	data.Description = types.StringValue(vlanGroup.Description)
	data.Tags = ConvertTagsFromAPI(vlanGroup.Tags)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VLANGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VLANGroupResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateData := VLANGroupAPIModel{
		Name: data.Name.ValueString(),
	}

	if !data.Slug.IsNull() {
		updateData.Slug = data.Slug.ValueString()
	}
	if !data.MinVID.IsNull() {
		updateData.MinVID = int(data.MinVID.ValueInt64())
	}
	if !data.MaxVID.IsNull() {
		updateData.MaxVID = int(data.MaxVID.ValueInt64())
	}
	if !data.Description.IsNull() {
		updateData.Description = data.Description.ValueString()
	}
	if len(data.Tags) > 0 {
		updateData.Tags = ConvertTagsToAPI(data.Tags)
	}

	apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/vlan-groups/%s/", data.ID.ValueString()), updateData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update VLAN group, got error: %s", err))
		return
	}

	var updated VLANGroupAPIModel
	if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN group response: %s", err))
		return
	}

	data.Slug = types.StringValue(updated.Slug)
	if updated.MinVID > 0 {
		data.MinVID = types.Int64Value(int64(updated.MinVID))
	}
	if updated.MaxVID > 0 {
		data.MaxVID = types.Int64Value(int64(updated.MaxVID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VLANGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VLANGroupResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, fmt.Sprintf("/api/ipam/vlan-groups/%s/", data.ID.ValueString()))
	if err != nil {
		// Treat 404 as success since resource is already gone
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete VLAN group, got error: %s", err))
		return
	}
}

func (r *VLANGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
