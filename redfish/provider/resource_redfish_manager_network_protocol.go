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
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"time"

	"terraform-provider-redfish/redfish/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stmcginnis/gofish"
)

// NewManagerNetworkProtocolResource manages which BMC network services are enabled.
func NewManagerNetworkProtocolResource() resource.Resource {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Description:   "Redfish URI of the NetworkProtocol resource.",
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"manager_id": schema.StringAttribute{
			Description:   "Manager ID; defaults to the first manager.",
			Optional:      true,
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured(), stringplanmodifier.UseStateForUnknown()},
		},
		"ntp_servers": schema.ListAttribute{
			Description:   "NTP servers.",
			ElementType:   types.StringType,
			Optional:      true,
			Computed:      true,
			PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
		},
	}
	for name, key := range protocolKeys {
		attrs[name] = schema.BoolAttribute{
			Description:   fmt.Sprintf("Whether the %s service is enabled. Unset means unmanaged.", key),
			Optional:      true,
			Computed:      true,
			PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
		}
	}
	return &syncResource[models.ManagerNetworkProtocol, *models.ManagerNetworkProtocol]{
		typeName: "manager_network_protocol",
		sync:     syncNetworkProtocol,
		schema: schema.Schema{
			Description: "Enables or disables BMC network services via the Redfish ManagerNetworkProtocol resource. " +
				"Only configured attributes are managed. Destroy only removes the resource from state.",
			Attributes: attrs,
		},
	}
}

// protocolKeys maps schema attributes to Redfish ManagerNetworkProtocol properties.
// HTTPS is left out on purpose: turning it off locks Terraform out of the BMC.
var protocolKeys = map[string]string{
	"ipmi_enabled":          "IPMI",
	"ssh_enabled":           "SSH",
	"snmp_enabled":          "SNMP", // read-only on AMI MegaRAC 6.09
	"ntp_enabled":           "NTP",
	"kvmip_enabled":         "KVMIP",
	"virtual_media_enabled": "VirtualMedia",
}

type protocol struct {
	ProtocolEnabled bool     `json:"ProtocolEnabled"`
	NTPServers      []string `json:"NTPServers,omitempty"`
}

// pendingProtocols lists the ProtocolEnabled values of payload not yet visible in current.
func pendingProtocols(payload map[string]interface{}, current map[string]protocol) []string {
	var pending []string
	for key, v := range payload {
		want, ok := v.(map[string]interface{})["ProtocolEnabled"].(bool)
		if ok && current[key].ProtocolEnabled != want {
			pending = append(pending, key)
		}
	}
	sort.Strings(pending)
	return pending
}

// readProtocols decodes only the protocol objects; the resource also holds plain
// strings such as Id and HostName.
func readProtocols(service *gofish.Service, uri string) (map[string]protocol, error) {
	var raw map[string]json.RawMessage
	if err := getJSON(service.GetClient(), uri, &raw); err != nil {
		return nil, err
	}
	out := map[string]protocol{}
	for _, key := range protocolKeys {
		var p protocol
		if b, ok := raw[key]; ok {
			if err := json.Unmarshal(b, &p); err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
		}
		out[key] = p
	}
	return out, nil
}

func networkProtocolURI(service *gofish.Service, managerID string) (string, string, error) {
	managers, err := service.Managers()
	if err != nil {
		return "", "", err
	}
	for _, m := range managers {
		if managerID == "" || m.ID == managerID {
			return m.ID, m.ODataID + "/NetworkProtocol", nil
		}
	}
	return "", "", fmt.Errorf("manager %q not found", managerID)
}

func syncNetworkProtocol(ctx context.Context, p *redfishProvider, m *models.ManagerNetworkProtocol, write bool) error {
	return withRedfish(p, &m.RedfishServer, func(service *gofish.Service) error {
		id, uri, err := networkProtocolURI(service, m.ManagerID.ValueString())
		if err != nil {
			return err
		}
		current, err := readProtocols(service, uri)
		if err != nil {
			return err
		}
		if write {
			payload := map[string]interface{}{}
			for name, key := range protocolKeys {
				want := m.Bool(name)
				if !want.IsNull() && !want.IsUnknown() && want.ValueBool() != current[key].ProtocolEnabled {
					payload[key] = map[string]interface{}{"ProtocolEnabled": want.ValueBool()}
				}
			}
			if !m.NTPServers.IsNull() && !m.NTPServers.IsUnknown() {
				var want []string
				m.NTPServers.ElementsAs(ctx, &want, false)
				if !slices.Equal(want, current["NTP"].NTPServers) {
					ntp, _ := payload["NTP"].(map[string]interface{})
					if ntp == nil {
						ntp = map[string]interface{}{}
					}
					ntp["NTPServers"] = want
					payload["NTP"] = ntp
				}
			}
			ntpOn := current["NTP"].ProtocolEnabled
			if managed(m.NTPEnabled) {
				ntpOn = m.NTPEnabled.ValueBool()
			}
			if ntp, ok := payload["NTP"].(map[string]interface{}); ok && ntp["NTPServers"] != nil && !ntpOn {
				// AMI MegaRAC answers 400 ServiceDisabled in that case.
				return fmt.Errorf("ntp_servers can only be changed while ntp_enabled = true")
			}
			if len(payload) > 0 {
				if err := patchWithETag(service.GetClient(), uri, payload); err != nil {
					return err
				}
				// AMI applies protocol changes asynchronously (SSH took a few seconds) and
				// accepts some it never applies, so poll until they show up or give up.
				for start := time.Now(); ; time.Sleep(5 * time.Second) {
					if current, err = readProtocols(service, uri); err != nil {
						return err
					}
					pending := pendingProtocols(payload, current)
					if len(pending) == 0 {
						break
					}
					if time.Since(start) > time.Minute {
						return fmt.Errorf("BMC accepted but did not apply %v after 60 s", pending)
					}
				}
			}
		}
		m.ID = types.StringValue(uri)
		m.ManagerID = types.StringValue(id)
		for name, key := range protocolKeys {
			m.SetBool(name, types.BoolValue(current[key].ProtocolEnabled))
		}
		servers := make([]attr.Value, 0, len(current["NTP"].NTPServers))
		for _, s := range current["NTP"].NTPServers {
			servers = append(servers, types.StringValue(s))
		}
		m.NTPServers = types.ListValueMust(types.StringType, servers)
		return nil
	})
}
