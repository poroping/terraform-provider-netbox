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
var _ resource.Resource = &ASNResource{}
var _ resource.ResourceWithImportState = &ASNResource{}

func NewASNResource() resource.Resource {
	return &ASNResource{}
}

// ASNResource defines the resource implementation.
type ASNResource struct {
	client *client.Client
}

// ASNResourceModel describes the resource data model.
type ASNResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ASN              types.Int64  `tfsdk:"asn"`
	RIR              types.Int64  `tfsdk:"rir"`
	Tenant           types.Int64  `tfsdk:"tenant"`
	Description      types.String `tfsdk:"description"`
	Comments         types.String `tfsdk:"comments"`
	Tags             []TagRef     `tfsdk:"tags"`
	Upsert           types.Bool   `tfsdk:"upsert"`
	Autoassign       types.Bool   `tfsdk:"autoassign"`
	ParentASNRangeID types.Int64  `tfsdk:"parent_asn_range_id"`
}

// ASNAPIModel represents the NetBox API response for an ASN
type ASNAPIModel struct {
	ID  int   `json:"id"`
	ASN int64 `json:"asn"`
	RIR *struct {
		ID int `json:"id"`
	} `json:"rir,omitempty"`
	Tenant      *TenantIDOrObject `json:"tenant,omitempty"`
	Description string            `json:"description,omitempty"`
	Comments    string            `json:"comments,omitempty"`
	Tags        []TagAPIRef       `json:"tags,omitempty"`
}

func (r *ASNResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asn"
}

