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
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"terraform-provider-redfish/redfish/ami"
	"terraform-provider-redfish/redfish/models"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// fakeBMC emulates the parts of the AMI MegaRAC web API the provider uses, with the
// shapes captured from a B650D4U (firmware 6.09) on 2026-10-10.
type fakeBMC struct {
	mu       sync.Mutex
	objects  map[string]ami.Object   // singletons: fan/mode, fan/manual, log, log-policy, fw-info
	rows     map[string][]ami.Object // collections: openloop, assignment, services, sensors, temp_sensors
	requests []string                // "METHOD path" of every write
	bodies   map[string]ami.Object   // last body per write path
}

func newFakeBMC() *fakeBMC {
	f := &fakeBMC{bodies: map[string]ami.Object{}}
	mode := ami.Object{"num_of_fan": 16.0, "min_duty": 20.0, "max_duty": 100.0, "misc_flag": 1.0}
	manual := ami.Object{"num_of_fan": 16.0, "min_duty": 20.0, "max_duty": 100.0}
	for i := 1; i <= 16; i++ {
		mode["fan"+strconv.Itoa(i)] = 1.0
		manual["duty"+strconv.Itoa(i)] = 30.0
	}
	mode["fan3"], manual["duty3"] = 2.0, 60.0
	f.objects = map[string]ami.Object{
		"/api/settings/fan/mode": mode, "/api/settings/fan/manual": manual,
		"/api/settings/log-policy": mustObject(`{"id": 1, "policy": 0}`),
		"/api/settings/log": mustObject(`{"id": 1, "audit_log": 1, "system_log": 1, "port_type": 0, "file_size": 50000,
			"rotate_count": 0, "port": 0, "server_addr": "", "remote": 0, "local": 1}`),
		"/api/asrr/fw-info": mustObject(`{"BMC_fw_version": "6.09.00", "BIOS_fw_version": "22.10", "ME_fw_version": "N/A",
			"PSP_fw_version": "00.42.00.1E", "Micro_Code_version": "0b404035", "CPLD_version": "N/A", "BP_version": "N/A"}`),
	}
	table2 := ami.Object{"id": 2.0, "Hysteresis": 5.0, "RampUpInterval": 0.0, "RampUpDuty": 0.0, "RampDownInterval": 1.0, "RampDownDuty": 1.0}
	_ = ami.SetCurve(table2, []ami.CurvePoint{{Temp: 20, Duty: 30}, {Temp: 45, Duty: 45}, {Temp: 55, Duty: 65}, {Temp: 65, Duty: 85}, {Temp: 75, Duty: 100}}, 24, 1, 120, 20, 100)
	f.rows = map[string][]ami.Object{
		"/api/settings/fan/openloop": {
			{"id": 0.0, "num_of_open": 8.0, "num_of_entry": 24.0, "min_temp": 1.0, "max_temp": 120.0, "min_duty": 20.0, "max_duty": 100.0},
			table2,
		},
		"/api/settings/fan/assignment": {
			{"id": 0.0, "num_of_assignment": 80.0},
			{"id": 2.0, "TempSensor": 81.0, "OpenLoopTable": 2.0, "ClosedLoopTable": 0.0, "SelectedFan": 42.0},
		},
		"/api/settings/fan/temp_sensors": {
			{"id": 1.0, "sensor_number": 42.0, "name": "TEMP_CPU"},
			{"id": 2.0, "sensor_number": 81.0, "name": "TEMP_MB"},
		},
		"/api/sensors": {
			{"id": 16.0, "sensor_number": 19.0, "name": "FAN3_1", "owner_lun": 0.0, "type": "fan"},
		},
		"/api/settings/sensor-threshold": {
			{
				"id": 19.0, "sensor_number": 19.0, "name": "FAN3_1", "owner_lun": 0.0, "settable_flag": 257.0,
				"lower_non_recoverable_threshold": "NA", "lower_critical_threshold": "NA", "lower_non_critical_threshold": 400.0,
				"higher_non_critical_threshold": "NA", "higher_critical_threshold": "NA", "higher_non_recoverable_threshold": "NA",
			},
		},
		"/api/settings/services": {
			{"id": 7.0, "service_id": 256.0, "service_name": "vnc", "state": 1.0, "non_secure_port": -1.0, "secure_port": 5901.0, "time_out": 600.0, "maximum_sessions": 130.0, "active_session": 128.0},
		},
	}
	return f
}

