resource "redfish_ami_service" "vnc" {
  redfish_server { redfish_alias = "bmc" }
  name    = "vnc"
  enabled = false
}
