# Changelog

## Unreleased (fork of dell/terraform-provider-redfish v1.6.2)

- Removed all iDRAC-only resources, data sources and the in-tree `gofish/dell`.
- Removed the iDRAC server-generation check that blocked every write on non-Dell BMCs.
- `redfish_bios`: writes to the advertised settings object, diffs against pending
  values, new `pending_reboot`, no reboot unless `reset_type` is set.
- New: `redfish_account`, `redfish_manager_network_protocol`, `redfish_ami_fan_mode`,
  `redfish_ami_fan_curve`, `redfish_ami_fan_assignment`, `redfish_ami_sensor_threshold`,
  `redfish_ami_service`.
- Removed `redfish_user_account` and `redfish_user_account_password` (Dell slot model).

Upstream history: https://github.com/dell/terraform-provider-redfish/blob/main/CHANGELOG.md
