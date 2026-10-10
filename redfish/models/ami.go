/*
Copyright (c) 2026 Simon Zürlein

Licensed under the Mozilla Public License Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://mozilla.org/MPL/2.0/
*/

package models

import "github.com/hashicorp/terraform-plugin-framework/types"

// AMIFanMode is the state of redfish_ami_fan_mode.
type AMIFanMode struct {
	ID                    types.String          `tfsdk:"id"`
	Fans                  map[string]AMIFanSlot `tfsdk:"fans"`
	FullSpeedOnFanFailure types.Bool            `tfsdk:"full_speed_on_fan_failure"`
	RedfishServer         []RedfishServer       `tfsdk:"redfish_server"`
}

// AMIFanSlot is one fan header in redfish_ami_fan_mode.
type AMIFanSlot struct {
	Mode types.String `tfsdk:"mode"`
	Duty types.Int64  `tfsdk:"duty"`
}

// AMIFanCurve is the state of redfish_ami_fan_curve.
type AMIFanCurve struct {
	ID               types.String    `tfsdk:"id"`
	Table            types.Int64     `tfsdk:"table"`
	Points           []AMICurvePoint `tfsdk:"points"`
	Hysteresis       types.Int64     `tfsdk:"hysteresis"`
	RampUpInterval   types.Int64     `tfsdk:"ramp_up_interval"`
	RampUpDuty       types.Int64     `tfsdk:"ramp_up_duty"`
	RampDownInterval types.Int64     `tfsdk:"ramp_down_interval"`
	RampDownDuty     types.Int64     `tfsdk:"ramp_down_duty"`
	RedfishServer    []RedfishServer `tfsdk:"redfish_server"`
}

// AMICurvePoint is one step of a fan curve.
type AMICurvePoint struct {
	Temp types.Int64 `tfsdk:"temp"`
	Duty types.Int64 `tfsdk:"duty"`
}

// AMIFanAssignment is the state of redfish_ami_fan_assignment.
type AMIFanAssignment struct {
	ID            types.String    `tfsdk:"id"`
	Slot          types.Int64     `tfsdk:"slot"`
	TempSensor    types.String    `tfsdk:"temp_sensor"`
	Table         types.Int64     `tfsdk:"table"`
	Fans          types.Set       `tfsdk:"fans"`
	RedfishServer []RedfishServer `tfsdk:"redfish_server"`
}

// AMISensorThreshold is the state of redfish_ami_sensor_threshold.
type AMISensorThreshold struct {
	ID                   types.String    `tfsdk:"id"`
	Sensor               types.String    `tfsdk:"sensor"`
	LowerNonRecoverable  types.String    `tfsdk:"lower_non_recoverable"`
	LowerCritical        types.String    `tfsdk:"lower_critical"`
	LowerNonCritical     types.String    `tfsdk:"lower_non_critical"`
	HigherNonCritical    types.String    `tfsdk:"upper_non_critical"`
	HigherCritical       types.String    `tfsdk:"upper_critical"`
	HigherNonRecoverable types.String    `tfsdk:"upper_non_recoverable"`
	RedfishServer        []RedfishServer `tfsdk:"redfish_server"`
}

// AMIService is the state of redfish_ami_service.
type AMIService struct {
	ID            types.String    `tfsdk:"id"`
	Name          types.String    `tfsdk:"name"`
	Enabled       types.Bool      `tfsdk:"enabled"`
	RedfishServer []RedfishServer `tfsdk:"redfish_server"`
}

// AMISELPolicy is the state of redfish_ami_sel_policy.
type AMISELPolicy struct {
	ID            types.String    `tfsdk:"id"`
	Policy        types.String    `tfsdk:"policy"`
	RedfishServer []RedfishServer `tfsdk:"redfish_server"`
}

// AMISyslog is the state of redfish_ami_syslog.
type AMISyslog struct {
	ID             types.String    `tfsdk:"id"`
	RemoteEnabled  types.Bool      `tfsdk:"remote_enabled"`
	RemoteServer   types.String    `tfsdk:"remote_server"`
	RemotePort     types.Int64     `tfsdk:"remote_port"`
	RemoteProtocol types.String    `tfsdk:"remote_protocol"`
	RedfishServer  []RedfishServer `tfsdk:"redfish_server"`
}

// AMIFirmware is the state of the redfish_ami_firmware data source.
type AMIFirmware struct {
	ID               types.String    `tfsdk:"id"`
	BMCVersion       types.String    `tfsdk:"bmc_version"`
	BIOSVersion      types.String    `tfsdk:"bios_version"`
	PSPVersion       types.String    `tfsdk:"psp_version"`
	MicrocodeVersion types.String    `tfsdk:"microcode_version"`
	RedfishServer    []RedfishServer `tfsdk:"redfish_server"`
}

// Servers implements the provider's serverModel.
func (m *AMIFanMode) Servers() *[]RedfishServer { return &m.RedfishServer }

// Servers implements the provider's serverModel.
func (m *AMIFanCurve) Servers() *[]RedfishServer { return &m.RedfishServer }

// Servers implements the provider's serverModel.
func (m *AMIFanAssignment) Servers() *[]RedfishServer { return &m.RedfishServer }

// Servers implements the provider's serverModel.
func (m *AMISensorThreshold) Servers() *[]RedfishServer { return &m.RedfishServer }

// Servers implements the provider's serverModel.
func (m *AMIService) Servers() *[]RedfishServer { return &m.RedfishServer }

// Servers implements the provider's serverModel.
func (m *AMISELPolicy) Servers() *[]RedfishServer { return &m.RedfishServer }

// Servers implements the provider's serverModel.
func (m *AMISyslog) Servers() *[]RedfishServer { return &m.RedfishServer }
