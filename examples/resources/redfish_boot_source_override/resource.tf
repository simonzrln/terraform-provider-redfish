# Boots once from PXE. reset_type is used only if the BMC answers with a job that needs a reboot.
resource "redfish_boot_source_override" "pxe" {
  redfish_server { redfish_alias = "bmc" }
  boot_source_override_enabled = "Once"
  boot_source_override_target  = "Pxe"
  boot_source_override_mode    = "UEFI"
  reset_type                   = "GracefulRestart"
}
