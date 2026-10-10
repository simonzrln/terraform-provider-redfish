/*
Copyright (c) 2023-2025 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/


Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package models

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ProviderConfig can be used to store data from the Terraform configuration.
type ProviderConfig struct {
	Username types.String `tfsdk:"user"`
	Password types.String `tfsdk:"password"`
	Servers  types.Map    `tfsdk:"redfish_servers"`
}

// RedfishServer to configure server config for resource/datasource.
type RedfishServer struct {
	RedfishAlias types.String `tfsdk:"redfish_alias"`
	User         types.String `tfsdk:"user"`
	Password     types.String `tfsdk:"password"`
	Endpoint     types.String `tfsdk:"endpoint"`
	SslInsecure  types.Bool   `tfsdk:"ssl_insecure"`
}

// RedfishServerPure defines server config without RedfishAlias.
type RedfishServerPure struct {
	User        types.String `tfsdk:"user"`
	Password    types.String `tfsdk:"password"`
	Endpoint    types.String `tfsdk:"endpoint"`
	SslInsecure types.Bool   `tfsdk:"ssl_insecure"`
}

// Account is the state of redfish_account.
type Account struct {
	ID            types.String    `tfsdk:"id"`
	Username      types.String    `tfsdk:"username"`
	Enabled       types.Bool      `tfsdk:"enabled"`
	RoleID        types.String    `tfsdk:"role_id"`
	RedfishServer []RedfishServer `tfsdk:"redfish_server"`
}

// ManagerNetworkProtocol is the state of redfish_manager_network_protocol.
type ManagerNetworkProtocol struct {
	ID                  types.String    `tfsdk:"id"`
	ManagerID           types.String    `tfsdk:"manager_id"`
	NTPServers          types.List      `tfsdk:"ntp_servers"`
	IPMIEnabled         types.Bool      `tfsdk:"ipmi_enabled"`
	SSHEnabled          types.Bool      `tfsdk:"ssh_enabled"`
	SNMPEnabled         types.Bool      `tfsdk:"snmp_enabled"`
	NTPEnabled          types.Bool      `tfsdk:"ntp_enabled"`
	KVMIPEnabled        types.Bool      `tfsdk:"kvmip_enabled"`
	VirtualMediaEnabled types.Bool      `tfsdk:"virtual_media_enabled"`
	RedfishServer       []RedfishServer `tfsdk:"redfish_server"`
}

func (m *ManagerNetworkProtocol) field(name string) *types.Bool {
	switch name {
	case "ipmi_enabled":
		return &m.IPMIEnabled
	case "ssh_enabled":
		return &m.SSHEnabled
	case "snmp_enabled":
		return &m.SNMPEnabled
	case "ntp_enabled":
		return &m.NTPEnabled
	case "kvmip_enabled":
		return &m.KVMIPEnabled
	case "virtual_media_enabled":
		return &m.VirtualMediaEnabled
	}
	panic("unknown protocol attribute " + name)
}

// Bool returns the protocol flag for a schema attribute name.
func (m *ManagerNetworkProtocol) Bool(name string) types.Bool { return *m.field(name) }

// SetBool sets the protocol flag for a schema attribute name.
func (m *ManagerNetworkProtocol) SetBool(name string, v types.Bool) { *m.field(name) = v }

// Servers implements the provider's serverModel.
func (m *ManagerNetworkProtocol) Servers() *[]RedfishServer { return &m.RedfishServer }
