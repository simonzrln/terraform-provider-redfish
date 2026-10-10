# Unlisted fan headers are not managed. A BMC cold start resets these modes.
resource "redfish_ami_fan_mode" "fans" {
  redfish_server { redfish_alias = "bmc" }
  fans = {
    "1" = { mode = "customized" }
    "3" = { mode = "manual", duty = 60 }
    "7" = { mode = "customized" }
  }
  full_speed_on_fan_failure = true
}
