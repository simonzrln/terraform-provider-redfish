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

	"terraform-provider-redfish/redfish/models"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// serverModel is any resource model carrying a redfish_server block.
type serverModel interface {
	Servers() *[]models.RedfishServer
}

// syncResource implements CRUD for settings that always exist on the BMC: Create and
// Update push the plan (sync with write=true), Read refreshes, Delete only forgets.
// sync must fill every computed attribute of m from the BMC.
type syncResource[T any, PT interface {
	*T
	serverModel
}] struct {
	p        *redfishProvider
	typeName string
	schema   schema.Schema
	sync     func(ctx context.Context, p *redfishProvider, m PT, write bool) error
}

func (r *syncResource[T, PT]) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.p = req.ProviderData.(*redfishProvider)
	}
}

func (r *syncResource[T, PT]) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.typeName
}

func (r *syncResource[T, PT]) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = r.schema
	resp.Schema.Blocks = RedfishServerResourceBlockMap()
}

func (r *syncResource[T, PT]) write(ctx context.Context, plan PT, diags interface{ AddError(string, string) }) bool {
	if err := r.sync(ctx, r.p, plan, true); err != nil {
		diags.AddError("cannot apply "+r.typeName, err.Error())
		return false
	}
	return true
}

func (r *syncResource[T, PT]) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan T
	if resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...); resp.Diagnostics.HasError() {
		return
	}
	if r.write(ctx, &plan, &resp.Diagnostics) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

func (r *syncResource[T, PT]) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state T
	if resp.Diagnostics.Append(req.State.Get(ctx, &state)...); resp.Diagnostics.HasError() {
		return
	}
	if err := r.sync(ctx, r.p, &state, false); err != nil {
		resp.Diagnostics.AddError("cannot read "+r.typeName, err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *syncResource[T, PT]) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan T
	if resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...); resp.Diagnostics.HasError() {
		return
	}
	if r.write(ctx, &plan, &resp.Diagnostics) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

// Delete leaves the BMC as it is: these settings cannot be "removed", only changed.
func (*syncResource[T, PT]) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State.RemoveResource(ctx)
}
