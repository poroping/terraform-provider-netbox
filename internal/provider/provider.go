package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/poroping/terraform-provider-netbox/internal/client"
)

// Ensure NetBoxProvider satisfies various provider interfaces.
var _ provider.Provider = &NetBoxProvider{}

// NetBoxProvider defines the provider implementation.
type NetBoxProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// NetBoxProviderModel describes the provider data model.
type NetBoxProviderModel struct {
	URL      types.String `tfsdk:"url"`
	Token    types.String `tfsdk:"token"`
	Insecure types.Bool   `tfsdk:"insecure"`
}

func (p *NetBoxProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "netbox"
	resp.Version = p.version
}

func (p *NetBoxProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for NetBox IPAM and organization management.",
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				Description: "NetBox API URL. Can also be set via NETBOX_URL environment variable.",
				Optional:    true,
			},
			"token": schema.StringAttribute{
				Description: "NetBox API authentication token. Can also be set via NETBOX_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"insecure": schema.BoolAttribute{
				Description: "Skip TLS verification. Not recommended for production use.",
				Optional:    true,
			},
		},
	}
}

func (p *NetBoxProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config NetBoxProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Configuration values can come from the HCL or environment variables
	url := os.Getenv("NETBOX_URL")
	token := os.Getenv("NETBOX_TOKEN")

	if !config.URL.IsNull() {
		url = config.URL.ValueString()
	}

	if !config.Token.IsNull() {
		token = config.Token.ValueString()
	}

	// Validate required configuration
	if url == "" {
		resp.Diagnostics.AddError(
			"Missing NetBox URL",
			"The provider cannot create the NetBox API client as there is a missing or empty value for the NetBox URL. "+
				"Set the url value in the provider configuration or use the NETBOX_URL environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if token == "" {
		resp.Diagnostics.AddError(
			"Missing NetBox API Token",
			"The provider cannot create the NetBox API client as there is a missing or empty value for the NetBox API token. "+
				"Set the token value in the provider configuration or use the NETBOX_TOKEN environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create NetBox client
	insecure := false
	if !config.Insecure.IsNull() {
		insecure = config.Insecure.ValueBool()
	}

	apiClient := client.NewClient(url, token, insecure)

	// Make the client available to data sources and resources
	resp.DataSourceData = apiClient
	resp.ResourceData = apiClient
}

func (p *NetBoxProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewTenantResource,
		NewVRFResource,
		NewRIRResource,
		NewASNResource,
		NewASNRangeResource,
		NewVLANGroupResource,
		NewVLANResource,
		NewRouteTargetResource,
		NewPrefixResource,
		NewIPRangeResource,
		NewIPAddressResource,
		NewTagResource,
	}
}

func (p *NetBoxProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewTenantDataSource,
		NewVRFDataSource,
		NewRIRDataSource,
		NewASNDataSource,
		NewASNRangeDataSource,
		NewVLANGroupDataSource,
		NewVLANDataSource,
		NewRouteTargetDataSource,
		NewPrefixDataSource,
		NewIPRangeDataSource,
		NewIPAddressDataSource,
		NewTagDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &NetBoxProvider{
			version: version,
		}
	}
}
