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
var _ resource.Resource = &IPAddressResource{}
var _ resource.ResourceWithImportState = &IPAddressResource{}

func NewIPAddressResource() resource.Resource {
	return &IPAddressResource{}
}

// IPAddressResource defines the resource implementation.
type IPAddressResource struct {
	client *client.Client
}

// IPAddressResourceModel describes the resource data model.
type IPAddressResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Address        types.String `tfsdk:"address"`
	VRF            types.Int64  `tfsdk:"vrf"`
	Tenant         types.Int64  `tfsdk:"tenant"`
	DNSName        types.String `tfsdk:"dns_name"`
	Description    types.String `tfsdk:"description"`
	Comments       types.String `tfsdk:"comments"`
	Tags           []TagRef     `tfsdk:"tags"`
	Upsert         types.Bool   `tfsdk:"upsert"`
	Autoassign     types.Bool   `tfsdk:"autoassign"`
	ParentPrefixID types.Int64  `tfsdk:"parent_prefix_id"`
}

// IPAddressAPIModel represents the NetBox API response for an IP address
type IPAddressAPIModel struct {
	ID          int               `json:"id"`
	Address     string            `json:"address"`
	VRF         *struct{ ID int } `json:"vrf,omitempty"`
	Tenant      *TenantIDOrObject `json:"tenant,omitempty"`
	DNSName     string            `json:"dns_name,omitempty"`
	Description string            `json:"description,omitempty"`
	Comments    string            `json:"comments,omitempty"`
	Tags        []TagAPIRef       `json:"tags,omitempty"`
}

func (r *IPAddressResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_address"
}

