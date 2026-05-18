package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/johnneerdael/terraform-provider-netskope-publisher/internal/client"
)

var (
	_ resource.Resource                = (*publisherResource)(nil)
	_ resource.ResourceWithConfigure   = (*publisherResource)(nil)
	_ resource.ResourceWithImportState = (*publisherResource)(nil)
)

type publisherResource struct {
	client *client.Client
}

func NewPublisherResource() resource.Resource {
	return &publisherResource{}
}

type publisherResourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	PublisherID types.Int64  `tfsdk:"publisher_id"`
	Name        types.String `tfsdk:"name"`
	CommonName  types.String `tfsdk:"common_name"`
	Registered  types.Bool   `tfsdk:"registered"`
	Status      types.String `tfsdk:"status"`
}

func (r *publisherResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_publisher"
}

func (r *publisherResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "An NPA publisher record in the Netskope tenant. " +
			"Create one for each publisher VM you intend to register; pair it with an " +
			"[`npa_publisher_token`](publisher_token) to get the bootstrap token.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				MarkdownDescription: "Terraform resource identifier. Equal to `publisher_id`.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					// Keep ID stable; not user-settable.
				},
			},
			"publisher_id": schema.Int64Attribute{
				MarkdownDescription: "Numeric publisher ID assigned by Netskope.",
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Publisher name as shown in the Netskope admin console.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"common_name": schema.StringAttribute{
				MarkdownDescription: "Common name reported by the publisher after first boot. Populated only when `registered = true`.",
				Computed:            true,
			},
			"registered": schema.BoolAttribute{
				MarkdownDescription: "True once the publisher has completed first-boot registration with Netskope.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current publisher status (e.g. `connected`, `not_registered`). Refreshes on read.",
				Computed:            true,
			},
		},
	}
}

func (r *publisherResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *publisherResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan publisherResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Idempotent create: if a publisher with that name already exists, adopt it.
	existing, err := r.client.FindPublisherByName(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Lookup before create failed", err.Error())
		return
	}
	var pub *client.Publisher
	if existing != nil {
		pub = existing
	} else {
		created, err := r.client.CreatePublisher(ctx, plan.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Create publisher failed", err.Error())
			return
		}
		pub = created
	}

	writeToModel(pub, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *publisherResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state publisherResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pub, err := r.client.GetPublisher(ctx, state.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read publisher failed", err.Error())
		return
	}
	if pub == nil {
		// Publisher was deleted out-of-band.
		resp.State.RemoveResource(ctx)
		return
	}

	writeToModel(pub, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *publisherResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state publisherResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueInt64()
	if plan.Name.ValueString() != state.Name.ValueString() {
		pub, err := r.client.UpdatePublisherName(ctx, id, plan.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Update publisher name failed", err.Error())
			return
		}
		writeToModel(pub, &plan)
	} else {
		// No-op update; copy state forward.
		plan.ID = state.ID
		plan.PublisherID = state.PublisherID
		plan.CommonName = state.CommonName
		plan.Registered = state.Registered
		plan.Status = state.Status
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *publisherResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state publisherResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeletePublisher(ctx, state.ID.ValueInt64()); err != nil {
		resp.Diagnostics.AddError("Delete publisher failed", err.Error())
		return
	}
}

func (r *publisherResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID",
			"Expected a numeric publisher ID; got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// writeToModel copies a Publisher into the Terraform state struct.
func writeToModel(p *client.Publisher, m *publisherResourceModel) {
	id := p.ResolvedID()
	m.ID = types.Int64Value(id)
	m.PublisherID = types.Int64Value(id)
	m.Name = types.StringValue(p.ResolvedName())
	if p.CommonName != "" {
		m.CommonName = types.StringValue(p.CommonName)
	} else {
		m.CommonName = types.StringNull()
	}
	m.Registered = types.BoolValue(p.Registered)
	if p.Status != "" {
		m.Status = types.StringValue(p.Status)
	} else {
		m.Status = types.StringNull()
	}
}
