/*
Copyright (c) 2026 Simon Zürlein

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/
*/

package provider

// Resources for AMI MegaRAC settings that Redfish does not expose (fan control,
// sensor thresholds, web API services). They talk to the web UI's private API.

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strconv"

	"terraform-provider-redfish/redfish/ami"
	"terraform-provider-redfish/redfish/models"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func managed[T interface {
	IsNull() bool
	IsUnknown() bool
}](v T) bool {
	return !v.IsNull() && !v.IsUnknown()
}

var idAttr = schema.StringAttribute{
	Computed:      true,
	PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
}

func optionalInt(desc string) schema.Int64Attribute {
	return schema.Int64Attribute{
		Description: desc, Optional: true, Computed: true,
		PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
	}
}

// ---------------------------------------------------------------- fan mode

// NewAMIFanModeResource manages the per-header fan mode and manual duty.
func NewAMIFanModeResource() resource.Resource {
	return &syncResource[models.AMIFanMode, *models.AMIFanMode]{
		typeName: "ami_fan_mode",
		sync:     syncFanMode,
		schema: schema.Schema{
			Description: "Fan mode per header (default, customized = follows its fan curve, manual = fixed duty) " +
				"of an AMI MegaRAC BMC. Only listed headers are managed. A BMC cold start resets these settings.",
			Attributes: map[string]schema.Attribute{
				"id": idAttr,
				"fans": schema.MapNestedAttribute{
					Description: "Keyed by fan header number (\"1\" = FAN1).",
					Required:    true,
					NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
						"mode": schema.StringAttribute{
							Description: "default = BMC firmware default, customized = follows its fan curve, manual = fixed duty.",
							Required:    true,
							Validators:  []validator.String{stringvalidator.OneOf("default", "customized", "manual")},
						},
						"duty": schema.Int64Attribute{
							Description: "Fixed duty in percent; only for mode = manual. Unset means unmanaged.",
							Optional:    true,
						},
					}},
				},
				"full_speed_on_fan_failure": schema.BoolAttribute{
					Description: "Run all fans at full speed while any fan reports a failure " +
						"(including a reading below its lower threshold).",
					Optional: true, Computed: true,
					PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				},
			},
		},
	}
}

func syncFanMode(_ context.Context, p *redfishProvider, m *models.AMIFanMode, write bool) error {
	return withAMI(p, m.Servers(), func(c *ami.Client) error {
		var mode, manual ami.Object
		read := func() error {
			if err := c.Get("/api/settings/fan/mode", &mode); err != nil {
				return err
			}
			return c.Get("/api/settings/fan/manual", &manual)
		}
		if err := read(); err != nil {
			return err
		}
		if write {
			newMode, newManual := maps.Clone(mode), maps.Clone(manual)
			modeChanged, manualChanged := false, false
			for k, slot := range m.Fans {
				n, err := strconv.Atoi(k)
				if err != nil || n < 1 || n > mode.Int("num_of_fan") {
					return fmt.Errorf("fan header %q: must be 1..%d", k, mode.Int("num_of_fan"))
				}
				v := ami.FanModes[slot.Mode.ValueString()]
				if mode.Int("fan"+k) != v {
					newMode["fan"+k], modeChanged = v, true
				}
				if !managed(slot.Duty) {
					continue
				}
				if slot.Mode.ValueString() != "manual" {
					return fmt.Errorf("fan %s: duty is only valid with mode = manual", k)
				}
				d := int(slot.Duty.ValueInt64())
				if d < manual.Int("min_duty") || d > manual.Int("max_duty") {
					return fmt.Errorf("fan %s: duty %d outside %d..%d", k, d, manual.Int("min_duty"), manual.Int("max_duty"))
				}
				if manual.Int("duty"+k) != d {
					newManual["duty"+k], manualChanged = d, true
				}
			}
			if managed(m.FullSpeedOnFanFailure) {
				flag := mode.Int("misc_flag") &^ 1
				if m.FullSpeedOnFanFailure.ValueBool() {
					flag |= 1
				}
				if flag != mode.Int("misc_flag") {
					newMode["misc_flag"], modeChanged = flag, true
				}
			}
			// Mode first: the BMC ignores a manual duty for a header that is not in manual mode.
			if modeChanged {
				if err := c.Post("/api/settings/fan/mode", newMode, nil); err != nil {
					return err
				}
			}
			if manualChanged {
				if err := c.Post("/api/settings/fan/manual", newManual, nil); err != nil {
					return err
				}
			}
			if modeChanged || manualChanged {
				if err := read(); err != nil {
					return err
				}
				for k := range newMode {
					if ami.Object(newMode).Int(k) != mode.Int(k) {
						return fmt.Errorf("BMC did not store fan mode field %s (wanted %v, has %v)", k, newMode[k], mode[k])
					}
				}
				for k := range newManual {
					if ami.Object(newManual).Int(k) != manual.Int(k) {
						return fmt.Errorf("BMC did not store manual duty field %s (wanted %v, has %v)", k, newManual[k], manual[k])
					}
				}
			}
		}
		m.ID = types.StringValue("fan_mode")
		for k, prev := range m.Fans {
			slot := models.AMIFanSlot{Mode: types.StringValue(ami.FanModeName(mode.Int("fan" + k))), Duty: types.Int64Null()}
			// Report the duty only where it is managed, so unmanaged headers stay quiet in plans.
			if managed(prev.Duty) {
				slot.Duty = types.Int64Value(int64(manual.Int("duty" + k)))
			}
			m.Fans[k] = slot
		}
		m.FullSpeedOnFanFailure = types.BoolValue(mode.Int("misc_flag")&1 == 1)
		return nil
	})
}

