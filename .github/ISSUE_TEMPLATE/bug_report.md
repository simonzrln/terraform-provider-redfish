---
name: Bug report
about: A resource fails or does not do what it should on your BMC
labels: bug
---

<!--
Redact before posting: passwords, session tokens (X-Auth-Token, CSRF tokens,
QSESSIONID cookies) and serial numbers.
-->

**Hardware**

- Board and vendor:
- BMC firmware version:
- BIOS version:

**Versions**

- Provider version:
- Terraform version:

**Failing resource and config**

```hcl

```

**Error output**

```

```

**What the BMC returns** (optional, redacted): the raw JSON of the failing resource, e.g.
`curl -k -u admin https://<bmc>/redfish/v1/...`

```

```