func (f *fakeBMC) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/api/session" {
		if r.Method == http.MethodPost {
			http.SetCookie(w, &http.Cookie{Name: "QSESSIONID", Value: "s"})
			_ = json.NewEncoder(w).Encode(map[string]string{"CSRFToken": "tok"})
		}
		return
	}
	if r.Header.Get("X-CSRFTOKEN") != "tok" {
		http.Error(w, "no csrf", http.StatusUnauthorized)
		return
	}
	var body ami.Object
	if r.Method != http.MethodGet {
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		f.bodies[r.URL.Path] = body
	}
	if obj, ok := f.objects[r.URL.Path]; ok {
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			for k, v := range body {
				obj[k] = toFloat(v)
			}
		}
		_ = json.NewEncoder(w).Encode(obj)
		return
	}
	if r.URL.Path == "/api/settings/sensor-threshold" {
		f.threshold(w, r, body)
		return
	}
	base, id := r.URL.Path, -1
	if i := strings.LastIndex(base, "/"); i > 0 {
		if n, err := strconv.Atoi(base[i+1:]); err == nil {
			base, id = base[:i], n
		}
	}
	rows, ok := f.rows[base]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPut {
		row, err := ami.FindRow(rows, id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		for k, v := range body {
			row[k] = toFloat(v)
		}
	}
	_ = json.NewEncoder(w).Encode(rows)
}

// threshold stores fan thresholds in 400-RPM steps, rounding up like the real BMC.
func (f *fakeBMC) threshold(w http.ResponseWriter, r *http.Request, body ami.Object) {
	row := f.rows["/api/settings/sensor-threshold"][0]
	if r.Method == http.MethodPut {
		for k, v := range body {
			if strings.HasSuffix(k, "_threshold") && v != "NA" {
				n, _ := strconv.ParseFloat(v.(string), 64)
				row[k] = float64(int((n+399)/400) * 400)
			}
		}
	}
	_ = json.NewEncoder(w).Encode(row)
}

func mustObject(s string) ami.Object {
	var o ami.Object
	if err := json.Unmarshal([]byte(s), &o); err != nil {
		panic(err)
	}
	return o
}

func toFloat(v interface{}) interface{} {
	if s, ok := v.(string); ok {
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			return n
		}
	}
	return v
}

func fakeProvider(t *testing.T) (*redfishProvider, []models.RedfishServer, *fakeBMC) {
	t.Helper()
	f := newFakeBMC()
	srv := httptest.NewTLSServer(f)
	t.Cleanup(srv.Close)
	p := &redfishProvider{RetryConfig: DefaultRetryConfig()}
	servers := []models.RedfishServer{{
		Endpoint: types.StringValue(srv.URL), User: types.StringValue("admin"),
		Password: types.StringValue("pw"), SslInsecure: types.BoolValue(true),
	}}
	return p, servers, f
}

func TestFanModeWritesOnlyManualDuty(t *testing.T) {
	p, servers, f := fakeProvider(t)
	m := &models.AMIFanMode{RedfishServer: servers, Fans: map[string]models.AMIFanSlot{
		"1": {Mode: types.StringValue("customized"), Duty: types.Int64Null()},
		"3": {Mode: types.StringValue("manual"), Duty: types.Int64Value(61)},
	}}
	if err := syncFanMode(context.Background(), p, m, true); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 1 || f.requests[0] != "POST /api/settings/fan/manual" {
		t.Fatalf("writes = %v, want only the manual duty", f.requests)
	}
	if m.Fans["3"].Duty.ValueInt64() != 61 || !m.Fans["1"].Duty.IsNull() || !m.FullSpeedOnFanFailure.ValueBool() {
		t.Fatalf("state = %+v", m)
	}
	m.Fans["1"] = models.AMIFanSlot{Mode: types.StringValue("customized"), Duty: types.Int64Value(50)}
	if err := syncFanMode(context.Background(), p, m, true); err == nil {
		t.Fatal("duty without manual mode must fail")
	}
}

