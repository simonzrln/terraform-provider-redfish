/*
Copyright (c) 2026 Simon Zürlein

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/
*/

package provider

// Log settings of AMI MegaRAC BMCs from the web UI's private API: SEL policy and remote syslog.

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"regexp"
	"slices"
	"strings"

	"terraform-provider-redfish/redfish/ami"
	"terraform-provider-redfish/redfish/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	selPolicyPath   = "/api/settings/log-policy"
	syslogPath      = "/api/settings/log"
	maxSyslogServer = 64
	maxPort         = 65535
)

// ---------------------------------------------------------------- SEL policy

// selPolicies are the SEL policies indexed by their value in /api/settings/log-policy.
var selPolicies = []string{"linear", "circular"}

// NewAMISELPolicyResource manages what the BMC does when the SEL is full.
func NewAMISELPolicyResource() resource.Resource {
	return &syncResource[models.AMISELPolicy, *models.AMISELPolicy]{
		typeName: "ami_sel_policy",
		sync:     syncSELPolicy,
		schema: schema.Schema{
			Description: "SEL (System Event Log) policy of an AMI MegaRAC BMC: linear stops logging when the SEL is full, " +
				"circular lets the BMC overwrite the oldest SEL records. A SEL that is already full may need a one-off clear " +
				"before new events show up; the provider does not clear it. The Redfish LogService OverWritePolicy is " +
				"not this setting.",
			Attributes: map[string]schema.Attribute{
				"id": idAttr,
				"policy": schema.StringAttribute{
					Description: "linear or circular.",
					Required:    true,
					Validators:  []validator.String{stringvalidator.OneOf(selPolicies...)},
				},
			},
		},
	}
}

func syncSELPolicy(_ context.Context, p *redfishProvider, m *models.AMISELPolicy, write bool) error {
	return withAMI(p, m.Servers(), func(c *ami.Client) error {
		var cur ami.Object
		if err := c.Get(selPolicyPath, &cur); err != nil {
			return err
		}
		if want := slices.Index(selPolicies, m.Policy.ValueString()); write && cur.Int("policy") != want {
			// The web UI saves this Backbone model (it has an id) with a PUT of the whole object.
			body := maps.Clone(cur)
			body["policy"] = want
			if err := c.Put(selPolicyPath, body, nil); err != nil {
				return err
			}
			if err := c.Get(selPolicyPath, &cur); err != nil {
				return err
			}
			if cur.Int("policy") != want {
				return fmt.Errorf("BMC did not store SEL policy %s (has %v)", m.Policy.ValueString(), cur["policy"])
			}
		}
		v, ok := cur["policy"].(float64)
		if !ok || (v != 0 && v != 1) {
			return fmt.Errorf("unexpected SEL policy %v from the BMC", cur["policy"])
		}
		m.ID = types.StringValue("log-policy")
		m.Policy = types.StringValue(selPolicies[int(v)])
		return nil
	})
}

// ---------------------------------------------------------------- remote syslog

// syslogProtocols are the remote syslog transports indexed by port_type; 0 means unset.
var syslogProtocols = []string{"", "udp", "tcp"}

// syslogFields maps the attributes onto /api/settings/log. want returns the value to
// store and whether the attribute is configured; set reads it back into the model.
var syslogFields = []struct {
	key  string
	want func(*models.AMISyslog) (interface{}, bool)
	set  func(*models.AMISyslog, ami.Object)
}{
	{
		"remote",
		func(m *models.AMISyslog) (interface{}, bool) {
			on := 0
			if m.RemoteEnabled.ValueBool() {
				on = 1
			}
			return on, managed(m.RemoteEnabled)
		},
		func(m *models.AMISyslog, o ami.Object) { m.RemoteEnabled = types.BoolValue(o.Int("remote") == 1) },
	},
	{
		"server_addr",
		func(m *models.AMISyslog) (interface{}, bool) {
			return m.RemoteServer.ValueString(), managed(m.RemoteServer)
		},
		func(m *models.AMISyslog, o ami.Object) {
			m.RemoteServer = types.StringValue(fmt.Sprint(o["server_addr"]))
		},
	},
	{
		"port",
		func(m *models.AMISyslog) (interface{}, bool) {
			return int(m.RemotePort.ValueInt64()), managed(m.RemotePort)
		},
		func(m *models.AMISyslog, o ami.Object) { m.RemotePort = types.Int64Value(int64(o.Int("port"))) },
	},
	{
		"port_type",
		func(m *models.AMISyslog) (interface{}, bool) {
			return slices.Index(syslogProtocols, m.RemoteProtocol.ValueString()), managed(m.RemoteProtocol)
		},
		func(m *models.AMISyslog, o ami.Object) {
			m.RemoteProtocol = types.StringNull()
			if t := o.Int("port_type"); t > 0 && t < len(syslogProtocols) {
				m.RemoteProtocol = types.StringValue(syslogProtocols[t])
			}
		},
	},
}

