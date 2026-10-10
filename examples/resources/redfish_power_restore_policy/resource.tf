# Takes effect at the next AC power loss, no reboot needed. Keep the BIOS setting
# "Restore AC Power Loss" at "No change", or the BIOS overwrites this at every boot.
resource "redfish_power_restore_policy" "board" {
  redfish_server { redfish_alias = "bmc" }
  policy = "LastState"
}
