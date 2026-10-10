resource "redfish_power" "system" {
  redfish_server { redfish_alias = "bmc" }
  desired_power_action = "On"
}
