# Raw Redfish attribute names, see the attribute registry under /redfish/v1/Registries.
# reset_type is not set: changes stay pending (pending_reboot = true) until the next reboot.
resource "redfish_bios" "board" {
  redfish_server { redfish_alias = "bmc" }
  attributes = {
    CbsCpuSvmEnableRPL  = "CbsCpuSvmEnableRPLEnabled"
    CbsCmnGnbNbIOMMURPL = "CbsCmnGnbNbIOMMURPLEnabled"
  }
}
