package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type vaultResource struct {
	client           *Client
	archiveOnDestroy bool
}

type vaultResourceModel struct {
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	Metadata    types.Map    `tfsdk:"metadata"`
	Archived    types.Bool   `tfsdk:"archived"`
}

var _ resource.Resource = (*vaultResource)(nil)
var _ resource.ResourceWithImportState = (*vaultResource)(nil)

func NewVaultResource() resource.Resource { return &vaultResource{} }

func (r *vaultResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_vault"
}

func (r *vaultResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *providerData, got %T", req.ProviderData),
		)
		return
	}
	r.client = pd.client
	r.archiveOnDestroy = pd.archiveOnDestroy
}

func (r *vaultResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		Description: "Vault for storing credentials used by managed agents.",
		Attributes: map[string]resourceschema.Attribute{
		"id": resourceschema.StringAttribute{
			Computed: true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"display_name": resourceschema.StringAttribute{Required: true},
		"metadata":     resourceschema.MapAttribute{Optional: true, ElementType: types.StringType},
		"archived": resourceschema.BoolAttribute{
			Computed: true,
			PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
		},
	}}
}

func (r *vaultResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vaultResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	meta, d := mapFromTF(ctx, plan.Metadata)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := vaultRequestPayload{DisplayName: plan.DisplayName.ValueString(), Metadata: meta}
	var api vaultAPIModel
	if err := r.client.Post(ctx, "/v1/vaults", payload, &api); err != nil {
		resp.Diagnostics.AddError("Create vault failed", err.Error())
		return
	}
	state, diags := flattenVaultState(ctx, api)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vaultResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var api vaultAPIModel
	if err := r.client.Get(ctx, fmt.Sprintf("/v1/vaults/%s", state.ID.ValueString()), &api); err != nil {
		var nfe *NotFoundError
		if errors.As(err, &nfe) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read vault failed", err.Error())
		return
	}
	newState, diags := flattenVaultState(ctx, api)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *vaultResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan vaultResourceModel
	var state vaultResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	meta, d := mapFromTF(ctx, plan.Metadata)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := vaultRequestPayload{DisplayName: plan.DisplayName.ValueString(), Metadata: meta}
	var api vaultAPIModel
	if err := r.client.Post(ctx, fmt.Sprintf("/v1/vaults/%s", state.ID.ValueString()), payload, &api); err != nil {
		resp.Diagnostics.AddError("Update vault failed", err.Error())
		return
	}
	newState, diags := flattenVaultState(ctx, api)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *vaultResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vaultResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var err error
	if r.archiveOnDestroy {
		err = r.client.Post(ctx, fmt.Sprintf("/v1/vaults/%s/archive", state.ID.ValueString()), map[string]any{}, nil)
	} else {
		err = r.client.Delete(ctx, fmt.Sprintf("/v1/vaults/%s", state.ID.ValueString()))
	}
	if err != nil {
		resp.Diagnostics.AddError("Delete vault failed", err.Error())
	}
}

func (r *vaultResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenVaultState(ctx context.Context, api vaultAPIModel) (vaultResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	state := vaultResourceModel{ID: types.StringValue(api.ID), DisplayName: types.StringValue(api.DisplayName), Archived: types.BoolValue(api.ArchivedAt != nil)}
	meta, d := mapToTF(ctx, api.Metadata)
	diags.Append(d...)
	state.Metadata = meta
	return state, diags
}
