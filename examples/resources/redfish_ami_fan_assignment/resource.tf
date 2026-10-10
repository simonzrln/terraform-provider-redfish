# TEMP_CPU drives table 1 for FAN1 and FAN7. The fans follow it only in mode "customized".
resource "redfish_ami_fan_assignment" "cpu" {
  redfish_server { redfish_alias = "bmc" }
  slot        = 1
  temp_sensor = "TEMP_CPU"
  table       = 1
  fans        = [1, 7]
}
