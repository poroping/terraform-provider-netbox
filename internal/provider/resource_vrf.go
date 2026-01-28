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
var _ resource.Resource = &VRFResource{}
var _ resource.ResourceWithImportState = &VRFResource{}

func NewVRFResource() resource.Resource {
	return &VRFResource{}
}

// VRFResource defines the resource implementation.
type VRFResource struct {
	client *client.Client
}

// VRFResourceModel describes the resource data model.
type VRFResourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	RD            types.String `tfsdk:"rd"`
	Tenant        types.Int64  `tfsdk:"tenant"`
	EnforceUnique types.Bool   `tfsdk:"enforce_unique"`
	Description   types.String `tfsdk:"description"`
	Comments      types.String `tfsdk:"comments"`
	Tags          []TagRef     `tfsdk:"tags"`
	Upsert        types.Bool   `tfsdk:"upsert"`
}

// TenantIDOrObject can represent either an ID or a full tenant object
type TenantIDOrObject struct {
	ID int `json:"id"`
}

// StatusValue represents a NetBox status field which returns as an object
type StatusValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// VRFAPIModel represents the NetBox API response for a VRF
type VRFAPIModel struct {
	ID            int               `json:"id"`
	Name          string            `json:"name"`
	RD            string            `json:"rd,omitempty"`
	Tenant        *TenantIDOrObject `json:"tenant,omitempty"`
	EnforceUnique bool              `json:"enforce_unique,omitempty"`
	Description   string            `json:"description,omitempty"`
	Comments      string            `json:"comments,omitempty"`
	Tags          []TagAPIRef       `json:"tags,omitempty"`
}

func (r *VRFResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vrf"
}

func (r *VRFResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NetBox VRF (Virtual Routing and Forwarding) instance.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NetBox internal ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the VRF.",
				Required:    true,
			},
			"rd": schema.StringAttribute{
				Description: "Route distinguisher (RD) for the VRF.",
				Optional:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "Tenant ID that owns this VRF.",
				Optional:    true,
			},
			"enforce_unique": schema.BoolAttribute{
				Description: "If true, prevent duplicate prefixes/IPs in this VRF.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the VRF.",
				Optional:    true,
			},
			"comments": schema.StringAttribute{
				Description: "Additional comments.",
				Optional:    true,
			},
			"upsert": schema.BoolAttribute{
				Description: "If true, will find and use existing VRF with matching name instead of creating a new one.",
				Optional:    true,
			},
			"tags": schema.ListNestedAttribute{
				Description: "Tags associated with this VRF.",
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

func (r *VRFResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VRFResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VRFResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if we should search for existing VRF
	if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
		// Search for existing VRF by name
		params := url.Values{}
		params.Add("name", data.Name.ValueString())

		results, err := r.client.GetList(ctx, "/api/ipam/vrfs/", params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for VRF, got error: %s", err))
			return
		}

		if len(results) > 0 {
			// Found existing VRF - update it to match desired state
			var existing VRFAPIModel
			if err := json.Unmarshal(results[0], &existing); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VRF response: %s", err))
				return
			}

			data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))

			// Update the existing VRF with desired configuration
			updateData := VRFAPIModel{
				Name: data.Name.ValueString(),
			}

			if !data.RD.IsNull() {
				updateData.RD = data.RD.ValueString()
			}
			if !data.Tenant.IsNull() {
				updateData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
			}
			if !data.EnforceUnique.IsNull() {
				updateData.EnforceUnique = data.EnforceUnique.ValueBool()
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

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/vrfs/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing VRF, got error: %s", err))
				return
			}

			var updated VRFAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VRF response: %s", err))
				return
			}

			// Update state with computed values
			if updated.RD != "" {
				data.RD = types.StringValue(updated.RD)
			}
			if updated.Tenant != nil {
				data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
			}

			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// Create new VRF
	createData := VRFAPIModel{
		Name: data.Name.ValueString(),
	}

	if !data.RD.IsNull() {
		createData.RD = data.RD.ValueString()
	}
	if !data.Tenant.IsNull() {
		createData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
	}
	if !data.EnforceUnique.IsNull() {
		createData.EnforceUnique = data.EnforceUnique.ValueBool()
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

	apiResp, err := r.client.Create(ctx, "/api/ipam/vrfs/", createData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create VRF, got error: %s", err))
		return
	}

	var created VRFAPIModel
	if err := json.Unmarshal(apiResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VRF response: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
	if created.RD != "" {
		data.RD = types.StringValue(created.RD)
	}
	if created.Tenant != nil {
		data.Tenant = types.Int64Value(int64(created.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VRFResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VRFResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/vrfs/%s/", data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read VRF, got error: %s", err))
		return
	}

	var vrf VRFAPIModel
	if err := json.Unmarshal(apiResp.Body, &vrf); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VRF response: %s", err))
		return
	}

	data.Name = types.StringValue(vrf.Name)
	if vrf.RD != "" {
		data.RD = types.StringValue(vrf.RD)
	} else {
		data.RD = types.StringNull()
	}
	if vrf.Tenant != nil {
		data.Tenant = types.Int64Value(int64(vrf.Tenant.ID))
	}
	// Only set enforce_unique if it was specified in the plan
	if !data.EnforceUnique.IsNull() {
		data.EnforceUnique = types.BoolValue(vrf.EnforceUnique)
	} else {
		data.EnforceUnique = types.BoolNull()
	}
	if vrf.Description != "" {
		data.Description = types.StringValue(vrf.Description)
	} else {
		data.Description = types.StringNull()
	}
	if vrf.Comments != "" {
		data.Comments = types.StringValue(vrf.Comments)
	} else {
		data.Comments = types.StringNull()
	}
	data.Tags = ConvertTagsFromAPI(vrf.Tags)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VRFResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data VRFResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateData := VRFAPIModel{
		Name: data.Name.ValueString(),
	}

	if !data.RD.IsNull() {
		updateData.RD = data.RD.ValueString()
	}
	if !data.Tenant.IsNull() {
		updateData.Tenant = &TenantIDOrObject{ID: int(data.Tenant.ValueInt64())}
	}
	if !data.EnforceUnique.IsNull() {
		updateData.EnforceUnique = data.EnforceUnique.ValueBool()
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

	apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/vrfs/%s/", data.ID.ValueString()), updateData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update VRF, got error: %s", err))
		return
	}

	var updated VRFAPIModel
	if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse VRF response: %s", err))
		return
	}

	// Update state with computed values
	if updated.RD != "" {
		data.RD = types.StringValue(updated.RD)
	}
	if updated.Tenant != nil {
		data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VRFResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VRFResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, fmt.Sprintf("/api/ipam/vrfs/%s/", data.ID.ValueString()))
	if err != nil {
		// Treat 404 as success since resource is already gone
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete VRF, got error: %s", err))
		return
	}
}

func (r *VRFResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
