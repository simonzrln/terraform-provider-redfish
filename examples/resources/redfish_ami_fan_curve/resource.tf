# Temperatures and duties must both be strictly increasing.
resource "redfish_ami_fan_curve" "cpu" {
  redfish_server { redfish_alias = "bmc" }
  table = 1
  points = [
    { temp = 30, duty = 50 }, { temp = 60, duty = 55 }, { temp = 70, duty = 65 },
    { temp = 80, duty = 75 }, { temp = 95, duty = 100 },
  ]
  hysteresis = 2
}
