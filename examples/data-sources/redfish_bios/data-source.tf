data "redfish_bios" "board" {
  redfish_server { redfish_alias = "bmc" }
}

output "svm" {
  value = data.redfish_bios.board.attributes["CbsCpuSvmEnableRPL"]
}
