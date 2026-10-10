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

	"terraform-provider-redfish/redfish/ami"
	"terraform-provider-redfish/redfish/models"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// NewAMIFirmwareDatasource reads the firmware versions an AMI MegaRAC BMC reports.
func NewAMIFirmwareDatasource() datasource.DataSource {
	return &amiFirmwareDatasource{}
}

type amiFirmwareDatasource struct {
	p *redfishProvider
}

// Configure stores the provider.
func (d *amiFirmwareDatasource) Configure(_ context.Context, req datasource.ConfigureRequest, _ *datasource.ConfigureResponse) {
	if req.ProviderData != nil {
		d.p = req.ProviderData.(*redfishProvider)
	}
}

// Metadata returns the data source type name.
func (*amiFirmwareDatasource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "ami_firmware"
}

// Schema returns the data source schema.
func (*amiFirmwareDatasource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	version := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Description: desc + " (\"N/A\" when the board does not report it).", Computed: true}
	}
	resp.Schema = schema.Schema{
		Description: "Firmware versions of an AMI MegaRAC BMC and its host, as shown on the web UI dashboard. " +
			"Use it in a check block to assert the expected versions: inside a check block a failed read " +
			"(for example an unreachable BMC) is only a warning and does not block the apply.",
		Attributes: map[string]schema.Attribute{
			"id":                schema.StringAttribute{Computed: true},
			"bmc_version":       version("BMC firmware version, e.g. 6.09.00"),
			"bios_version":      version("BIOS version"),
			"psp_version":       version("AMD PSP firmware version"),
			"microcode_version": version("CPU microcode version"),
		},
		Blocks: RedfishServerDatasourceBlockMap(),
	}
}

// Read fetches the versions.
func (d *amiFirmwareDatasource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m models.AMIFirmware
	if resp.Diagnostics.Append(req.Config.Get(ctx, &m)...); resp.Diagnostics.HasError() {
		return
	}
	if err := readAMIFirmware(d.p, &m); err != nil {
		resp.Diagnostics.AddError("cannot read ami_firmware", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func readAMIFirmware(p *redfishProvider, m *models.AMIFirmware) error {
	return withAMI(p, &m.RedfishServer, func(c *ami.Client) error {
		var info struct {
			BMC       string `json:"BMC_fw_version"`
			BIOS      string `json:"BIOS_fw_version"`
			PSP       string `json:"PSP_fw_version"`
			Microcode string `json:"Micro_Code_version"`
		}
		if err := c.Get("/api/asrr/fw-info", &info); err != nil {
			return err
		}
		m.ID = types.StringValue("fw-info")
		m.BMCVersion = types.StringValue(info.BMC)
		m.BIOSVersion = types.StringValue(info.BIOS)
		m.PSPVersion = types.StringValue(info.PSP)
		m.MicrocodeVersion = types.StringValue(info.Microcode)
		return nil
	})
}
