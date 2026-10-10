# Adopts the existing account "monitor" and disables it. Destroy only removes it from state.
resource "redfish_account" "monitor" {
  redfish_server { redfish_alias = "bmc" }
  username = "monitor"
  enabled  = false
}
