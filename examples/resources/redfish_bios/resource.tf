# settings uses the names and values the BIOS setup shows, mapped through the
# attribute registry the BMC publishes; attributes takes raw Redfish names.
# A display name shared by several attributes takes its menu path as prefix.
# reset_type is not set: changes stay pending (pending_reboot = true) until the next reboot.
resource "redfish_bios" "board" {
  redfish_server { redfish_alias = "bmc" }
  settings = {
    "SVM Enable"                                  = "Enabled"
    "IOMMU"                                       = "Enabled"
    "Server Mgmt/BMC Tools/Restore AC Power Loss" = "No change"
  }
  attributes = {
    Pbs_CoreCountControl = "Pbs_CoreCountControlDisabled"
  }
}
