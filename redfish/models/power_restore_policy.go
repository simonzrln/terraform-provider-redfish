/*
Copyright (c) 2026 Simon Zürlein

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/
*/

package models

import "github.com/hashicorp/terraform-plugin-framework/types"

// PowerRestorePolicy is the redfish_power_restore_policy resource.
type PowerRestorePolicy struct {
	ID            types.String    `tfsdk:"id"`
	Policy        types.String    `tfsdk:"policy"`
	SystemID      types.String    `tfsdk:"system_id"`
	RedfishServer []RedfishServer `tfsdk:"redfish_server"`
}

// Servers returns the redfish_server block.
func (m *PowerRestorePolicy) Servers() *[]RedfishServer { return &m.RedfishServer }
