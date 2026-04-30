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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/poroping/terraform-provider-netbox/internal/client"
	"github.com/poroping/terraform-provider-netbox/internal/planmodifiers"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &VLANResource{}
var _ resource.ResourceWithImportState = &VLANResource{}

func NewVLANResource() resource.Resource {
	return &VLANResource{}
}

// VLANResource defines the resource implementation.
type VLANResource struct {
	client *client.Client
}

// VLANResourceModel describes the resource data model.
type VLANResourceModel struct {
	ID          types.String `tfsdk:"id"`
	VID         types.Int64  `tfsdk:"vid"`
	Name        types.String `tfsdk:"name"`
	Status      types.String `tfsdk:"status"`
	Group       types.Int64  `tfsdk:"group"`
	Tenant      types.Int64  `tfsdk:"tenant"`
	Description types.String `tfsdk:"description"`
	Comments    types.String `tfsdk:"comments"`
	Tags        []TagRef     `tfsdk:"tags"`
	Upsert      types.Bool   `tfsdk:"upsert"`
	Autoassign  types.Bool   `tfsdk:"autoassign"`
}

// VLANAPIModel represents the NetBox API response for a VLAN
type VLANAPIModel struct {
	ID          int               `json:"id"`
	VID         int               `json:"vid"`
	Name        string            `json:"name"`
	Group       *TenantIDOrObject `json:"group,omitempty"`
	Tenant      *TenantIDOrObject `json:"tenant,omitempty"`
	Description string            `json:"description,omitempty"`
	Comments    string            `json:"comments,omitempty"`
	Tags        []TagAPIRef       `json:"tags,omitempty"`
}

func (r *VLANResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vlan"
}