// ---------------------------------------------------------------- fan curve

// NewAMIFanCurveResource manages one open-loop fan table.
func NewAMIFanCurveResource() resource.Resource {
	return &syncResource[models.AMIFanCurve, *models.AMIFanCurve]{
		typeName: "ami_fan_curve",
		sync:     syncFanCurve,
		schema: schema.Schema{
			Description: "One open-loop fan table (temperature to duty steps) of an AMI MegaRAC BMC. " +
				"Which sensor drives it and which fans follow it is set by redfish_ami_fan_assignment.",
			Attributes: map[string]schema.Attribute{
				"id": idAttr,
				"table": schema.Int64Attribute{
					Description:   "Table number.",
					Required:      true,
					Validators:    []validator.Int64{int64validator.Between(1, 8)},
					PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				},
				"points": schema.ListNestedAttribute{
					Description: "Steps in ascending order; from temp °C on the fans run at duty %. " +
						"Temperatures and duties must both be strictly increasing.",
					Required: true,
					NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
						"temp": schema.Int64Attribute{Description: "Temperature in °C from which this step applies.", Required: true},
						"duty": schema.Int64Attribute{Description: "Fan duty in percent for this step.", Required: true},
					}},
				},
				"hysteresis":         optionalInt("Falling hysteresis in °C."),
				"ramp_up_interval":   optionalInt("Ramp-up interval in seconds; 0 together with ramp_up_duty = 0 disables it."),
				"ramp_up_duty":       optionalInt("Ramp-up step in percent."),
				"ramp_down_interval": optionalInt("Ramp-down interval in seconds; 0 together with ramp_down_duty = 0 disables it."),
				"ramp_down_duty":     optionalInt("Ramp-down step in percent."),
			},
		},
	}
}

var curveExtras = []struct {
	key string
	get func(*models.AMIFanCurve) *types.Int64
}{
	{"Hysteresis", func(m *models.AMIFanCurve) *types.Int64 { return &m.Hysteresis }},
	{"RampUpInterval", func(m *models.AMIFanCurve) *types.Int64 { return &m.RampUpInterval }},
	{"RampUpDuty", func(m *models.AMIFanCurve) *types.Int64 { return &m.RampUpDuty }},
	{"RampDownInterval", func(m *models.AMIFanCurve) *types.Int64 { return &m.RampDownInterval }},
	{"RampDownDuty", func(m *models.AMIFanCurve) *types.Int64 { return &m.RampDownDuty }},
}

