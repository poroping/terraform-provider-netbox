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
var _ resource.Resource = &PrefixResource{}
var _ resource.ResourceWithImportState = &PrefixResource{}

func NewPrefixResource() resource.Resource {
	return &PrefixResource{}
}

// PrefixResource defines the resource implementation.
type PrefixResource struct {
	client *client.Client
}

// PrefixResourceModel describes the resource data model.
type PrefixResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Prefix         types.String `tfsdk:"prefix"`
	VRF            types.Int64  `tfsdk:"vrf"`
	Tenant         types.Int64  `tfsdk:"tenant"`
	VLAN           types.Int64  `tfsdk:"vlan"`
	Description    types.String `tfsdk:"description"`
	Comments       types.String `tfsdk:"comments"`
	Tags           []TagRef     `tfsdk:"tags"`
	Upsert         types.Bool   `tfsdk:"upsert"`
	Autoassign     types.Bool   `tfsdk:"autoassign"`
	ParentPrefixID types.Int64  `tfsdk:"parent_prefix_id"`
	PrefixLength   types.Int64  `tfsdk:"prefix_length"`
}

// PrefixAPIModel represents the NetBox API response for a prefix
type PrefixAPIModel struct {
	ID          int               `json:"id"`
	Prefix      string            `json:"prefix"`
	VRF         *TenantIDOrObject `json:"vrf,omitempty"`
	Tenant      *TenantIDOrObject `json:"tenant,omitempty"`
	VLAN        *TenantIDOrObject `json:"vlan,omitempty"`
	Description string            `json:"description,omitempty"`
	Comments    string            `json:"comments,omitempty"`
	Tags        []TagAPIRef       `json:"tags,omitempty"`
}

func (r *PrefixResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prefix"
}