func (r *VLANResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NetBox VLAN.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NetBox internal ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vid": schema.Int64Attribute{
				Description: "VLAN ID (1-4094). If autoassign is true and this is not set, will be automatically assigned from the group's available range.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the VLAN.",
				Required:    true,
			},
			"status": schema.StringAttribute{
				Description: "Status of the VLAN (active, reserved, deprecated).",
				Optional:    true,
			},
			"group": schema.Int64Attribute{
				Description: "VLAN group ID. Required when autoassign is true.",
				Optional:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "Tenant ID that owns this VLAN.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the VLAN.",
				Optional:    true,
			},
			"comments": schema.StringAttribute{
				Description: "Additional comments.",
				Optional:    true,
			},
			"upsert": schema.BoolAttribute{
				Description: "If true, will find and use existing VLAN with matching name instead of creating a new one.",
				Optional:    true,
			},
			"autoassign": schema.BoolAttribute{
				Description: "If true, automatically assign an available VID from the VLAN group. Requires group to be set.",
				Optional:    true,
			},
			"tags": schema.ListNestedAttribute{
				Description: "Tags associated with this VLAN.",
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

func (r *VLANResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VLANResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VLANResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if we should search for existing VLAN (upsert without autoassign).
	// Scoped to the group when group is set, otherwise global name search.
	if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
		params := url.Values{}
		params.Add("name", data.Name.ValueString())
		if !data.Group.IsNull() {
			params.Add("group_id", fmt.Sprintf("%d", data.Group.ValueInt64()))
		}

		results, err := r.client.GetList(ctx, "/api/ipam/vlans/", params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for VLAN, got error: %s", err))
			return
		}

		if len(results) > 0 {
			var existing VLANAPIModel
			if err := json.Unmarshal(results[0], &existing); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN response: %s", err))
				return
			}

			data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))
			data.VID = types.Int64Value(int64(existing.VID))

			updateData := VLANAPIModel{
				VID:  existing.VID,
				Name: data.Name.ValueString(),
			}

			if !data.Group.IsNull() {
				updateData.Group = &TenantIDOrObject{ID: int(data.Group.ValueInt64())}
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

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/vlans/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing VLAN, got error: %s", err))
				return
			}

			var updated VLANAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN response: %s", err))
				return
			}

			data.VID = types.Int64Value(int64(updated.VID))
			if updated.Group != nil {
				data.Group = types.Int64Value(int64(updated.Group.ID))
			}
			if updated.Tenant != nil {
				data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
			}

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// Handle autoassign: get available VID from VLAN group
	if !data.Autoassign.IsNull() && data.Autoassign.ValueBool() {
		// Validate that group is set
		if data.Group.IsNull() {
			resp.Diagnostics.AddError("Configuration Error", "group must be set when autoassign is true")
			return
		}

		// If upsert is true, check if VLAN with this name already exists in the group
		if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
			params := url.Values{}
			params.Add("name", data.Name.ValueString())
			params.Add("group_id", fmt.Sprintf("%d", data.Group.ValueInt64()))

			existingVLANs, err := r.client.GetList(ctx, "/api/ipam/vlans/", params)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to check for existing VLAN, got error: %s", err))
				return
			}

			if len(existingVLANs) > 0 {
				// VLAN already exists - update it to match desired state
				var existing VLANAPIModel
				if err := json.Unmarshal(existingVLANs[0], &existing); err != nil {
					resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse existing VLAN response: %s", err))
					return
				}

				data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))
				data.VID = types.Int64Value(int64(existing.VID))

				// Update the existing VLAN with desired configuration
				updateData := VLANAPIModel{
					VID:  existing.VID,
					Name: data.Name.ValueString(),
				}

				if !data.Group.IsNull() {
					updateData.Group = &TenantIDOrObject{ID: int(data.Group.ValueInt64())}
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

				apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/vlans/%s/", data.ID.ValueString()), updateData)
				if err != nil {
					resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing VLAN, got error: %s", err))
					return
				}

				var updated VLANAPIModel
				if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
					resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN response: %s", err))
					return
				}

				if updated.Group != nil {
					data.Group = types.Int64Value(int64(updated.Group.ID))
				}
				if updated.Tenant != nil {
					data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}
		}

		// Get available VID from group
		apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/vlan-groups/%d/available-vlans/", data.Group.ValueInt64()))
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get available VIDs from group, got error: %s", err))
			return
		}

		var availableVLANs []struct {
			VID int `json:"vid"`
		}
		if err := json.Unmarshal(apiResp.Body, &availableVLANs); err != nil {
			resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse available VLANs response: %s", err))
			return
		}

		if len(availableVLANs) == 0 {
			resp.Diagnostics.AddError("Resource Exhaustion", "No available VIDs in the VLAN group")
			return
		}

		// Use the first available VID
		data.VID = types.Int64Value(int64(availableVLANs[0].VID))
	}

	// Validate VID is set
	if data.VID.IsNull() {
		resp.Diagnostics.AddError("Configuration Error", "vid must be set or autoassign must be enabled")
		return
	}

	// Create new VLAN
	createData := VLANAPIModel{
		VID:  int(data.VID.ValueInt64()),
		Name: data.Name.ValueString(),
	}

	if !data.Group.IsNull() {
		createData.Group = &TenantIDOrObject{ID: int(data.Group.ValueInt64())}
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

	apiResp, err := r.client.Create(ctx, "/api/ipam/vlans/", createData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create VLAN, got error: %s", err))
		return
	}

	var created VLANAPIModel
	if err := json.Unmarshal(apiResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN response: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
	data.VID = types.Int64Value(int64(created.VID))
	if created.Group != nil {
		data.Group = types.Int64Value(int64(created.Group.ID))
	}
	if created.Tenant != nil {
		data.Tenant = types.Int64Value(int64(created.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VLANResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VLANResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/vlans/%s/", data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read VLAN, got error: %s", err))
		return
	}

	var vlan VLANAPIModel
	if err := json.Unmarshal(apiResp.Body, &vlan); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN response: %s", err))
		return
	}

	data.VID = types.Int64Value(int64(vlan.VID))
	data.Name = types.StringValue(vlan.Name)
	if vlan.Group != nil {
		data.Group = types.Int64Value(int64(vlan.Group.ID))
	}
	if vlan.Tenant != nil {
		data.Tenant = types.Int64Value(int64(vlan.Tenant.ID))
	}
	if vlan.Description == "" {
		data.Description = types.StringNull()
	} else {
		data.Description = types.StringValue(vlan.Description)
	}

	if vlan.Comments == "" {
		data.Comments = types.StringNull()
	} else {
		data.Comments = types.StringValue(vlan.Comments)
	}
	data.Tags = ConvertTagsFromAPI(vlan.Tags)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VLANResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VLANResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateData := VLANAPIModel{
		VID:  int(data.VID.ValueInt64()),
		Name: data.Name.ValueString(),
	}

	if !data.Group.IsNull() {
		updateData.Group = &TenantIDOrObject{ID: int(data.Group.ValueInt64())}
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

	apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/vlans/%s/", data.ID.ValueString()), updateData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update VLAN, got error: %s", err))
		return
	}

	var updated VLANAPIModel
	if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VLAN response: %s", err))
		return
	}

	data.VID = types.Int64Value(int64(updated.VID))
	if updated.Group != nil {
		data.Group = types.Int64Value(int64(updated.Group.ID))
	}
	if updated.Tenant != nil {
		data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VLANResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VLANResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Try to deprecate before deletion (some NetBox instances require this)
	deprecateData := map[string]interface{}{"status": "deprecated"}
	_, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/vlans/%s/", data.ID.ValueString()), deprecateData)
	if err != nil {
		// Log the warning but continue with deletion
		resp.Diagnostics.AddWarning(
			"Deprecation Warning",
			fmt.Sprintf("Unable to set status to deprecated before deletion: %s. Continuing with deletion.", err),
		)
	}

	_, err = r.client.Delete(ctx, fmt.Sprintf("/api/ipam/vlans/%s/", data.ID.ValueString()))
	if err != nil {
		// Treat 404 as success since resource is already gone
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete VLAN, got error: %s", err))
		return
	}
}

func (r *VLANResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
