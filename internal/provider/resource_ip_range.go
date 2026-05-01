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
var _ resource.Resource = &IPRangeResource{}
var _ resource.ResourceWithImportState = &IPRangeResource{}

func NewIPRangeResource() resource.Resource {
	return &IPRangeResource{}
}

// IPRangeResource defines the resource implementation.
type IPRangeResource struct {
	client *client.Client
}

// IPRangeResourceModel describes the resource data model.
type IPRangeResourceModel struct {
	ID           types.String `tfsdk:"id"`
	StartAddress types.String `tfsdk:"start_address"`
	EndAddress   types.String `tfsdk:"end_address"`
	Status       types.String `tfsdk:"status"`
	VRF          types.Int64  `tfsdk:"vrf"`
	Tenant       types.Int64  `tfsdk:"tenant"`
	Description  types.String `tfsdk:"description"`
	Comments     types.String `tfsdk:"comments"`
	Tags         []TagRef     `tfsdk:"tags"`
	Upsert       types.Bool   `tfsdk:"upsert"`
}

// IPRangeAPIModel represents the NetBox API response for an IP range
type IPRangeAPIModel struct {
	ID           int               `json:"id"`
	StartAddress string            `json:"start_address"`
	EndAddress   string            `json:"end_address"`
	Status       string            `json:"status,omitempty"`
	VRF          *TenantIDOrObject `json:"vrf,omitempty"`
	Tenant       *TenantIDOrObject `json:"tenant,omitempty"`
	Description  string            `json:"description,omitempty"`
	Comments     string            `json:"comments,omitempty"`
	Tags         []TagAPIRef       `json:"tags,omitempty"`
}

func (r *IPRangeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_range"
}

func (r *IPRangeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NetBox IP range.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NetBox internal ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"start_address": schema.StringAttribute{
				Description: "Starting IP address in the range.",
				Required:    true,
			},
			"end_address": schema.StringAttribute{
				Description: "Ending IP address in the range.",
				Required:    true,
			},
			"status": schema.StringAttribute{
				Description: "Status of the IP range (active, reserved, deprecated).",
				Optional:    true,
			},
			"vrf": schema.Int64Attribute{
				Description: "VRF ID that contains this IP range.",
				Optional:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "Tenant ID that owns this IP range.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the IP range.",
				Optional:    true,
			},
			"comments": schema.StringAttribute{
				Description: "Additional comments.",
				Optional:    true,
			},
			"upsert": schema.BoolAttribute{
				Description: "If true, will find and use existing IP range with matching start/end addresses instead of creating a new one.",
				Optional:    true,
			}, "tags": schema.ListNestedAttribute{
				Description: "Tags associated with this IP range.",
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
			}},
	}
}

func (r *IPRangeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *IPRangeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data IPRangeResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if we should search for existing IP range
	if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
		// Search for existing IP range by start and end addresses
		params := url.Values{}
		params.Add("start_address", data.StartAddress.ValueString())
		params.Add("end_address", data.EndAddress.ValueString())

		results, err := r.client.GetList(ctx, "/api/ipam/ip-ranges/", params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for IP range, got error: %s", err))
			return
		}

		if len(results) > 0 {
			// Found existing IP range - update it to match desired state
			var existing IPRangeAPIModel
			if err := json.Unmarshal(results[0], &existing); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP range response: %s", err))
				return
			}

			data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))

			// Update the existing IP range with desired configuration
			updateData := IPRangeAPIModel{
				StartAddress: data.StartAddress.ValueString(),
				EndAddress:   data.EndAddress.ValueString(),
			}

			if !data.Status.IsNull() {
				updateData.Status = data.Status.ValueString()
			}
			if !data.VRF.IsNull() {
				updateData.VRF = &TenantIDOrObject{ID: int(data.VRF.ValueInt64())}
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

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/ip-ranges/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing IP range, got error: %s", err))
				return
			}

			var updated IPRangeAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP range response: %s", err))
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

	// Create new IP range
	createData := IPRangeAPIModel{
		StartAddress: data.StartAddress.ValueString(),
		EndAddress:   data.EndAddress.ValueString(),
	}

	if !data.Status.IsNull() {
		createData.Status = data.Status.ValueString()
	}
	if !data.VRF.IsNull() {
		createData.VRF = &TenantIDOrObject{ID: int(data.VRF.ValueInt64())}
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

	apiResp, err := r.client.Create(ctx, "/api/ipam/ip-ranges/", createData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create IP range, got error: %s", err))
		return
	}

	var created IPRangeAPIModel
	if err := json.Unmarshal(apiResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP range response: %s", err))
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

func (r *IPRangeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IPRangeResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/ip-ranges/%s/", data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read IP range, got error: %s", err))
		return
	}

	var ipRange IPRangeAPIModel
	if err := json.Unmarshal(apiResp.Body, &ipRange); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP range response: %s", err))
		return
	}

	data.StartAddress = types.StringValue(ipRange.StartAddress)
	data.EndAddress = types.StringValue(ipRange.EndAddress)
	if ipRange.Status != "" {
		data.Status = types.StringValue(ipRange.Status)
	}
	if ipRange.VRF != nil {
		data.VRF = types.Int64Value(int64(ipRange.VRF.ID))
	}
	if ipRange.Tenant != nil {
		data.Tenant = types.Int64Value(int64(ipRange.Tenant.ID))
	}
	data.Description = types.StringValue(ipRange.Description)
	data.Comments = types.StringValue(ipRange.Comments)
	data.Tags = ConvertTagsFromAPI(ipRange.Tags)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IPRangeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data IPRangeResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateData := IPRangeAPIModel{
		StartAddress: data.StartAddress.ValueString(),
		EndAddress:   data.EndAddress.ValueString(),
	}

	if !data.Status.IsNull() {
		updateData.Status = data.Status.ValueString()
	}
	if !data.VRF.IsNull() {
		updateData.VRF = &TenantIDOrObject{ID: int(data.VRF.ValueInt64())}
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

	apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/ip-ranges/%s/", data.ID.ValueString()), updateData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update IP range, got error: %s", err))
		return
	}

	var updated IPRangeAPIModel
	if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse IP range response: %s", err))
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

func (r *IPRangeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data IPRangeResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, fmt.Sprintf("/api/ipam/ip-ranges/%s/", data.ID.ValueString()))
	if err != nil {
		// Treat 404 as success since resource is already gone
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete IP range, got error: %s", err))
		return
	}
}

func (r *IPRangeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
