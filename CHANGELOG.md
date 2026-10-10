# Changelog

## 2.1.0 (2026-10-10)

- New `redfish_ami_sel_policy`: SEL storage policy (linear or circular).
- New `redfish_ami_syslog`: remote syslog forwarding of the BMC.
- New `redfish_power_restore_policy`: Redfish `PowerRestorePolicy`, read back until the
  BMC applied it (the BMC answers with an asynchronous task).
- New data source `redfish_ami_firmware`: BMC, BIOS, PSP and microcode versions.
- `redfish_bios`: new `settings` map that takes setup-menu labels and values, mapped
  through the BIOS attribute registry. `attributes` is unchanged.
- `redfish_bios`: numeric BIOS values are no longer rendered in exponent notation
  (`4294967295` instead of `4.294967295e+09`).

## 2.0.1 (2026-10-10)

Hygiene release, no functional changes.

- golangci-lint clean and enforced in CI.
- `go mod tidy`: dropped unused dependencies (terraform-plugin-sdk/v2, go-git, terraform-exec and others).
- Dependabot for Go modules and GitHub Actions.
- Provider description no longer says "Terraform Provider Redfish".
- Docs mark the resources inherited from Dell (`redfish_boot_order`, `redfish_boot_source_override`,
  `redfish_power`, `redfish_manager_reset`) as not tested on AMI firmware.
- Makefile: removed Dell's website and sweep targets and `go test -i`, added `make docs`.

## 2.0.0 (2026-10-10)

First registry release of the fork of dell/terraform-provider-redfish v1.6.2.

- Published on the Terraform Registry as `simonzrln/redfish`, plugin protocol 6.
- Tested hardware: ASRock Rack B650D4U (BMC firmware 6.09, BIOS 22.10).
- Removed all iDRAC-only resources, data sources and the in-tree `gofish/dell`.
- Removed the iDRAC server-generation check that blocked every write on non-Dell BMCs.
- `redfish_bios`: writes to the advertised settings object, diffs against pending
  values, new `pending_reboot`, no reboot unless `reset_type` is set.
- New: `redfish_account`, `redfish_manager_network_protocol`, `redfish_ami_fan_mode`,
  `redfish_ami_fan_curve`, `redfish_ami_fan_assignment`, `redfish_ami_sensor_threshold`,
  `redfish_ami_service`.
- Removed `redfish_user_account` and `redfish_user_account_password` (Dell slot model).

Versions below 2.0.0 are Dell's upstream history: https://github.com/dell/terraform-provider-redfish/blob/main/CHANGELOG.md
