# reset_type is used only if the BMC answers with a job that needs a reboot.
resource "redfish_boot_order" "system" {
  redfish_server { redfish_alias = "bmc" }
  boot_order = ["Boot0001", "Boot0002"]
  reset_type = "GracefulRestart"
}
