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
var _ resource.Resource = &ASNRangeResource{}
var _ resource.ResourceWithImportState = &ASNRangeResource{}

func NewASNRangeResource() resource.Resource {
	return &ASNRangeResource{}
}

// ASNRangeResource defines the resource implementation.
type ASNRangeResource struct {
	client *client.Client
}

// ASNRangeResourceModel describes the resource data model.
type ASNRangeResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Slug        types.String `tfsdk:"slug"`
	Start       types.Int64  `tfsdk:"start"`
	End         types.Int64  `tfsdk:"end"`
	RIR         types.Int64  `tfsdk:"rir"`
	Tenant      types.Int64  `tfsdk:"tenant"`
	Description types.String `tfsdk:"description"`
	Tags        []TagRef     `tfsdk:"tags"`
	Upsert      types.Bool   `tfsdk:"upsert"`
}

// ASNRangeAPIModel represents the NetBox API response for an ASN range
type ASNRangeAPIModel struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
	RIR   *struct {
		ID int `json:"id"`
	} `json:"rir,omitempty"`
	Tenant      *TenantIDOrObject `json:"tenant,omitempty"`
	Description string            `json:"description,omitempty"`
	Tags        []TagAPIRef       `json:"tags,omitempty"`
}

func (r *ASNRangeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asn_range"
}

func (r *ASNRangeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NetBox ASN range.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "NetBox internal ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the ASN range.",
				Required:    true,
			},
			"slug": schema.StringAttribute{
				Description: "URL-friendly slug for the ASN range. Auto-generated from name if not provided.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"start": schema.Int64Attribute{
				Description: "Starting ASN in the range.",
				Required:    true,
			},
			"end": schema.Int64Attribute{
				Description: "Ending ASN in the range.",
				Required:    true,
			},
			"rir": schema.Int64Attribute{
				Description: "RIR ID that allocated this range.",
				Optional:    true,
			},
			"tenant": schema.Int64Attribute{
				Description: "Tenant ID that owns this range.",
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description of the ASN range.",
				Optional:    true,
			},
			"upsert": schema.BoolAttribute{
				Description: "If true, will find and use existing ASN range with matching name instead of creating a new one.",
				Optional:    true,
			},
			"tags": schema.ListNestedAttribute{
				Description: "Tags associated with this ASN range.",
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

func (r *ASNRangeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ASNRangeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ASNRangeResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if we should search for existing ASN range
	if !data.Upsert.IsNull() && data.Upsert.ValueBool() {
		// Search for existing ASN range by name
		params := url.Values{}
		params.Add("name", data.Name.ValueString())

		results, err := r.client.GetList(ctx, "/api/ipam/asn-ranges/", params)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to search for ASN range, got error: %s", err))
			return
		}

		if len(results) > 0 {
			// Found existing ASN range - update it to match desired state
			var existing ASNRangeAPIModel
			if err := json.Unmarshal(results[0], &existing); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN range response: %s", err))
				return
			}

			data.ID = types.StringValue(fmt.Sprintf("%d", existing.ID))

			// Update the existing ASN range with desired configuration
			updateData := ASNRangeAPIModel{
				Name:  data.Name.ValueString(),
				Start: data.Start.ValueInt64(),
				End:   data.End.ValueInt64(),
			}

			if !data.Slug.IsNull() {
				updateData.Slug = data.Slug.ValueString()
			} else {
				updateData.Slug = existing.Slug
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
			if len(data.Tags) > 0 {
				updateData.Tags = ConvertTagsToAPI(data.Tags)
			}

			apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/asn-ranges/%s/", data.ID.ValueString()), updateData)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update existing ASN range, got error: %s", err))
				return
			}

			var updated ASNRangeAPIModel
			if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
				resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN range response: %s", err))
				return
			}

			data.Slug = types.StringValue(updated.Slug)
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

	// Create new ASN range
	createData := ASNRangeAPIModel{
		Name:  data.Name.ValueString(),
		Start: data.Start.ValueInt64(),
		End:   data.End.ValueInt64(),
	}

	if !data.Slug.IsNull() {
		createData.Slug = data.Slug.ValueString()
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
	if len(data.Tags) > 0 {
		createData.Tags = ConvertTagsToAPI(data.Tags)
	}

	apiResp, err := r.client.Create(ctx, "/api/ipam/asn-ranges/", createData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create ASN range, got error: %s", err))
		return
	}

	var created ASNRangeAPIModel
	if err := json.Unmarshal(apiResp.Body, &created); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN range response: %s", err))
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("%d", created.ID))
	data.Slug = types.StringValue(created.Slug)
	if created.RIR != nil {
		data.RIR = types.Int64Value(int64(created.RIR.ID))
	}
	if created.Tenant != nil {
		data.Tenant = types.Int64Value(int64(created.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ASNRangeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ASNRangeResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiResp, err := r.client.Get(ctx, fmt.Sprintf("/api/ipam/asn-ranges/%s/", data.ID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read ASN range, got error: %s", err))
		return
	}

	var asnRange ASNRangeAPIModel
	if err := json.Unmarshal(apiResp.Body, &asnRange); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN range response: %s", err))
		return
	}

	data.Name = types.StringValue(asnRange.Name)
	data.Slug = types.StringValue(asnRange.Slug)
	data.Start = types.Int64Value(asnRange.Start)
	data.End = types.Int64Value(asnRange.End)
	if asnRange.RIR != nil {
		data.RIR = types.Int64Value(int64(asnRange.RIR.ID))
	}
	if asnRange.Tenant != nil {
		data.Tenant = types.Int64Value(int64(asnRange.Tenant.ID))
	}
	data.Description = types.StringValue(asnRange.Description)
	data.Tags = ConvertTagsFromAPI(asnRange.Tags)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ASNRangeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ASNRangeResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateData := ASNRangeAPIModel{
		Name:  data.Name.ValueString(),
		Start: data.Start.ValueInt64(),
		End:   data.End.ValueInt64(),
	}

	if !data.Slug.IsNull() {
		updateData.Slug = data.Slug.ValueString()
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
	if len(data.Tags) > 0 {
		updateData.Tags = ConvertTagsToAPI(data.Tags)
	}

	apiResp, err := r.client.Update(ctx, fmt.Sprintf("/api/ipam/asn-ranges/%s/", data.ID.ValueString()), updateData)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update ASN range, got error: %s", err))
		return
	}

	var updated ASNRangeAPIModel
	if err := json.Unmarshal(apiResp.Body, &updated); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse ASN range response: %s", err))
		return
	}

	data.Slug = types.StringValue(updated.Slug)
	if updated.RIR != nil {
		data.RIR = types.Int64Value(int64(updated.RIR.ID))
	}
	if updated.Tenant != nil {
		data.Tenant = types.Int64Value(int64(updated.Tenant.ID))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ASNRangeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ASNRangeResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Delete(ctx, fmt.Sprintf("/api/ipam/asn-ranges/%s/", data.ID.ValueString()))
	if err != nil {
		// Treat 404 as success since resource is already gone
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete ASN range, got error: %s", err))
		return
	}
}

func (r *ASNRangeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