func syncFanCurve(_ context.Context, p *redfishProvider, m *models.AMIFanCurve, write bool) error {
	table := int(m.Table.ValueInt64())
	return withAMI(p, m.Servers(), func(c *ami.Client) error {
		var meta, row ami.Object
		read := func() error {
			// Table=1 is the customized set; without it the BMC returns factory defaults.
			var rows []ami.Object
			if err := c.Get("/api/settings/fan/openloop?Table=1", &rows); err != nil {
				return err
			}
			var err error
			if meta, err = ami.FindRow(rows, 0); err != nil {
				return err
			}
			row, err = ami.FindRow(rows, table)
			return err
		}
		if err := read(); err != nil {
			return err
		}
		entries := meta.Int("num_of_entry")
		if write {
			pts := make([]ami.CurvePoint, len(m.Points))
			for i, pt := range m.Points {
				pts[i] = ami.CurvePoint{Temp: int(pt.Temp.ValueInt64()), Duty: int(pt.Duty.ValueInt64())}
			}
			newRow := maps.Clone(row)
			if err := ami.SetCurve(newRow, pts, entries, meta.Int("min_temp"), meta.Int("max_temp"), meta.Int("min_duty"), meta.Int("max_duty")); err != nil {
				return fmt.Errorf("table %d: %w", table, err)
			}
			changed := !slices.Equal(pts, ami.Curve(row, entries))
			for _, e := range curveExtras {
				if v := *e.get(m); managed(v) && int(v.ValueInt64()) != row.Int(e.key) {
					newRow[e.key], changed = int(v.ValueInt64()), true
				}
			}
			if changed {
				if err := c.Put(fmt.Sprintf("/api/settings/fan/openloop/%d", table), newRow, nil); err != nil {
					return err
				}
				if err := read(); err != nil {
					return err
				}
				if !slices.Equal(pts, ami.Curve(row, entries)) {
					return fmt.Errorf("BMC did not store table %d: has %v", table, ami.Curve(row, entries))
				}
			}
		}
		m.ID = types.StringValue(fmt.Sprintf("openloop/%d", table))
		m.Points = nil
		for _, pt := range ami.Curve(row, entries) {
			m.Points = append(m.Points, models.AMICurvePoint{Temp: types.Int64Value(int64(pt.Temp)), Duty: types.Int64Value(int64(pt.Duty))})
		}
		for _, e := range curveExtras {
			*e.get(m) = types.Int64Value(int64(row.Int(e.key)))
		}
		return nil
	})
}

// ---------------------------------------------------------------- fan assignment

// NewAMIFanAssignmentResource manages which sensor drives which fan table for which fans.
func NewAMIFanAssignmentResource() resource.Resource {
	return &syncResource[models.AMIFanAssignment, *models.AMIFanAssignment]{
		typeName: "ami_fan_assignment",
		sync:     syncFanAssignment,
		schema: schema.Schema{
			Description: "One fan assignment slot of an AMI MegaRAC BMC: temperature sensor -> open-loop table -> fans. " +
				"Fans only follow the table while their mode is customized (redfish_ami_fan_mode).",
			Attributes: map[string]schema.Attribute{
				"id": idAttr,
				"slot": schema.Int64Attribute{
					Description:   "Assignment slot number.",
					Required:      true,
					Validators:    []validator.Int64{int64validator.AtLeast(1)},
					PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				},
				"temp_sensor": schema.StringAttribute{
					Description: "Temperature sensor name, e.g. TEMP_CPU.",
					Required:    true,
				},
				"table": schema.Int64Attribute{
					Description: "Open-loop table number.",
					Required:    true,
					Validators:  []validator.Int64{int64validator.Between(1, 8)},
				},
				"fans": schema.SetAttribute{
					Description: "Fan header numbers that follow the table.",
					ElementType: types.Int64Type,
					Required:    true,
				},
			},
		},
	}
}

