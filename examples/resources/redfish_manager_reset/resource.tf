# Restarts the BMC on create. id is the manager Id under /redfish/v1/Managers.
resource "redfish_manager_reset" "bmc" {
  redfish_server { redfish_alias = "bmc" }
  id         = "Self"
  reset_type = "GracefulRestart"
}