func (r *PrefixResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NetBox prefix.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NetBox internal ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"prefix": schema.StringAttribute{
				Description: "IP prefix in CIDR notation (e.g., '10.0.0.0/24'). Optional when autoassign is true.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"autoassign": schema.BoolAttribute{
				Description: "If true, automatically allocate a prefix from parent_prefix_id. Requires parent_prefix_id and prefix_length.",
				Optional:    true,
			},
			"parent_prefix_id": schema.Int64Attribute{
				Description: "Parent prefix ID to allocate from when autoassign is true.",
				Optional:    true,
			},
			"prefix_length": schema.Int64Attribute{
				Description: "Prefix length for auto-allocated prefix (e.g., 24 for /24). Required when autoassign is true.",
				Optional:    true,
			},
			"vrf": schema.Int64Attribute{
				Description: "VRF ID that contains this prefix.",
				Optional:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "Tenant ID that owns this prefix.",
				Optional:    true,
			},
			"vlan": schema.Int64Attribute{
				Description: "VLAN ID to associate with this prefix. Used as an additional match dimension in autoassign+upsert lookups.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the prefix.",
				Optional:    true,
			},
			"comments": schema.StringAttribute{
				Description: "Additional comments.",
				Optional:    true,
			},
			"upsert": schema.BoolAttribute{
				Description: "If true, will find and use existing prefix with matching CIDR instead of creating a new one.",
				Optional:    true,
			},
			"tags": schema.ListNestedAttribute{
				Description: "Tags associated with this prefix.",
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

func (r *PrefixResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *PrefixResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PrefixResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate autoassign requirements
	if !data.Autoassign.IsNull() && data.Autoassign.ValueBool() {
		if data.ParentPrefixID.IsNull() {
			resp.Diagnostics.AddError("Configuration Error", "parent_prefix_id is required when autoassign is true")
			return
		}
		if data.PrefixLength.IsNull() {
			resp.Diagnostics.AddError("Configuration Error", "prefix_length is required when autoassign is true")
			return
		}
	}

	// Handle autoassign mode
	if !data.Autoassign.IsNull() && data.Autoassign.ValueBool() {
		parentID := data.ParentPrefixID.ValueInt64()

		// If upsert is true, search for existing prefix in parent with matching tenant and tags
		if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
			// Get all prefixes under the parent
			params := url.Values{}
			params.Add("parent_id", fmt.Sprintf("%d", parentID))

			results, err := r.client.GetList(ctx, "/api/ipam/prefixes/", params)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for existing prefixes, got error: %s", err))
				return
			}

			// Search for matching prefix by tenant and tags
			for _, result := range results {
				var existing PrefixAPIModel
				if err := json.Unmarshal(result, &existing); err != nil {
					continue
				}

				// Check tenant match
				tenantMatches := false
				if data.Tenant.IsNull() && existing.Tenant == nil {
					tenantMatches = true
				} else if !data.Tenant.IsNull() && existing.Tenant != nil && int64(existing.Tenant.ID) == data.Tenant.ValueInt64() {
					tenantMatches = true
				}

				if !tenantMatches {
					continue
				}

				// Check tags match
				tagsMatch := true
				if len(data.Tags) != len(existing.Tags) {
					tagsMatch = false
				} else {
					// Create maps for comparison
					dataTags := make(map[string]bool)
					for _, tag := range data.Tags {
						dataTags[tag.Slug.ValueString()] = true
					}
					for _, tag := range existing.Tags {
						if !dataTags[tag.Slug] {
							tagsMatch = false
							break
						}
					}
				}

				// Check VLAN match (only when vlan is set in config).
				vlanMatches := false
				if data.VLAN.IsNull() {
					// vlan not specified — skip vlan as a match criterion
					vlanMatches = true
				} else if existing.VLAN != nil && int64(existing.VLAN.ID) == data.VLAN.ValueInt64() {
					vlanMatches = true
				}

				if tagsMatch && vlanMatches {
					// Found existing prefix - update it to match desired state
					data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))
					data.Prefix = types.StringValue(existing.Prefix)

					updateData := PrefixAPIModel{
						Prefix: existing.Prefix,
					}

					if !data.VRF.IsNull() {
						updateData.VRF = &TenantIDOrObject{ID: int(data.VRF.ValueInt64())}
					}
					if !data.Tenant.IsNull() {
						updateData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
					}
					if !data.VLAN.IsNull() {
						updateData.VLAN = &TenantIDOrObject{ID: int(data.VLAN.ValueInt64())}
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

					apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/prefixes/%s/", data.ID.ValueString()), updateData)
					if err != nil {
						resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing prefix, got error: %s", err))
						return
					}

					var updated PrefixAPIModel
					if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
						resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse prefix response: %s", err))
						return
					}

					if updated.VRF != nil {
						data.VRF = types.Int64Value(int64(updated.VRF.ID))
					}
					if updated.Tenant != nil {
						data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
					}
					if updated.VLAN != nil {
						data.VLAN = types.Int64Value(int64(updated.VLAN.ID))
					}

					resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
					return
				}
			}
		}

		// Allocate new prefix from parent
		allocateData := map[string]interface{}{
			"prefix_length": data.PrefixLength.ValueInt64(),
		}

		if !data.VRF.IsNull() {
			allocateData["vrf"] = map[string]interface{}{"id": int(data.VRF.ValueInt64())}
		}
		if !data.Tenant.IsNull() {
			allocateData["tenant"] = map[string]interface{}{"id": int(data.Tenant.ValueInt64())}
		}
		if !data.VLAN.IsNull() {
			allocateData["vlan"] = map[string]interface{}{"id": int(data.VLAN.ValueInt64())}
		}
		if !data.Description.IsNull() {
			allocateData["description"] = data.Description.ValueString()
		}
		if !data.Comments.IsNull() {
			allocateData["comments"] = data.Comments.ValueString()
		}
		if len(data.Tags) > 0 {
			allocateData["tags"] = ConvertTagsToAPI(data.Tags)
		}

		apiResp, err := r.client.Create(ctx, fmt.Sprintf("/api/ipam/prefixes/%d/available-prefixes/", parentID), allocateData)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to allocate prefix from parent, got error: %s", err))
			return
		}

		var created PrefixAPIModel
		if err := json.Unmarshal(apiResp.Body, &created); err != nil {
			resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse allocation response: %s", err))
			return
		}
		data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
		data.Prefix = types.StringValue(created.Prefix)
		if created.VRF != nil {
			data.VRF = types.Int64Value(int64(created.VRF.ID))
		}
		if created.Tenant != nil {
			data.Tenant = types.Int64Value(int64(created.Tenant.ID))
		}
		if created.VLAN != nil {
			data.VLAN = types.Int64Value(int64(created.VLAN.ID))
		}

		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Check if we should search for existing prefix (non-autoassign mode)
	if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
		// Search for existing prefix by CIDR
		params := url.Values{}
		params.Add("prefix", data.Prefix.ValueString())

		results, err := r.client.GetList(ctx, "/api/ipam/prefixes/", params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for prefix, got error: %s", err))
			return
		}

		if len(results) > 0 {
			// Found existing prefix - update it to match desired state
			var existing PrefixAPIModel
			if err := json.Unmarshal(results[0], &existing); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse prefix response: %s", err))
				return
			}

			data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))

			// Update the existing prefix with desired configuration
			updateData := PrefixAPIModel{
				Prefix: data.Prefix.ValueString(),
			}

			if !data.VRF.IsNull() {
				updateData.VRF = &TenantIDOrObject{ID: int(data.VRF.ValueInt64())}
			}
			if !data.Tenant.IsNull() {
				updateData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
			}
			if !data.VLAN.IsNull() {
				updateData.VLAN = &TenantIDOrObject{ID: int(data.VLAN.ValueInt64())}
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

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/prefixes/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing prefix, got error: %s", err))
				return
			}

			var updated PrefixAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse prefix response: %s", err))
				return
			}

			if updated.VRF != nil {
				data.VRF = types.Int64Value(int64(updated.VRF.ID))
			}
			if updated.Tenant != nil {
				data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
			}
			if updated.VLAN != nil {
				data.VLAN = types.Int64Value(int64(updated.VLAN.ID))
			}

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// Create new prefix (normal mode)
	if data.Prefix.IsNull() {
		resp.Diagnostics.AddError("Configuration Error", "prefix is required when autoassign is false")
		return
	}

	createData := PrefixAPIModel{
		Prefix: data.Prefix.ValueString(),
	}

	if !data.VRF.IsNull() {
		createData.VRF = &TenantIDOrObject{ID: int(data.VRF.ValueInt64())}
	}
	if !data.Tenant.IsNull() {
		createData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
	}
	if !data.VLAN.IsNull() {
		createData.VLAN = &TenantIDOrObject{ID: int(data.VLAN.ValueInt64())}
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

	apiResp, err := r.client.Create(ctx, "/api/ipam/prefixes/", createData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create prefix, got error: %s", err))
		return
	}

	var created PrefixAPIModel
	if err := json.Unmarshal(apiResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse prefix response: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
	data.Prefix = types.StringValue(created.Prefix)
	if created.VRF != nil {
		data.VRF = types.Int64Value(int64(created.VRF.ID))
	}
	if created.Tenant != nil {
		data.Tenant = types.Int64Value(int64(created.Tenant.ID))
	}
	if created.VLAN != nil {
		data.VLAN = types.Int64Value(int64(created.VLAN.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PrefixResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PrefixResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/prefixes/%s/", data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read prefix, got error: %s", err))
		return
	}

	var prefix PrefixAPIModel
	if err := json.Unmarshal(apiResp.Body, &prefix); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse prefix response: %s", err))
		return
	}

	data.Prefix = types.StringValue(prefix.Prefix)
	if prefix.VRF != nil {
		data.VRF = types.Int64Value(int64(prefix.VRF.ID))
	}
	if prefix.Tenant != nil {
		data.Tenant = types.Int64Value(int64(prefix.Tenant.ID))
	}
	if prefix.VLAN != nil {
		data.VLAN = types.Int64Value(int64(prefix.VLAN.ID))
	} else {
		data.VLAN = types.Int64Null()
	}
	if prefix.Description != "" {
		data.Description = types.StringValue(prefix.Description)
	} else {
		data.Description = types.StringNull()
	}
	if prefix.Comments != "" {
		data.Comments = types.StringValue(prefix.Comments)
	} else {
		data.Comments = types.StringNull()
	}
	data.Tags = ConvertTagsFromAPI(prefix.Tags)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PrefixResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PrefixResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateData := PrefixAPIModel{
		Prefix: data.Prefix.ValueString(),
	}

	if !data.VRF.IsNull() {
		updateData.VRF = &TenantIDOrObject{ID: int(data.VRF.ValueInt64())}
	}
	if !data.Tenant.IsNull() {
		updateData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
	}
	if !data.VLAN.IsNull() {
		updateData.VLAN = &TenantIDOrObject{ID: int(data.VLAN.ValueInt64())}
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

	apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/prefixes/%s/", data.ID.ValueString()), updateData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update prefix, got error: %s", err))
		return
	}

	var updated PrefixAPIModel
	if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse prefix response: %s", err))
		return
	}

	if updated.VRF != nil {
		data.VRF = types.Int64Value(int64(updated.VRF.ID))
	}
	if updated.Tenant != nil {
		data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
	}
	if updated.VLAN != nil {
		data.VLAN = types.Int64Value(int64(updated.VLAN.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PrefixResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PrefixResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Some NetBox instances have validation rules requiring status='deprecated' before deletion
	// Update to deprecated status first to satisfy these rules
	deprecateData := map[string]interface{}{
		"status": "deprecated",
	}

	_, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/prefixes/%s/", data.ID.ValueString()), deprecateData)
	if err != nil {
		// If deprecation fails, log but continue with deletion attempt
		// Some instances may not require this step
		resp.Diagnostics.AddWarning("Deprecation Warning", fmt.Sprintf("Unable to set status to deprecated before deletion: %s", err))
	}

	_, err = r.client.Delete(ctx, fmt.Sprintf("/api/ipam/prefixes/%s/", data.ID.ValueString()))
	if err != nil {
		// Treat 404 as success - resource already deleted
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete prefix, got error: %s", err))
		return
	}
}

func (r *PrefixResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
