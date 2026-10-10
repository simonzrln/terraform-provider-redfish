resource "redfish_ami_syslog" "remote" {
  redfish_server { redfish_alias = "bmc" }
  remote_enabled  = true
  remote_server   = "192.0.2.20"
  remote_port     = 514
  remote_protocol = "udp"
}
