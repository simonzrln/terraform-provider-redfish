# Terraform Provider Redfish for AMI MegaRAC BMCs

A slim fork of [dell/terraform-provider-redfish](https://github.com/dell/terraform-provider-redfish)
(v1.6.2) for servers whose BMC runs **AMI MegaRAC** firmware, such as ASRock Rack
boards with an AST2600. Developed and tested against an ASRock Rack **B650D4U**
(BMC firmware 6.09, BIOS 22.10).

The Dell provider reads these BMCs fine but cannot write to them: every write first
asks for the iDRAC server generation, and BIOS changes go to a hardcoded
`/Bios/Settings` URI. This fork removes the iDRAC dependencies, keeps the standard
Redfish resources, and adds resources for what AMI exposes only through the private
REST API behind its web UI: fan modes, fan curves, fan assignments, sensor
thresholds and services.

## Resources

| Resource | API | Manages |
|---|---|---|
| `redfish_bios` | Redfish | BIOS attributes via the settings object the BMC advertises (`/Bios/SD` on AMI). Diffs against pending values; `pending_reboot` shows whether a reboot is still due. **Never reboots unless `reset_type` is set.** |
| `redfish_account` | Redfish | `enabled` and `role_id` of an existing account, matched by user name. Never creates or deletes accounts, never touches passwords. |
| `redfish_manager_network_protocol` | Redfish | IPMI, SSH, SNMP, NTP (and its servers), KVM-IP, virtual media on/off. HTTPS is left out on purpose. |
| `redfish_ami_fan_mode` | AMI web API | Mode per fan header (`default`, `customized`, `manual`), manual duty, full speed on fan failure. |
| `redfish_ami_fan_curve` | AMI web API | One open-loop table: temperature to duty steps, hysteresis, ramps. |
| `redfish_ami_fan_assignment` | AMI web API | Which temperature sensor drives which table for which fans. |
| `redfish_ami_sensor_threshold` | AMI web API | Thresholds of one IPMI sensor, e.g. the lower non-critical RPM of a fan. |
| `redfish_ami_service` | AMI web API | Enable or disable kvm, cd-media, hd-media, ssh, solssh, vnc. `web` is refused (changing it ends the session). |
| `redfish_boot_order`, `redfish_boot_source_override`, `redfish_power`, `redfish_manager_reset` | Redfish | Unchanged from upstream apart from dropping the iDRAC generation check. |

Data sources: `redfish_bios`, `redfish_system_boot`.

Conventions that differ from upstream:

* Settings always exist on the BMC, so **destroy only removes a resource from state**; it
  never resets the hardware.
* Only configured attributes are managed. Unset optional attributes are read but
  never written.
* After every write the provider reads the value back and **fails if the BMC did not
  store it**. AMI firmware often acknowledges a write it silently ignores, for example
  fan thresholds that snap to 400-RPM steps.
* A BMC cold start (AC loss) resets fan modes and thresholds. Run `terraform plan`
  regularly to catch it.

## Example

```hcl
terraform {
  required_providers {
    redfish = { source = "simonzrln/redfish" }
  }
}

provider "redfish" {
  redfish_servers = {
    bmc = { user = "admin", password = var.bmc_password, endpoint = "https://192.168.1.99", ssl_insecure = true }
  }
}

resource "redfish_bios" "board" {
  redfish_server { redfish_alias = "bmc" }
  attributes = {
    CbsCpuSvmEnableRPL  = "CbsCpuSvmEnableRPLEnabled"
    CbsCmnGnbNbIOMMURPL = "CbsCmnGnbNbIOMMURPLEnabled"
  }
}

resource "redfish_ami_fan_mode" "fans" {
  redfish_server { redfish_alias = "bmc" }
  fans = {
    "1" = { mode = "customized" }
    "3" = { mode = "manual", duty = 60 }
  }
}

resource "redfish_ami_fan_curve" "cpu" {
  redfish_server { redfish_alias = "bmc" }
  table  = 1
  points = [{ temp = 30, duty = 50 }, { temp = 60, duty = 55 }, { temp = 80, duty = 75 }, { temp = 95, duty = 100 }]
}

resource "redfish_ami_fan_assignment" "cpu" {
  redfish_server { redfish_alias = "bmc" }
  slot        = 1
  temp_sensor = "TEMP_CPU"
  table       = 1
  fans        = [1, 7]
}

resource "redfish_ami_sensor_threshold" "fan3" {
  redfish_server { redfish_alias = "bmc" }
  sensor             = "FAN3_1"
  lower_non_critical = "0"
}

resource "redfish_manager_network_protocol" "bmc" {
  redfish_server { redfish_alias = "bmc" }
  snmp_enabled = false
}

resource "redfish_account" "monitor" {
  redfish_server { redfish_alias = "bmc" }
  username = "monitor"
  enabled  = false
}
```

BIOS attribute names and values are the raw Redfish names (`CbsCpuSvmEnableRPL`),
not the setup-menu labels. Map them with the attribute registry linked from
`/redfish/v1/Registries`.

## Building

There is no registry release. Build the binary and point Terraform at it with a
filesystem mirror or `dev_overrides`:

```sh
go build -o terraform-provider-redfish .        # or scripts/docker-go.sh build ...
go test ./redfish/...                            # unit tests incl. a fake AMI BMC
```

## License

MPL-2.0, like the upstream provider. Upstream code is © Dell Inc.; changes in this
fork are listed in the git history.
