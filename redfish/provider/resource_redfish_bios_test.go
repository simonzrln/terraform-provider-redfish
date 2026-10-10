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
	"maps"
	"strings"
	"testing"

	"terraform-provider-redfish/redfish/models"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// applyBios runs the resource's apply against the fake like a Create would: attributes
// nil means unset in config (unknown in the plan), settings nil means null.
func applyBios(t *testing.T, attributes, settings map[string]string) (*fakeRedfish, *models.Bios, *BiosResource, error) {
	t.Helper()
	p, servers, f := fakeRedfishProvider(t)
	plan := &models.Bios{
		RedfishServer: servers, ID: types.StringUnknown(), SystemID: types.StringUnknown(), PendingReboot: types.BoolUnknown(),
		Attributes: types.MapUnknown(types.StringType), Settings: types.MapNull(types.StringType),
		SettingsApplyTime: types.StringValue("OnReset"), ResetType: types.StringNull(),
		ResetTimeout: types.Int64Value(defaultBiosConfigServerResetTimeout), JobTimeout: types.Int64Value(defaultBiosConfigJobTimeout),
	}
	if attributes != nil {
		plan.Attributes, _ = types.MapValueFrom(context.Background(), types.StringType, attributes)
	}
	if settings != nil {
		plan.Settings, _ = types.MapValueFrom(context.Background(), types.StringType, settings)
	}
	r := &BiosResource{p: p, ctx: context.Background()}
	api, err := NewConfig(p, &servers)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(api.Logout)
	state, diags := r.updateRedfishDellBiosAttributes(context.Background(), api.Service, plan)
	if diags.HasError() {
		return f, nil, r, fmt.Errorf("%s: %s", diags.Errors()[0].Summary(), diags.Errors()[0].Detail())
	}
	return f, state, r, nil
}

func stringMap(t *testing.T, m types.Map) map[string]string {
	t.Helper()
	out := map[string]string{}
	if d := m.ElementsAs(context.Background(), &out, false); d.HasError() {
		t.Fatal(d)
	}
	return out
}

func TestBiosSettingsByDisplayName(t *testing.T) {
	settings := map[string]string{
		"Restore AC Power Loss": "Power Off",
		"Core Count Control":    "Enabled", // registry: " Core Count Control" = "Enabled "
		"SVM Enable":            " Disabled ",
		"Advanced/Serial Port Console Redirection/COM1/Terminal Type": "VT-UTF8",
		"Setup Prompt Timeout": "5",
	}
	f, state, _, err := applyBios(t, nil, settings)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"ACPLBMC000": "ACPLBMC000PowerOff", "Pbs_CoreCountControl": "Pbs_CoreCountControlEnabled",
		"CbsCpuSvmEnableRPL": "CbsCpuSvmEnableRPLDisabled", "TER012": "TER012VTUTF8", "SETUP003": 5.0,
	}
	if got := f.patches[fakeBiosSD]["Attributes"].(fakeObj); !maps.Equal(got, want) {
		t.Fatalf("PATCH Attributes = %v, want %v", got, want)
	}
	if got := stringMap(t, state.Settings); !maps.Equal(got, settings) {
		t.Fatalf("settings in state = %v, want the configured spelling %v", got, settings)
	}
	if state.Attributes.IsNull() || len(state.Attributes.Elements()) != 0 || !state.PendingReboot.ValueBool() {
		t.Fatalf("attributes = %v (want empty, not every attribute), pending_reboot = %v", state.Attributes, state.PendingReboot)
	}
	if f.gzipped == 0 {
		t.Fatal("the fake served nothing gzip-compressed: transparent decompression is untested")
	}
}

func TestBiosSettingsReadBack(t *testing.T) {
	f, state, r, err := applyBios(t, nil, map[string]string{"SVM Enable": "Enabled", "IOMMU": "Enabled"})
	if err != nil {
		t.Fatal(err)
	}
	if f.patches[fakeBiosSD] != nil || state.PendingReboot.ValueBool() {
		t.Fatalf("values already set: PATCH %v, pending_reboot %v", f.patches[fakeBiosSD], state.PendingReboot)
	}
	sd := f.docs[fakeBiosSD]["Attributes"].(fakeObj)
	sd["CbsCpuSvmEnableRPL"], sd["CbsCmnGnbNbIOMMURPL"] = "CbsCpuSvmEnableRPLAuto", "SomethingNew"
	api, err := NewConfig(r.p, &state.RedfishServer)
	if err != nil {
		t.Fatal(err)
	}
	defer api.Logout()
	if err := r.readRedfishDellBiosAttributes(api.Service, state, nil); err != nil {
		t.Fatal(err)
	}
	got := stringMap(t, state.Settings)
	if got["SVM Enable"] != "Auto" || got["IOMMU"] != "SomethingNew" || !state.PendingReboot.ValueBool() {
		t.Fatalf("read back %v, pending_reboot %v; want display value, raw value for unknown ones, pending", got, state.PendingReboot)
	}
}