func syncFanAssignment(ctx context.Context, p *redfishProvider, m *models.AMIFanAssignment, write bool) error {
	slot := int(m.Slot.ValueInt64())
	return withAMI(p, m.Servers(), func(c *ami.Client) error {
		var sensors []ami.Object
		if err := c.Get("/api/settings/fan/temp_sensors", &sensors); err != nil {
			return err
		}
		var row ami.Object
		read := func() error {
			var rows []ami.Object
			if err := c.Get("/api/settings/fan/assignment?Table=1", &rows); err != nil {
				return err
			}
			var err error
			row, err = ami.FindRow(rows, slot)
			return err
		}
		if err := read(); err != nil {
			return err
		}
		if write {
			number := -1
			for _, s := range sensors {
				if s["name"] == m.TempSensor.ValueString() {
					number = s.Int("sensor_number")
				}
			}
			if number < 0 {
				return fmt.Errorf("temperature sensor %q not found", m.TempSensor.ValueString())
			}
			var fans []int64
			if diags := m.Fans.ElementsAs(ctx, &fans, false); diags.HasError() {
				return fmt.Errorf("fans: %v", diags)
			}
			ints := make([]int, len(fans))
			for i, f := range fans {
				if f < 1 || f > 16 {
					return fmt.Errorf("fan header %d: must be 1..16", f)
				}
				ints[i] = int(f)
			}
			want := ami.Object{"TempSensor": number, "OpenLoopTable": int(m.Table.ValueInt64()), "SelectedFan": ami.FanMask(ints)}
			newRow, changed := maps.Clone(row), false
			for k := range want {
				if row.Int(k) != want.Int(k) {
					newRow[k], changed = want[k], true
				}
			}
			if changed {
				if err := c.Put(fmt.Sprintf("/api/settings/fan/assignment/%d", slot), newRow, nil); err != nil {
					return err
				}
				if err := read(); err != nil {
					return err
				}
				for k := range want {
					if row.Int(k) != want.Int(k) {
						return fmt.Errorf("BMC did not store assignment %d field %s (wanted %v, has %v)", slot, k, want[k], row[k])
					}
				}
			}
		}
		m.ID = types.StringValue(fmt.Sprintf("assignment/%d", slot))
		m.TempSensor = types.StringValue("")
		for _, s := range sensors {
			if s.Int("sensor_number") == row.Int("TempSensor") {
				m.TempSensor = types.StringValue(fmt.Sprint(s["name"]))
			}
		}
		m.Table = types.Int64Value(int64(row.Int("OpenLoopTable")))
		fans := []attr.Value{}
		for _, f := range ami.MaskFans(row.Int("SelectedFan")) {
			fans = append(fans, types.Int64Value(int64(f)))
		}
		m.Fans = types.SetValueMust(types.Int64Type, fans)
		return nil
	})
}

// ---------------------------------------------------------------- sensor threshold

var thresholdFields = []struct {
	attr, key   string
	settableBit int
	get         func(*models.AMISensorThreshold) *types.String
}{
	{"lower_non_recoverable", "lower_non_recoverable_threshold", 1 << 10, func(m *models.AMISensorThreshold) *types.String { return &m.LowerNonRecoverable }},
	{"lower_critical", "lower_critical_threshold", 1 << 9, func(m *models.AMISensorThreshold) *types.String { return &m.LowerCritical }},
	{"lower_non_critical", "lower_non_critical_threshold", 1 << 8, func(m *models.AMISensorThreshold) *types.String { return &m.LowerNonCritical }},
	{"upper_non_critical", "higher_non_critical_threshold", 1 << 11, func(m *models.AMISensorThreshold) *types.String { return &m.HigherNonCritical }},
	{"upper_critical", "higher_critical_threshold", 1 << 12, func(m *models.AMISensorThreshold) *types.String { return &m.HigherCritical }},
	{"upper_non_recoverable", "higher_non_recoverable_threshold", 1 << 13, func(m *models.AMISensorThreshold) *types.String { return &m.HigherNonRecoverable }},
}