func TestFanCurvePutsPackedTable(t *testing.T) {
	p, servers, f := fakeProvider(t)
	m := &models.AMIFanCurve{
		RedfishServer: servers, Table: types.Int64Value(2), Hysteresis: types.Int64Value(4),
		Points: []models.AMICurvePoint{{Temp: types.Int64Value(30), Duty: types.Int64Value(40)}, {Temp: types.Int64Value(70), Duty: types.Int64Value(100)}},
	}
	if err := syncFanCurve(context.Background(), p, m, true); err != nil {
		t.Fatal(err)
	}
	b := f.bodies["/api/settings/fan/openloop/2"]
	if b == nil || b.Int("temp2") != 70 || b.Int("temp3") != 255 || b.Int("Hysteresis") != 4 || b.Int("RampDownDuty") != 1 {
		t.Fatalf("PUT body = %v", b)
	}
	if len(m.Points) != 2 || m.Hysteresis.ValueInt64() != 4 {
		t.Fatalf("state = %+v", m)
	}
}

func TestFanAssignmentMask(t *testing.T) {
	p, servers, f := fakeProvider(t)
	fans, _ := types.SetValueFrom(context.Background(), types.Int64Type, []int64{2, 4, 5, 6})
	m := &models.AMIFanAssignment{RedfishServer: servers, Slot: types.Int64Value(2), TempSensor: types.StringValue("TEMP_MB"), Table: types.Int64Value(2), Fans: fans}
	if err := syncFanAssignment(context.Background(), p, m, true); err != nil {
		t.Fatal(err)
	}
	if b := f.bodies["/api/settings/fan/assignment/2"]; b.Int("SelectedFan") != 58 || b.Int("TempSensor") != 81 {
		t.Fatalf("PUT body = %v", b)
	}
}

func TestSensorThresholdDetectsRounding(t *testing.T) {
	p, servers, f := fakeProvider(t)
	m := &models.AMISensorThreshold{RedfishServer: servers, Sensor: types.StringValue("FAN3_1"), LowerNonCritical: types.StringValue("200")}
	err := syncSensorThreshold(context.Background(), p, m, true)
	if err == nil || !strings.Contains(err.Error(), "instead of 200") {
		t.Fatalf("200 RPM must be reported as rounded, got %v", err)
	}
	m.LowerNonCritical = types.StringValue("0")
	if err := syncSensorThreshold(context.Background(), p, m, true); err != nil {
		t.Fatal(err)
	}
	b := f.bodies["/api/settings/sensor-threshold"]
	if b["lower_non_critical_threshold"] != "0" || b["higher_critical_threshold"] != "NA" || m.LowerNonCritical.ValueString() != "0" {
		t.Fatalf("PUT body = %v, state = %v", b, m.LowerNonCritical)
	}
	m.HigherCritical = types.StringValue("5000")
	if err := syncSensorThreshold(context.Background(), p, m, true); err == nil || !strings.Contains(err.Error(), "not settable") {
		t.Fatalf("upper_critical on a fan must be refused, got %v", err)
	}
}

func TestServiceDisableSendsFormFieldsAsStrings(t *testing.T) {
	p, servers, f := fakeProvider(t)
	m := &models.AMIService{RedfishServer: servers, Name: types.StringValue("vnc"), Enabled: types.BoolValue(false)}
	if err := syncService(context.Background(), p, m, true); err != nil {
		t.Fatal(err)
	}
	b := f.bodies["/api/settings/services/7"]
	if b.Int("state") != 0 || b["secure_port"] != "5901" || b["active_session"] != "128" || b["maximum_sessions"] != 130.0 || m.Enabled.ValueBool() {
		t.Fatalf("PUT body = %v, state = %v", b, m.Enabled)
	}
}

func TestSELPolicy(t *testing.T) {
	p, servers, f := fakeProvider(t)
	m := &models.AMISELPolicy{RedfishServer: servers}
	if err := syncSELPolicy(context.Background(), p, m, false); err != nil || m.Policy.ValueString() != "linear" || m.ID.ValueString() != "log-policy" {
		t.Fatalf("read: %v, state = %+v", err, m)
	}
	if err := syncSELPolicy(context.Background(), p, m, true); err != nil || len(f.requests) != 0 {
		t.Fatalf("no-op apply: %v, writes = %v", err, f.requests)
	}
	m.Policy = types.StringValue("circular")
	if err := syncSELPolicy(context.Background(), p, m, true); err != nil {
		t.Fatal(err)
	}
	if b := f.bodies["/api/settings/log-policy"]; len(f.requests) != 1 || f.requests[0] != "PUT /api/settings/log-policy" ||
		b.Int("policy") != 1 || b.Int("id") != 1 || m.Policy.ValueString() != "circular" {
		t.Fatalf("writes = %v, body = %v, state = %v", f.requests, b, m.Policy)
	}
	f.objects["/api/settings/log-policy"]["policy"] = 7.0
	if err := syncSELPolicy(context.Background(), p, m, false); err == nil {
		t.Fatal("an unknown policy value must fail")
	}
}