func TestBiosSettingsErrors(t *testing.T) {
	for _, c := range []struct {
		attributes, settings map[string]string
		want                 string
	}{
		{nil, map[string]string{"Console Redirection": "Enabled"}, "TER001 (Advanced/Serial Port Console Redirection/Console Redirection), TER002"},
		{nil, map[string]string{"Advanced/Serial Port Console Redirection/Console Redirection": "Enabled"}, "set the attribute by name in attributes"},
		{nil, map[string]string{"Terminal Type": "VT100"}, "use \"<menu path>/<display name>\""},
		{nil, map[string]string{"SVM Enable": "On"}, `"On" is not one of ["Enabled" "Disabled" "Auto"]`},
		{nil, map[string]string{"No Such Setting": "x"}, "no attribute has this display name"},
		{nil, map[string]string{"Supervisor Password": "secret"}, "password"},
		{map[string]string{"ACPLBMC000": "ACPLBMC000PowerOn"}, map[string]string{"Restore AC Power Loss": "Power On"}, "which attributes also sets"},
		{nil, map[string]string{"Restore AC Power Loss": "Power On", "Server Mgmt/BMC Tools/Restore AC Power Loss": "Power Off"}, "another settings key"},
	} {
		f, _, _, err := applyBios(t, c.attributes, c.settings)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: got %v, want %q", c.settings, err, c.want)
		}
		if f.patches[fakeBiosSD] != nil {
			t.Errorf("%v: PATCH sent despite the error", c.settings)
		}
	}
}

func TestBiosSettingsNotStored(t *testing.T) {
	p, servers, f := fakeRedfishProvider(t)
	f.frozen[fakeBiosSD] = true
	plan := &models.Bios{
		RedfishServer: servers, SystemID: types.StringNull(), Attributes: types.MapUnknown(types.StringType),
		Settings: types.MapValueMust(types.StringType, map[string]attr.Value{"SVM Enable": types.StringValue("Disabled")}),
	}
	api, err := NewConfig(p, &servers)
	if err != nil {
		t.Fatal(err)
	}
	defer api.Logout()
	_, diags := (&BiosResource{p: p, ctx: context.Background()}).updateRedfishDellBiosAttributes(context.Background(), api.Service, plan)
	if !diags.HasError() || !strings.Contains(diags.Errors()[0].Detail(), "did not store") {
		t.Fatalf("a setting the BMC ignores must fail, got %v", diags)
	}
}

// Without settings the resource must behave exactly as in 2.0: same PATCH, same state,
// and the registry is never fetched.
func TestBiosAttributesOnlyUnchanged(t *testing.T) {
	f, state, r, err := applyBios(t, map[string]string{"CbsCpuSvmEnableRPL": "CbsCpuSvmEnableRPLDisabled"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.patches[fakeBiosSD]; len(got) != 1 || !maps.Equal(got["Attributes"].(fakeObj), fakeObj{"CbsCpuSvmEnableRPL": "CbsCpuSvmEnableRPLDisabled"}) {
		t.Fatalf("PATCH body = %v", got)
	}
	if got := stringMap(t, state.Attributes); len(got) != 1 || got["CbsCpuSvmEnableRPL"] != "CbsCpuSvmEnableRPLDisabled" {
		t.Fatalf("attributes = %v", got)
	}
	if !state.Settings.IsNull() || !state.PendingReboot.ValueBool() || state.SystemID.ValueString() != "Self" {
		t.Fatalf("settings = %v, pending_reboot = %v, system_id = %v", state.Settings, state.PendingReboot, state.SystemID)
	}
	if f.requested("GET", "/redfish/v1/Registries/"+fakeRegistry) || f.requested("GET", fakeRegistryJ) {
		t.Fatal("registry fetched without settings")
	}

	// Neither attributes nor settings configured: state holds every attribute, as before.
	state.Attributes = types.MapUnknown(types.StringType)
	api, err := NewConfig(r.p, &state.RedfishServer)
	if err != nil {
		t.Fatal(err)
	}
	defer api.Logout()
	if err := r.readRedfishDellBiosAttributes(api.Service, state, nil); err != nil {
		t.Fatal(err)
	}
	if n := len(state.Attributes.Elements()); n != len(f.docs[fakeBios]["Attributes"].(fakeObj)) {
		t.Fatalf("attributes in state = %d, want all", n)
	}
}

func TestBiosValueString(t *testing.T) {
	for in, want := range map[interface{}]string{4294967295.0: "4294967295", 5.0: "5", 1.5: "1.5", "Enabled": "Enabled", true: "true"} {
		if got := biosValueString(in); got != want {
			t.Errorf("biosValueString(%v) = %q, want %q", in, got, want)
		}
	}
}