func (r *ASNResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NetBox ASN (Autonomous System Number).",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NetBox internal ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"asn": schema.Int64Attribute{
				Description: "Autonomous System Number. Optional when autoassign is true.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"autoassign": schema.BoolAttribute{
				Description: "If true, automatically allocate an ASN from parent_asn_range_id. Requires parent_asn_range_id.",
				Optional:    true,
			},
			"parent_asn_range_id": schema.Int64Attribute{
				Description: "Parent ASN range ID to allocate from when autoassign is true.",
				Optional:    true,
			},
			"rir": schema.Int64Attribute{
				Description: "RIR ID that allocated this ASN.",
				Optional:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "Tenant ID that owns this ASN.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the ASN.",
				Optional:    true,
			},
			"comments": schema.StringAttribute{
				Description: "Additional comments.",
				Optional:    true,
			},
			"upsert": schema.BoolAttribute{
				Description: "If true, will find and use existing ASN with matching number instead of creating a new one.",
				Optional:    true,
			},
			"tags": schema.ListNestedAttribute{
				Description: "Tags associated with this ASN.",
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

func (r *ASNResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ASNResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ASNResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate autoassign requirements
	if !data.Autoassign.IsNull() && data.Autoassign.ValueBool() {
		if data.ParentASNRangeID.IsNull() {
			resp.Diagnostics.AddError("Configuration Error", "parent_asn_range_id is required when autoassign is true")
			return
		}
	}

	// Handle autoassign mode
	if !data.Autoassign.IsNull() && data.Autoassign.ValueBool() {
		parentID := data.ParentASNRangeID.ValueInt64()

		// If upsert is true, search for existing ASN in range with matching tenant and tags
		if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
			// Get parent ASN range to determine the range bounds
			apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/asn-ranges/%d/", parentID))
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read parent ASN range, got error: %s", err))
				return
			}

			var asnRange struct {
				ID    int   `json:"id"`
				Start int64 `json:"start"`
				End   int64 `json:"end"`
			}
			if err := json.Unmarshal(apiResp.Body, &asnRange); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN range response: %s", err))
				return
			}

			// Search for ASNs within the parent range
			params := url.Values{}
			params.Add("asn__gte", fmt.Sprintf("%d", asnRange.Start))
			params.Add("asn__lte", fmt.Sprintf("%d", asnRange.End))

			results, err := r.client.GetList(ctx, "/api/ipam/asns/", params)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for existing ASNs, got error: %s", err))
				return
			}

			// Search for matching ASN by tenant and tags
			for _, result := range results {
				var existing ASNAPIModel
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

				if tagsMatch {
					// Found existing ASN - update it to match desired state
					data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))
					data.ASN = types.Int64Value(existing.ASN)

					// Update the existing ASN with desired configuration
					updateData := ASNAPIModel{
						ASN: existing.ASN,
					}

					if !data.RIR.IsNull() {
						updateData.RIR = &struct {
							ID int `json:"id"`
						}{ID: int(data.RIR.ValueInt64())}
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

					apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/asns/%s/", data.ID.ValueString()), updateData)
					if err != nil {
						resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing ASN, got error: %s", err))
						return
					}

					var updated ASNAPIModel
					if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
						resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN response: %s", err))
						return
					}

					if updated.RIR != nil {
						data.RIR = types.Int64Value(int64(updated.RIR.ID))
					}
					if updated.Tenant != nil {
						data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
					}

					resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
					return
				}
			}
		}

		// Allocate new ASN from range
		allocateData := map[string]interface{}{}

		if !data.RIR.IsNull() {
			allocateData["rir"] = map[string]interface{}{"id": int(data.RIR.ValueInt64())}
		}
		if !data.Tenant.IsNull() {
			allocateData["tenant"] = map[string]interface{}{"id": int(data.Tenant.ValueInt64())}
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

		apiResp, err := r.client.Create(ctx, fmt.Sprintf("/api/ipam/asn-ranges/%d/available-asns/", parentID), allocateData)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to allocate ASN from range, got error: %s", err))
			return
		}

		var created ASNAPIModel
		if err := json.Unmarshal(apiResp.Body, &created); err != nil {
			resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse allocation response: %s", err))
			return
		}
		data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
		data.ASN = types.Int64Value(created.ASN)
		if created.RIR != nil {
			data.RIR = types.Int64Value(int64(created.RIR.ID))
		}
		if created.Tenant != nil {
			data.Tenant = types.Int64Value(int64(created.Tenant.ID))
		}

		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Check if we should search for existing ASN (non-autoassign mode)
	if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
		// Search for existing ASN by number
		params := url.Values{}
		params.Add("asn", fmt.Sprintf("%d", data.ASN.ValueInt64()))

		results, err := r.client.GetList(ctx, "/api/ipam/asns/", params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for ASN, got error: %s", err))
			return
		}

		if len(results) > 0 {
			// Found existing ASN - update it to match desired state
			var existing ASNAPIModel
			if err := json.Unmarshal(results[0], &existing); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN response: %s", err))
				return
			}

			data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))

			// Update the existing ASN with desired configuration
			updateData := ASNAPIModel{
				ASN: data.ASN.ValueInt64(),
			}

			if !data.RIR.IsNull() {
				updateData.RIR = &struct {
					ID int `json:"id"`
				}{ID: int(data.RIR.ValueInt64())}
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

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/asns/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing ASN, got error: %s", err))
				return
			}

			var updated ASNAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN response: %s", err))
				return
			}

			if updated.RIR != nil {
				data.RIR = types.Int64Value(int64(updated.RIR.ID))
			}
			if updated.Tenant != nil {
				data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
			}

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// Create new ASN (normal mode)
	if data.ASN.IsNull() {
		resp.Diagnostics.AddError("Configuration Error", "asn is required when autoassign is false")
		return
	}

	createData := ASNAPIModel{
		ASN: data.ASN.ValueInt64(),
	}

	if !data.RIR.IsNull() {
		createData.RIR = &struct {
			ID int `json:"id"`
		}{ID: int(data.RIR.ValueInt64())}
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

	apiResp, err := r.client.Create(ctx, "/api/ipam/asns/", createData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create ASN, got error: %s", err))
		return
	}

	var created ASNAPIModel
	if err := json.Unmarshal(apiResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN response: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
	data.ASN = types.Int64Value(created.ASN)
	if created.RIR != nil {
		data.RIR = types.Int64Value(int64(created.RIR.ID))
	}
	if created.Tenant != nil {
		data.Tenant = types.Int64Value(int64(created.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ASNResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ASNResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/asns/%s/", data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read ASN, got error: %s", err))
		return
	}

	var asn ASNAPIModel
	if err := json.Unmarshal(apiResp.Body, &asn); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN response: %s", err))
		return
	}

	data.ASN = types.Int64Value(asn.ASN)
	if asn.RIR != nil {
		data.RIR = types.Int64Value(int64(asn.RIR.ID))
	}
	if asn.Tenant != nil {
		data.Tenant = types.Int64Value(int64(asn.Tenant.ID))
	}

	if asn.Description == "" {
		data.Description = types.StringNull()
	} else {
		data.Description = types.StringValue(asn.Description)
	}

	if asn.Comments == "" {
		data.Comments = types.StringNull()
	} else {
		data.Comments = types.StringValue(asn.Comments)
	}

	data.Tags = ConvertTagsFromAPI(asn.Tags)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ASNResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ASNResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateData := ASNAPIModel{
		ASN: data.ASN.ValueInt64(),
	}

	if !data.RIR.IsNull() {
		updateData.RIR = &struct {
			ID int `json:"id"`
		}{ID: int(data.RIR.ValueInt64())}
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

	apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/asns/%s/", data.ID.ValueString()), updateData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update ASN, got error: %s", err))
		return
	}

	var updated ASNAPIModel
	if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN response: %s", err))
		return
	}

	if updated.RIR != nil {
		data.RIR = types.Int64Value(int64(updated.RIR.ID))
	}
	if updated.Tenant != nil {
		data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ASNResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ASNResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, fmt.Sprintf("/api/ipam/asns/%s/", data.ID.ValueString()))
	if err != nil {
		// Treat 404 as success since resource is already gone
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete ASN, got error: %s", err))
		return
	}
}

func (r *ASNResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
