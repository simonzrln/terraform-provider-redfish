# Roadmap

Ideas for the next releases, ordered by value for a homelab running an AMI MegaRAC
board (developed on an ASRock Rack B650D4U, BMC firmware 6.09). Nothing here is
promised. Items marked **(2.1)** shipped in 2.1.0.

## New settings

### High value

1. **SEL storage policy (2.1).** Circular instead of linear, via the web API
   (`/api/settings/log-policy`). With the linear default a full SEL silently stops
   recording, and the SEL is where watchdog resets and power events are proven.
2. **Remote syslog (2.1).** Send the BMC's audit and system log to a syslog server
   (`/api/settings/log`), so fan, watchdog and power events end up next to the host's
   own logs.
3. **Power restore policy on the BMC (2.1).** Redfish `ComputerSystem.PowerRestorePolicy`
   (`AlwaysOn` / `AlwaysOff` / `LastState`). Today only the BIOS attribute exists,
   which takes effect after the next reboot; the BMC property applies at once and
   shows the real state in plans.
4. **Account security.** Lockout threshold and duration, minimum password length
   (Redfish `AccountService` plus the vendor password policy of the web API, which
   has no lockout by default).
5. **Time zone.** `/api/settings/date-time`; NTP itself is already covered by
   `redfish_manager_network_protocol`.
6. **Session timeouts.** Extend `redfish_ami_service` with `time_out` (web, KVM, SSH),
   respecting the firmware's rules (multiple of 60, per-service ranges).

### Complete fan control

7. **Global fan settings.** Default mode, minimum and maximum duty, full speed during
   firmware updates (`/api/settings/fan/mode`).
8. **Express "empty".** Open-loop tables and assignment slots that are unused, so a
   plan also checks that they stay empty; manual duty for every header, not only for
   headers in manual mode.
9. **Closed-loop tables** (`/api/settings/fan/closedloop`).

### Later, with safeguards

10. **BMC network.** Host name, IPv6 on/off, VLAN tag (Redfish `EthernetInterfaces`).
    A wrong value cuts the BMC off the network; needs a pre-write reachability check
    or an explicit opt-in.
11. **BMC firewall rules** (allow lists by IP range). Same lockout risk as 10.
12. **TLS certificate upload,** so `ssl_insecure` can go away. Needs a renewal story
    for short-lived certificates (90 days with Let's Encrypt).

## Using the provider

13. **BIOS attributes by display name (2.1).** Write `"Restore AC Power Loss" = "Power On"`
    instead of `ACPLBMC000 = "ACPLBMC000PowerOn"`, mapped through the BIOS attribute
    registry the BMC publishes. The biggest readability win for real configs.
14. **Plan-time validation.** Curves must be strictly increasing, `duty` only with
    `mode = "manual"`, threshold values in 400-RPM steps for fans: report these in
    `ValidateConfig` instead of failing during apply.
15. **One web API session per run** instead of one login per resource. Faster, and
    AMI BMCs have only a few session slots.
16. **Full Redfish error bodies.** gofish returns an empty body for some AMI errors
    (e.g. `PropertyNotWritable` for SNMP); surface the `@Message.ExtendedInfo` text.
17. **Firmware data source (2.1).** BMC and BIOS versions, so a `check` block can warn
    when the firmware differs from a tested version: after an update, behaviour of
    the private web API may change.

## For other users

18. **Import** for every resource. Today `Create` adopts the existing setting, which
    works but is unusual for Terraform users.
19. **Sensor data source.** List sensor names, readings and thresholds, so nobody has
    to copy names from the web UI.
20. **Recorded-BMC tests in CI.** Replay captured AMI responses, extending the fake BMC
    the unit tests already use, so changes are tested without hardware.
21. **Compatibility reports.** Collect which boards and firmware versions work, via
    the bug report template.