func TestSyslogMergesRemoteFields(t *testing.T) {
	p, servers, f := fakeProvider(t)
	m := &models.AMISyslog{RedfishServer: servers, RemoteEnabled: types.BoolValue(true)}
	if err := syncSyslog(context.Background(), p, m, true); err == nil || len(f.requests) != 0 {
		t.Fatalf("remote_enabled without a server must fail before any write: %v, writes = %v", err, f.requests)
	}
	m.RemoteServer, m.RemoteProtocol = types.StringValue("192.0.2.20"), types.StringValue("udp")
	// No port: the UI would silently send 514; the provider refuses, so plan and state never disagree.
	if err := syncSyslog(context.Background(), p, m, true); err == nil || len(f.requests) != 0 {
		t.Fatalf("remote_enabled without a port must fail before any write: %v", err)
	}
	m.RemotePort = types.Int64Value(514)
	for _, bad := range []string{"127.0.0.1", "localhost", "bad_host!"} {
		m.RemoteServer = types.StringValue(bad)
		if err := syncSyslog(context.Background(), p, m, true); err == nil || len(f.requests) != 0 {
			t.Fatalf("remote_server %q must be refused before any write: %v", bad, err)
		}
	}
	m.RemoteServer = types.StringValue("192.0.2.20")
	if err := syncSyslog(context.Background(), p, m, true); err != nil {
		t.Fatal(err)
	}
	b := f.bodies["/api/settings/log"]
	if len(f.requests) != 1 || f.requests[0] != "PUT /api/settings/log" || b["file_size"] != 50000.0 || b["local"] != 1.0 ||
		b["remote"] != 1.0 || b["port_type"] != 1.0 || b["port"] != 514.0 || b["server_addr"] != "192.0.2.20" {
		t.Fatalf("writes = %v, body = %v", f.requests, b)
	}
	if m.RemotePort.ValueInt64() != 514 || m.ID.ValueString() != "log" || !m.RemoteEnabled.ValueBool() {
		t.Fatalf("state = %+v", m)
	}
}

func TestSyslogReadsUnsetProtocolAsNull(t *testing.T) {
	p, servers, _ := fakeProvider(t)
	m := &models.AMISyslog{RedfishServer: servers}
	if err := syncSyslog(context.Background(), p, m, false); err != nil {
		t.Fatal(err)
	}
	if !m.RemoteProtocol.IsNull() || m.RemoteEnabled.ValueBool() || m.RemoteServer.ValueString() != "" {
		t.Fatalf("state = %+v", m)
	}
}

func TestFirmwareRead(t *testing.T) {
	p, servers, _ := fakeProvider(t)
	m := &models.AMIFirmware{RedfishServer: servers}
	if err := readAMIFirmware(p, m); err != nil {
		t.Fatal(err)
	}
	if m.BMCVersion.ValueString() != "6.09.00" || m.BIOSVersion.ValueString() != "22.10" ||
		m.PSPVersion.ValueString() != "00.42.00.1E" || m.MicrocodeVersion.ValueString() != "0b404035" {
		t.Fatalf("state = %+v", m)
	}
}

func TestSyslogNeedsSystemLogging(t *testing.T) {
	p, servers, f := fakeProvider(t)
	f.objects["/api/settings/log"]["system_log"] = 0.0
	m := &models.AMISyslog{
		RedfishServer: servers, RemoteEnabled: types.BoolValue(true), RemoteServer: types.StringValue("192.0.2.20"),
		RemotePort: types.Int64Value(514), RemoteProtocol: types.StringValue("udp"),
	}
	if err := syncSyslog(context.Background(), p, m, true); err == nil || len(f.requests) != 0 {
		t.Fatalf("forwarding with system logging off must fail before any write: %v, writes = %v", err, f.requests)
	}
}
