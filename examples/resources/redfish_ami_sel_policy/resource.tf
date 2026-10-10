# Overwrite the oldest SEL records instead of stopping when the SEL is full.
resource "redfish_ami_sel_policy" "sel" {
  redfish_server { redfish_alias = "bmc" }
  policy = "circular"
}
