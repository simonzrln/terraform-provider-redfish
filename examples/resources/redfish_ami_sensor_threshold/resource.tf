# Fan sensors store RPM in 400-RPM steps; "0" disables stall detection for FAN3.
resource "redfish_ami_sensor_threshold" "fan3" {
  redfish_server { redfish_alias = "bmc" }
  sensor             = "FAN3_1"
  lower_non_critical = "0"
}
