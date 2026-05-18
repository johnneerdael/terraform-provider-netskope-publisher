package provider

import (
	"context"
	"os"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/johnneerdael/terraform-provider-netskope-publisher/internal/client"
)

// Provider type name in HCL. Chosen short + distinct from the official
// `netskope` provider so the two can coexist in one root module without aliasing.
const ProviderTypeName = "npa"

// Compile-time interface assertion.
var _ provider.Provider = (*npaProvider)(nil)

type npaProvider struct {
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &npaProvider{version: version}
	}
}

func (p *npaProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = ProviderTypeName
	resp.Version = p.version
}

type npaProviderConfig struct {
	TenantURL types.String `tfsdk:"tenant_url"`
	APIToken  types.String `tfsdk:"api_token"`
}

func (p *npaProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The `npa` provider manages Netskope Private Access publisher records and registration tokens. " +
			"It only covers the NPA publisher endpoints needed to provision and register publishers. " +
			"For the full Netskope API surface use the official [`netskopeoss/netskope`](https://registry.terraform.io/providers/netskopeoss/netskope) provider instead.",
		Attributes: map[string]schema.Attribute{
			"tenant_url": schema.StringAttribute{
				MarkdownDescription: "Netskope tenant URL, e.g. `https://tenant.goskope.com`. " +
					"Must start with `https://`. Falls back to `NETSKOPE_TENANT_URL` environment variable.",
				Optional: true,
			},
			"api_token": schema.StringAttribute{
				MarkdownDescription: "Netskope NPA API token with publisher read/write scope. " +
					"Falls back to `NETSKOPE_API_TOKEN` environment variable.",
				Optional:  true,
				Sensitive: true,
			},
		},
	}
}

func (p *npaProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg npaProviderConfig
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tenantURL := cfg.TenantURL.ValueString()
	if tenantURL == "" {
		tenantURL = os.Getenv("NETSKOPE_TENANT_URL")
	}
	apiToken := cfg.APIToken.ValueString()
	if apiToken == "" {
		apiToken = os.Getenv("NETSKOPE_API_TOKEN")
	}

	if tenantURL == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("tenant_url"),
			"Missing tenant_url",
			"Set `tenant_url` in the provider block or export NETSKOPE_TENANT_URL.",
		)
	} else if matched, _ := regexp.MatchString(`^https://`, tenantURL); !matched {
		resp.Diagnostics.AddAttributeError(
			path.Root("tenant_url"),
			"Invalid tenant_url",
			"tenant_url must start with https://",
		)
	}
	if apiToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing api_token",
			"Set `api_token` in the provider block or export NETSKOPE_API_TOKEN.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c := client.New(tenantURL, apiToken)
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *npaProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewPublisherResource,
		NewPublisherTokenResource,
	}
}

func (p *npaProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
