data "redfish_system_boot" "system" {
  redfish_server { redfish_alias = "bmc" }
}

output "boot_order" {
  value = data.redfish_system_boot.system.boot_order
}
