# A failed read inside a check block is only a warning, so an unreachable BMC does not block the apply.
check "bmc_firmware" {
  data "redfish_ami_firmware" "bmc" {
    redfish_server { redfish_alias = "bmc" }
  }

  assert {
    condition     = data.redfish_ami_firmware.bmc.bmc_version == "6.09.00"
    error_message = "BMC runs firmware ${data.redfish_ami_firmware.bmc.bmc_version}, expected 6.09.00."
  }
}
