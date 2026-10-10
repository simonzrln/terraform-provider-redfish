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
	"fmt"
	"time"

	"terraform-provider-redfish/redfish/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stmcginnis/gofish"
)

// How often and how long to re-read the policy after a write; vars so tests need not wait.
var (
	powerRestorePollInterval = 5 * time.Second
	powerRestorePollTimeout  = 30 * time.Second
)

// NewPowerRestorePolicyResource manages ComputerSystem.PowerRestorePolicy.
func NewPowerRestorePolicyResource() resource.Resource {
	return &syncResource[models.PowerRestorePolicy, *models.PowerRestorePolicy]{
		typeName: "power_restore_policy",
		sync:     syncPowerRestorePolicy,
		schema: schema.Schema{
			Description: "What the host does when AC power returns after a loss (Redfish ComputerSystem.PowerRestorePolicy), " +
				"written to the BMC and effective at the next AC power loss, without a reboot. " +
				"Manage it in one place only: the BIOS attribute \"Restore AC Power Loss\" (ACPLBMC000) is pushed to the BMC " +
				"by the BIOS at every boot unless it is \"No change\", and would overwrite this value. " +
				"Destroy only removes the resource from state.",
			Attributes: map[string]schema.Attribute{
				"id": schema.StringAttribute{
					Description:   "Redfish URI of the computer system.",
					Computed:      true,
					PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				},
				"policy": schema.StringAttribute{
					Description: "AlwaysOn, AlwaysOff or LastState.",
					Required:    true,
					Validators:  []validator.String{stringvalidator.OneOf("AlwaysOn", "AlwaysOff", "LastState")},
				},
				"system_id": schema.StringAttribute{
					Description: "System ID; defaults to the first system.",
					Optional:    true,
					Computed:    true,
					PlanModifiers: []planmodifier.String{
						stringplanmodifier.RequiresReplaceIfConfigured(), stringplanmodifier.UseStateForUnknown(),
					},
				},
			},
		},
	}
}

func syncPowerRestorePolicy(_ context.Context, p *redfishProvider, m *models.PowerRestorePolicy, write bool) error {
	return withRedfish(p, m.Servers(), func(service *gofish.Service) error {
		system, err := getSystemResource(service, m.SystemID.ValueString())
		if err != nil {
			return err
		}
		policy := string(system.PowerRestorePolicy)
		if want := m.Policy.ValueString(); write && want != policy {
			if err := patchWithETag(service.GetClient(), system.ODataID, map[string]string{"PowerRestorePolicy": want}); err != nil {
				return err
			}
			// Read back: a PATCH the BMC accepts is not proof that it stored the value.
			for start := time.Now(); ; time.Sleep(powerRestorePollInterval) {
				var sys struct{ PowerRestorePolicy string }
				if err := getJSON(service.GetClient(), system.ODataID, &sys); err != nil {
					return err
				}
				if policy = sys.PowerRestorePolicy; policy == want {
					break
				}
				if time.Since(start) > powerRestorePollTimeout {
					return fmt.Errorf("BMC accepted but did not store PowerRestorePolicy %s (still %s after %s)",
						want, policy, powerRestorePollTimeout)
				}
			}
		}
		m.ID = types.StringValue(system.ODataID)
		m.SystemID = types.StringValue(system.ID)
		m.Policy = types.StringValue(policy)
		return nil
	})
}