func (r *IPAddressResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NetBox IP address.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NetBox internal ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"address": schema.StringAttribute{
				Description: "IP address with prefix length (e.g., '10.0.0.1/24'). Computed when autoassign is true.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vrf": schema.Int64Attribute{
				Description: "VRF ID that contains this IP address.",
				Optional:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "Tenant ID that owns this IP address.",
				Optional:    true,
			},
			"dns_name": schema.StringAttribute{
				Description: "DNS name for this IP address.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the IP address.",
				Optional:    true,
			},
			"comments": schema.StringAttribute{
				Description: "Additional comments.",
				Optional:    true,
			},
			"upsert": schema.BoolAttribute{
				Description: "If true, will find and use existing IP address with matching address instead of creating a new one. When combined with autoassign, matches within the parent prefix by dns_name (if set) or tenant+tags.",
				Optional:    true,
			},
			"autoassign": schema.BoolAttribute{
				Description: "If true, automatically allocate an IP address from parent_prefix_id using the /api/ipam/prefixes/{id}/available-ips/ endpoint. Requires parent_prefix_id to be set.",
				Optional:    true,
			},
			"parent_prefix_id": schema.Int64Attribute{
				Description: "Parent prefix ID to allocate from when autoassign is true.",
				Optional:    true,
			},
			"tags": schema.ListNestedAttribute{
				Description: "Tags associated with this IP address.",
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

func (r *IPAddressResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *IPAddressResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data IPAddressResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	isAutoassign := !data.Autoassign.IsNull() && data.Autoassign.ValueBool()
	isUpsert := !data.Upsert.IsNull() && data.Upsert.ValueBool()

	// Validate requirements
	if isAutoassign {
		if data.ParentPrefixID.IsNull() {
			resp.Diagnostics.AddError("Configuration Error", "parent_prefix_id is required when autoassign is true")
			return
		}
	} else {
		if data.Address.IsNull() || data.Address.ValueString() == "" {
			resp.Diagnostics.AddError("Configuration Error", "address must be set when autoassign is false")
			return
		}
	}

	// autoassign + upsert: search for an existing IP within the parent prefix before
	// allocating a new one. Matches by dns_name (if set) or tenant+tags.
	if isAutoassign && isUpsert {
		// Fetch the parent prefix to learn its CIDR for scoped searching.
		prefixResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/prefixes/%d/", data.ParentPrefixID.ValueInt64()))
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read parent prefix, got error: %s", err))
			return
		}
		var parentPrefix PrefixAPIModel
		if err := json.Unmarshal(prefixResp.Body, &parentPrefix); err != nil {
			resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse parent prefix: %s", err))
			return
		}

		searchParams := url.Values{}
		searchParams.Add("parent", parentPrefix.Prefix)
		if !data.DNSName.IsNull() && data.DNSName.ValueString() != "" {
			searchParams.Add("dns_name", data.DNSName.ValueString())
		}

		candidates, err := r.client.GetList(ctx, "/api/ipam/ip-addresses/", searchParams)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for existing IP addresses, got error: %s", err))
			return
		}

		for _, raw := range candidates {
			var candidate IPAddressAPIModel
			if err := json.Unmarshal(raw, &candidate); err != nil {
				continue
			}

			// Tenant must match.
			tenantMatch := false
			if data.Tenant.IsNull() && candidate.Tenant == nil {
				tenantMatch = true
			} else if !data.Tenant.IsNull() && candidate.Tenant != nil && int64(candidate.Tenant.ID) == data.Tenant.ValueInt64() {
				tenantMatch = true
			}
			if !tenantMatch {
				continue
			}

			// When dns_name is not the primary key, also require tags to match.
			if data.DNSName.IsNull() || data.DNSName.ValueString() == "" {
				if len(data.Tags) != len(candidate.Tags) {
					continue
				}
				slugSet := make(map[string]bool, len(data.Tags))
				for _, t := range data.Tags {
					slugSet[t.Slug.ValueString()] = true
				}
				tagsMatch := true
				for _, t := range candidate.Tags {
					if !slugSet[t.Slug] {
						tagsMatch = false
						break
					}
				}
				if !tagsMatch {
					continue
				}
			}

			// Found an existing IP — adopt it.
			data.ID = types.StringValue(fmt.Sprintf("%d", candidate.ID))
			data.Address = types.StringValue(candidate.Address)

			updateData := IPAddressAPIModel{Address: candidate.Address}
			if !data.VRF.IsNull() {
				updateData.VRF = &struct{ ID int }{ID: int(data.VRF.ValueInt64())}
			}
			if !data.Tenant.IsNull() {
				updateData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
			}
			if !data.DNSName.IsNull() {
				updateData.DNSName = data.DNSName.ValueString()
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

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/ip-addresses/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing IP address, got error: %s", err))
				return
			}
			var updated IPAddressAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP address response: %s", err))
				return
			}
			if updated.VRF != nil {
				data.VRF = types.Int64Value(int64(updated.VRF.ID))
			}
			if updated.Tenant != nil {
				data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// Autoassign: allocate a new IP from the parent prefix.
	if isAutoassign {
		allocateData := map[string]interface{}{}
		if !data.VRF.IsNull() {
			allocateData["vrf"] = map[string]interface{}{"id": data.VRF.ValueInt64()}
		}
		if !data.Tenant.IsNull() {
			allocateData["tenant"] = map[string]interface{}{"id": data.Tenant.ValueInt64()}
		}
		if !data.DNSName.IsNull() {
			allocateData["dns_name"] = data.DNSName.ValueString()
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

		apiResp, err := r.client.Create(ctx, fmt.Sprintf("/api/ipam/prefixes/%d/available-ips/", data.ParentPrefixID.ValueInt64()), allocateData)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to allocate IP address from prefix, got error: %s", err))
			return
		}

		var created IPAddressAPIModel
		if err := json.Unmarshal(apiResp.Body, &created); err != nil {
			resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP address response: %s", err))
			return
		}

		data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
		data.Address = types.StringValue(created.Address)
		if created.VRF != nil {
			data.VRF = types.Int64Value(int64(created.VRF.ID))
		}
		if created.Tenant != nil {
			data.Tenant = types.Int64Value(int64(created.Tenant.ID))
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Upsert (non-autoassign): search for existing IP by exact address.
	if isUpsert {
		params := url.Values{}
		params.Add("address", data.Address.ValueString())

		results, err := r.client.GetList(ctx, "/api/ipam/ip-addresses/", params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for IP address, got error: %s", err))
			return
		}

		if len(results) > 0 {
			var existing IPAddressAPIModel
			if err := json.Unmarshal(results[0], &existing); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP address response: %s", err))
				return
			}

			data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))

			updateData := IPAddressAPIModel{Address: data.Address.ValueString()}
			if !data.VRF.IsNull() {
				updateData.VRF = &struct{ ID int }{ID: int(data.VRF.ValueInt64())}
			}
			if !data.Tenant.IsNull() {
				updateData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
			}
			if !data.DNSName.IsNull() {
				updateData.DNSName = data.DNSName.ValueString()
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

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/ip-addresses/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing IP address, got error: %s", err))
				return
			}

			var updated IPAddressAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP address response: %s", err))
				return
			}
			if updated.VRF != nil {
				data.VRF = types.Int64Value(int64(updated.VRF.ID))
			}
			if updated.Tenant != nil {
				data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// Normal create.
	createData := IPAddressAPIModel{
		Address: data.Address.ValueString(),
	}
	if !data.VRF.IsNull() {
		createData.VRF = &struct{ ID int }{ID: int(data.VRF.ValueInt64())}
	}
	if !data.Tenant.IsNull() {
		createData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
	}
	if !data.DNSName.IsNull() {
		createData.DNSName = data.DNSName.ValueString()
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

	apiResp, err := r.client.Create(ctx, "/api/ipam/ip-addresses/", createData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create IP address, got error: %s", err))
		return
	}

	var created IPAddressAPIModel
	if err := json.Unmarshal(apiResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP address response: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
	if created.VRF != nil {
		data.VRF = types.Int64Value(int64(created.VRF.ID))
	}
	if created.Tenant != nil {
		data.Tenant = types.Int64Value(int64(created.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IPAddressResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IPAddressResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/ip-addresses/%s/", data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read IP address, got error: %s", err))
		return
	}

	var ipAddress IPAddressAPIModel
	if err := json.Unmarshal(apiResp.Body, &ipAddress); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP address response: %s", err))
		return
	}

	data.Address = types.StringValue(ipAddress.Address)
	if ipAddress.VRF != nil {
		data.VRF = types.Int64Value(int64(ipAddress.VRF.ID))
	}
	if ipAddress.Tenant != nil {
		data.Tenant = types.Int64Value(int64(ipAddress.Tenant.ID))
	}
	if ipAddress.DNSName != "" {
		data.DNSName = types.StringValue(ipAddress.DNSName)
	} else {
		data.DNSName = types.StringNull()
	}
	if ipAddress.Description != "" {
		data.Description = types.StringValue(ipAddress.Description)
	} else {
		data.Description = types.StringNull()
	}
	if ipAddress.Comments != "" {
		data.Comments = types.StringValue(ipAddress.Comments)
	} else {
		data.Comments = types.StringNull()
	}
	data.Tags = ConvertTagsFromAPI(ipAddress.Tags)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IPAddressResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data IPAddressResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateData := IPAddressAPIModel{
		Address: data.Address.ValueString(),
	}

	if !data.VRF.IsNull() {
		updateData.VRF = &struct{ ID int }{ID: int(data.VRF.ValueInt64())}
	}
	if !data.Tenant.IsNull() {
		updateData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
	}
	if !data.DNSName.IsNull() {
		updateData.DNSName = data.DNSName.ValueString()
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

	apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/ip-addresses/%s/", data.ID.ValueString()), updateData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update IP address, got error: %s", err))
		return
	}

	var updated IPAddressAPIModel
	if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP address response: %s", err))
		return
	}

	if updated.VRF != nil {
		data.VRF = types.Int64Value(int64(updated.VRF.ID))
	}
	if updated.Tenant != nil {
		data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IPAddressResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data IPAddressResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Some NetBox instances have validation rules requiring status='deprecated' before deletion
	// Update to deprecated status first to satisfy these rules
	deprecateData := map[string]interface{}{
		"status": "deprecated",
	}

	_, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/ip-addresses/%s/", data.ID.ValueString()), deprecateData)
	if err != nil {
		// If deprecation fails, log but continue with deletion attempt
		// Some instances may not require this step
		resp.Diagnostics.AddWarning("Deprecation Warning", fmt.Sprintf("Unable to set status to deprecated before deletion: %s", err))
	}

	_, err = r.client.Delete(ctx, fmt.Sprintf("/api/ipam/ip-addresses/%s/", data.ID.ValueString()))
	if err != nil {
		// Treat 404 as success - resource already deleted
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete IP address, got error: %s", err))
		return
	}
}

func (r *IPAddressResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