// thresholdString renders a threshold the way it is configured: "NA" or a plain number.
func thresholdString(v interface{}) string {
	if f, ok := v.(float64); ok {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

// NewAMISensorThresholdResource manages the thresholds of one IPMI sensor.
func NewAMISensorThresholdResource() resource.Resource {
	attrs := map[string]schema.Attribute{
		"id": idAttr,
		"sensor": schema.StringAttribute{
			Description:   "Sensor name, e.g. FAN3_1.",
			Required:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
	}
	for _, f := range thresholdFields {
		attrs[f.attr] = schema.StringAttribute{
			Description: "Threshold as a number or \"NA\". Unset means unmanaged. Fan sensors store RPM in " +
				"400-RPM steps; a value the BMC rounds away fails the apply.",
			Optional: true, Computed: true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		}
	}
	return &syncResource[models.AMISensorThreshold, *models.AMISensorThreshold]{
		typeName: "ami_sensor_threshold",
		sync:     syncSensorThreshold,
		schema: schema.Schema{
			Description: "Thresholds of one IPMI sensor of an AMI MegaRAC BMC. Firmware updates and cold " +
				"starts may restore factory thresholds.",
			Attributes: attrs,
		},
	}
}

func syncSensorThreshold(_ context.Context, p *redfishProvider, m *models.AMISensorThreshold, write bool) error {
	return withAMI(p, m.Servers(), func(c *ami.Client) error {
		var sensors []ami.Object
		if err := c.Get("/api/sensors", &sensors); err != nil {
			return err
		}
		var sensor ami.Object
		for _, s := range sensors {
			if s["name"] == m.Sensor.ValueString() {
				sensor = s
			}
		}
		if sensor == nil {
			return fmt.Errorf("sensor %q not found", m.Sensor.ValueString())
		}
		uri := fmt.Sprintf("/api/settings/sensor-threshold?LUN_NUM=%d&SENSOR_NUM=%d", sensor.Int("owner_lun"), sensor.Int("sensor_number"))
		var cur ami.Object
		if err := c.Get(uri, &cur); err != nil {
			return err
		}
		if write {
			body, changed := maps.Clone(cur), false
			for _, f := range thresholdFields {
				body[f.key] = thresholdString(cur[f.key]) // the UI always sends all six as strings
				if v := *f.get(m); managed(v) && v.ValueString() != thresholdString(cur[f.key]) {
					if cur.Int("settable_flag")&f.settableBit == 0 {
						return fmt.Errorf("%s of %s is not settable on this BMC", f.attr, m.Sensor.ValueString())
					}
					body[f.key], changed = v.ValueString(), true
				}
			}
			if changed {
				body["retain_threshold_value"] = 0
				if err := c.Put("/api/settings/sensor-threshold", body, nil); err != nil {
					return err
				}
				if err := c.Get(uri, &cur); err != nil {
					return err
				}
				for _, f := range thresholdFields {
					if v := *f.get(m); managed(v) && v.ValueString() != thresholdString(cur[f.key]) {
						return fmt.Errorf("BMC stored %s = %s instead of %s (fan sensors use 400-RPM steps)",
							f.attr, thresholdString(cur[f.key]), v.ValueString())
					}
				}
			}
		}
		m.ID = types.StringValue(fmt.Sprintf("sensor/%d", sensor.Int("sensor_number")))
		for _, f := range thresholdFields {
			*f.get(m) = types.StringValue(thresholdString(cur[f.key]))
		}
		return nil
	})
}

// ---------------------------------------------------------------- services

// NewAMIServiceResource enables or disables one service of the BMC web API.
func NewAMIServiceResource() resource.Resource {
	return &syncResource[models.AMIService, *models.AMIService]{
		typeName: "ami_service",
		sync:     syncService,
		schema: schema.Schema{
			Description: "Enables or disables one AMI MegaRAC service (kvm, cd-media, hd-media, ssh, solssh, vnc). " +
				"Saving restarts the service. The web service is not manageable: changing it ends the session.",
			Attributes: map[string]schema.Attribute{
				"id": idAttr,
				"name": schema.StringAttribute{
					Description:   "Service name: kvm, cd-media, hd-media, ssh, solssh or vnc.",
					Required:      true,
					Validators:    []validator.String{stringvalidator.NoneOf("web")},
					PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				},
				"enabled": schema.BoolAttribute{Description: "Whether the service runs.", Required: true},
			},
		},
	}
}

func syncService(_ context.Context, p *redfishProvider, m *models.AMIService, write bool) error {
	return withAMI(p, m.Servers(), func(c *ami.Client) error {
		var row ami.Object
		read := func() error {
			var rows []ami.Object
			if err := c.Get("/api/settings/services", &rows); err != nil {
				return err
			}
			names := []string{}
			for _, r := range rows {
				names = append(names, fmt.Sprint(r["service_name"]))
				if r["service_name"] == m.Name.ValueString() {
					row = r
					return nil
				}
			}
			sort.Strings(names)
			return fmt.Errorf("service %q not found, have %v", m.Name.ValueString(), names)
		}
		if err := read(); err != nil {
			return err
		}
		state := 0
		if m.Enabled.ValueBool() {
			state = 1
		}
		if write && row.Int("state") != state {
			// The firmware wants these four as strings, exactly like the web UI sends its
			// form fields; any other mix fails with "Invalid Variable Type Received".
			body := maps.Clone(row)
			body["state"] = state
			for _, k := range []string{"non_secure_port", "secure_port", "time_out", "active_session"} {
				body[k] = strconv.Itoa(row.Int(k))
			}
			if err := c.Put(fmt.Sprintf("/api/settings/services/%d", row.Int("id")), body, nil); err != nil {
				return err
			}
			if err := read(); err != nil {
				return err
			}
			if row.Int("state") != state {
				return fmt.Errorf("BMC did not change service %s to state %d", m.Name.ValueString(), state)
			}
		}
		m.ID = types.StringValue(fmt.Sprintf("service/%d", row.Int("id")))
		m.Enabled = types.BoolValue(row.Int("state") == 1)
		return nil
	})
}
