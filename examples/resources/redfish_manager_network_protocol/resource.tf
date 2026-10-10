# Unset protocols are not managed.
resource "redfish_manager_network_protocol" "bmc" {
  redfish_server { redfish_alias = "bmc" }
  ipmi_enabled = true
  ssh_enabled  = false
  ntp_enabled  = false
}
