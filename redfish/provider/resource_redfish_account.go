/*
Copyright (c) 2026 Simon Zürlein

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/
*/

package provider

import (
	"context"
	"errors"
	"fmt"

	"terraform-provider-redfish/redfish/models"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stmcginnis/gofish"
	"github.com/stmcginnis/gofish/redfish"
)

var _ resource.Resource = &accountResource{}

// NewAccountResource manages Enabled and RoleId of an existing BMC account.
func NewAccountResource() resource.Resource { return &accountResource{} }

type accountResource struct{ p *redfishProvider }

var errAccountNotFound = errors.New("account not found")

func (r *accountResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.p = req.ProviderData.(*redfishProvider)
	}
}

func (*accountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "account"
}

func (*accountResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Adopts an existing account of the Redfish AccountService by user name and manages whether it is " +
			"enabled and its role. It never creates or deletes accounts and never touches passwords: destroying the " +
			"resource only removes it from state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Redfish URI of the account.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"username": schema.StringAttribute{
				Description:   "User name of the existing account.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled": schema.BoolAttribute{
				Description:   "Whether the account may log in.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"role_id": schema.StringAttribute{
				Description:   "Role of the account, e.g. Administrator, Operator, ReadOnly.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
		Blocks: RedfishServerResourceBlockMap(),
	}
}

func findAccount(service *gofish.Service, username string) (*redfish.ManagerAccount, error) {
	as, err := service.AccountService()
	if err != nil {
		return nil, err
	}
	accounts, err := as.Accounts()
	if err != nil {
		return nil, err
	}
	for _, a := range accounts {
		if a.UserName == username {
			return a, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", errAccountNotFound, username)
}

// apply patches the fields set in plan that differ, then refreshes plan from the BMC.
func (r *accountResource) apply(plan *models.Account) error {
	return withRedfish(r.p, &plan.RedfishServer, func(service *gofish.Service) error {
		a, err := findAccount(service, plan.Username.ValueString())
		if err != nil {
			return err
		}
		payload := map[string]interface{}{}
		if !plan.Enabled.IsUnknown() && !plan.Enabled.IsNull() && plan.Enabled.ValueBool() != a.Enabled {
			payload["Enabled"] = plan.Enabled.ValueBool()
		}
		if !plan.RoleID.IsUnknown() && !plan.RoleID.IsNull() && plan.RoleID.ValueString() != a.RoleID {
			payload["RoleId"] = plan.RoleID.ValueString()
		}
		if len(payload) > 0 {
			if err := patchWithETag(service.GetClient(), a.ODataID, payload); err != nil {
				return err
			}
			if a, err = findAccount(service, plan.Username.ValueString()); err != nil {
				return err
			}
		}
		plan.ID = types.StringValue(a.ODataID)
		plan.Enabled = types.BoolValue(a.Enabled)
		plan.RoleID = types.StringValue(a.RoleID)
		return nil
	})
}

func (r *accountResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan models.Account
	if resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...); resp.Diagnostics.HasError() {
		return
	}
	if err := r.apply(&plan); err != nil {
		resp.Diagnostics.AddError("cannot adopt account", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accountResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state models.Account
	if resp.Diagnostics.Append(req.State.Get(ctx, &state)...); resp.Diagnostics.HasError() {
		return
	}
	err := withRedfish(r.p, &state.RedfishServer, func(service *gofish.Service) error {
		a, err := findAccount(service, state.Username.ValueString())
		if err != nil {
			return err
		}
		state.ID = types.StringValue(a.ODataID)
		state.Enabled = types.BoolValue(a.Enabled)
		state.RoleID = types.StringValue(a.RoleID)
		return nil
	})
	if errors.Is(err, errAccountNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("cannot read account", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *accountResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan models.Account
	if resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...); resp.Diagnostics.HasError() {
		return
	}
	if err := r.apply(&plan); err != nil {
		resp.Diagnostics.AddError("cannot update account", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only forgets the account; deleting BMC users is out of scope on purpose.
func (*accountResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State.RemoveResource(ctx)
}