// NewAMISyslogResource manages remote syslog forwarding of the BMC.
func NewAMISyslogResource() resource.Resource {
	return &syncResource[models.AMISyslog, *models.AMISyslog]{
		typeName: "ami_syslog",
		sync:     syncSyslog,
		schema: schema.Schema{
			Description: "Remote syslog forwarding of an AMI MegaRAC BMC (Settings > Log Settings > Advanced). " +
				"Only configured attributes are written; local log file settings are left alone. tcp on this firmware " +
				"means syslog over TLS and needs a CA certificate uploaded via the web UI, which this resource does not manage.",
			Attributes: map[string]schema.Attribute{
				"id": idAttr,
				"remote_enabled": schema.BoolAttribute{
					Description: "Forward the BMC's logs to remote_server. Needs remote_server, remote_port and remote_protocol, " +
						"and system logging switched on in the web UI (the provider checks it).",
					Optional: true, Computed: true,
					PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				},
				"remote_server": schema.StringAttribute{
					Description: "Syslog server address: IP address or host name, at most 64 characters, no loopback.",
					Optional:    true, Computed: true,
					Validators:    []validator.String{stringvalidator.LengthAtMost(maxSyslogServer)},
					PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				},
				"remote_port": schema.Int64Attribute{
					Description: "Syslog server port, usually 514. Unset on the BMC reads as 0.",
					Optional:    true, Computed: true,
					Validators:    []validator.Int64{int64validator.Between(1, maxPort)},
					PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				},
				"remote_protocol": schema.StringAttribute{
					Description: "udp, or tcp (TLS with a CA certificate uploaded via the web UI). Null while unset on the BMC.",
					Optional:    true, Computed: true,
					Validators:    []validator.String{stringvalidator.OneOf(syslogProtocols[1:]...)},
					PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				},
			},
		},
	}
}

func syncSyslog(_ context.Context, p *redfishProvider, m *models.AMISyslog, write bool) error {
	return withAMI(p, m.Servers(), func(c *ami.Client) error {
		var cur ami.Object
		if err := c.Get(syslogPath, &cur); err != nil {
			return err
		}
		if write {
			var err error
			if cur, err = writeSyslog(c, m, cur); err != nil {
				return err
			}
		}
		m.ID = types.StringValue("log")
		for _, f := range syslogFields {
			f.set(m, cur)
		}
		return nil
	})
}

// writeSyslog PUTs cur with the configured attributes merged in, if anything changed,
// and returns the object the BMC stored.
func writeSyslog(c *ami.Client, m *models.AMISyslog, cur ami.Object) (ami.Object, error) {
	body, changed := maps.Clone(cur), false
	for _, f := range syslogFields {
		if v, ok := f.want(m); ok && fmt.Sprint(v) != fmt.Sprint(cur[f.key]) {
			body[f.key], changed = v, true
		}
	}
	if !changed {
		return cur, nil
	}
	// The web UI's validation rules for the advanced log form.
	if body.Int("remote") == 1 {
		if fmt.Sprint(body["server_addr"]) == "" {
			return nil, errors.New("remote_enabled needs remote_server")
		}
		if t := body.Int("port_type"); t != 1 && t != 2 {
			return nil, errors.New("remote_enabled needs remote_protocol")
		}
		if body.Int("port") == 0 {
			return nil, errors.New("remote_enabled needs remote_port")
		}
		if err := checkSyslogServer(fmt.Sprint(body["server_addr"])); err != nil {
			return nil, err
		}
		// The web UI hides all remote fields while system logging is off; nothing is forwarded then.
		if body.Int("system_log") != 1 {
			return nil, errors.New("remote_enabled needs system logging, which is off on the BMC; switch it on in the web UI")
		}
	}
	if body.Int("system_log") == 1 && body.Int("remote") == 0 && body.Int("local") == 0 {
		return nil, errors.New("the BMC needs local or remote logging while system logging is on; enable local logging in the web UI first")
	}
	if err := c.Put(syslogPath, body, nil); err != nil {
		return nil, err
	}
	var got ami.Object
	if err := c.Get(syslogPath, &got); err != nil {
		return nil, err
	}
	for _, f := range syslogFields {
		if fmt.Sprint(body[f.key]) != fmt.Sprint(got[f.key]) {
			return nil, fmt.Errorf("BMC did not store syslog field %s (wanted %v, has %v)", f.key, body[f.key], got[f.key])
		}
	}
	return got, nil
}

// syslogHost is the web UI's host name rule for server_addr.
var syslogHost = regexp.MustCompile(`^(?i)([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)*[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.?$`)

// checkSyslogServer rejects addresses the web UI would refuse: loopback, unspecified, malformed names.
func checkSyslogServer(addr string) error {
	if ip := net.ParseIP(addr); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.Equal(net.IPv4bcast) {
			return fmt.Errorf("remote_server %q is not a usable syslog address", addr)
		}
		return nil
	}
	if !syslogHost.MatchString(addr) || strings.EqualFold(strings.TrimSuffix(addr, "."), "localhost") {
		return fmt.Errorf("remote_server %q is neither an IP address nor a host name", addr)
	}
	return nil
}
