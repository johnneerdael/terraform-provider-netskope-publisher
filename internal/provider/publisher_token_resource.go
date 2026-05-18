package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/johnneerdael/terraform-provider-netskope-publisher/internal/client"
)

var (
	_ resource.Resource              = (*publisherTokenResource)(nil)
	_ resource.ResourceWithConfigure = (*publisherTokenResource)(nil)
)

type publisherTokenResource struct {
	client *client.Client
}

func NewPublisherTokenResource() resource.Resource {
	return &publisherTokenResource{}
}

type publisherTokenResourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	PublisherID types.Int64  `tfsdk:"publisher_id"`
	Token       types.String `tfsdk:"token"`
}

func (r *publisherTokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_publisher_token"
}

func (r *publisherTokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A one-shot registration token for an NPA publisher. " +
			"Issued by POSTing to `/api/v2/infrastructure/publishers/{id}/registration_token`. " +
			"The token is sensitive and is consumed by `npa_publisher_wizard -token <token>` on the publisher VM " +
			"during first-boot cloud-init. To re-issue, taint this resource (force-replace).",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				MarkdownDescription: "Equal to `publisher_id`.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"publisher_id": schema.Int64Attribute{
				MarkdownDescription: "Numeric publisher ID this token belongs to. Changing it replaces the resource (issues a fresh token).",
				Required:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"token": schema.StringAttribute{
				MarkdownDescription: "The registration token. Sensitive; consumed once by the publisher wizard.",
				Computed:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *publisherTokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *publisherTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan publisherTokenResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token, err := r.client.IssueRegistrationToken(ctx, plan.PublisherID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Issue registration token failed", err.Error())
		return
	}

	plan.ID = plan.PublisherID
	plan.Token = types.StringValue(token)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *publisherTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// The API doesn't expose "get current token" — it can only issue a new one.
	// On Read we verify the parent publisher still exists; the token field
	// stays whatever's in state (treat it as opaque cached secret).
	var state publisherTokenResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pub, err := r.client.GetPublisher(ctx, state.PublisherID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read publisher (for token) failed", err.Error())
		return
	}
	if pub == nil {
		// Parent publisher gone → drop this token from state.
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *publisherTokenResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All updatable fields RequireReplace; Update is never called in practice.
	// Defensive: leave state untouched.
	resp.Diagnostics.AddError(
		"In-place updates are not supported",
		"Change to publisher_id forces replacement. Other fields are computed.")
}

func (r *publisherTokenResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// Tokens have no DELETE endpoint. Removing from state is sufficient — the
	// physical token expires server-side or is consumed by the wizard.
}
