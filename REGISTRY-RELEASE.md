# Plan: publish v2.0.0 to the Terraform Registry

Working note for the release session (written 2026-10-10). Delete this file in the
release commit once everything below is done.

**Goal:** `simonzrln/redfish` installs from registry.terraform.io like any other
provider, so `terraform init -upgrade` picks up new releases. Today the homelab builds
the binary into a local filesystem mirror (see "Homelab cleanup" at the end).

## Decisions already made

- **First registry version is `v2.0.0`.** The fork breaks upstream (Dell resources
  removed, new ones added), and a plain semver avoids the prerelease quirks of
  `v1.6.2-ami.1` (Terraform only selects prereleases with an exact pin).
- The repo inherited Dell's tags `v0.1.0` to `v1.6.2`. They have no GitHub releases in
  this fork, so the registry ignores them; `v2.0.0` sorts above all of them. Do not
  delete them.
- GPG key without expiry date; private key and passphrase go into Simon's password
  safe (MCP `secrets`, entry name `terraform-registry-gpg`), never into the repo.
- Compatibility is stated honestly: tested only on an ASRock Rack B650D4U, BMC
  firmware 6.09 (AST2600), BIOS 22.10.

## 1. Fix before the first release

- [ ] `terraform-registry-manifest.json`: `protocol_versions` must be `["6.0"]`.
      `main.go` uses `providerserver.Serve`, which speaks protocol 6; the file still
      says `5.0` from upstream.
- [ ] `main.go`: drop the commented-out old `main()` block and the `go:generate`
      lines that point at the deleted `examples/` tree, or replace them by the
      docs generation below.

## 2. Docs the registry renders

The registry shows one page per resource from `docs/`. Generate it instead of
writing it by hand:

- [ ] `examples/provider/provider.tf` plus one `examples/resources/<name>/resource.tf`
      per resource (reuse the README example and the homelab's
      `/opt/homelab/stacks/terraform/bmc/board.tf`, with `192.0.2.10` as endpoint).
- [ ] `templates/index.md.tmpl` with the AMI focus, the tested hardware and the
      conventions from the README (destroy only forgets, read-back check, no reboot
      without `reset_type`).
- [ ] `go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name redfish`
      (pin a current version), commit `docs/`. Re-run whenever a schema
      description changes; add a CI check that `docs/` is up to date.

## 3. Release pipeline

- [ ] `.goreleaser.yml` from HashiCorp's provider scaffolding
      (github.com/hashicorp/terraform-provider-scaffolding-framework): builds
      `CGO_ENABLED=0` for linux/darwin/windows/freebsd × amd64/arm64 (drop
      windows/arm as upstream did), zips named
      `terraform-provider-redfish_{{ .Version }}_{{ .Os }}_{{ .Arch }}.zip`,
      `terraform-provider-redfish_{{ .Version }}_SHA256SUMS` plus `.sig`, and
      `terraform-registry-manifest.json` renamed to
      `terraform-provider-redfish_{{ .Version }}_manifest.json` as extra file.
- [ ] `.github/workflows/release.yml`: on tag `v*`, checkout, setup-go from
      `go.mod`, import the key with `crazy-max/ghaction-import-gpg`, run
      `goreleaser/goreleaser-action` with `release --clean`, env `GPG_FINGERPRINT`.
      Pin action versions.
- [ ] GPG key: `gpg --quick-generate-key "Simon (terraform-provider-redfish) <106229814+simonzrln@users.noreply.github.com>" rsa4096 sign never`,
      export the private key armored, store it plus passphrase in the safe, then
      `gh secret set GPG_PRIVATE_KEY` and `gh secret set PASSPHRASE` on
      `simonzrln/terraform-provider-redfish`.
- [ ] Dry run: `goreleaser release --snapshot --clean` locally (docker
      `goreleaser/goreleaser`), check the archive names and the manifest file.

## 4. Repo polish for other users

- [ ] README: section "Compatibility" (tested hardware above, AMI firmware differs
      between boards; things that may differ: string-typed service fields, SNMP
      writability, settings object path).
- [ ] `.github/ISSUE_TEMPLATE/bug_report.md`: board, BMC firmware, BIOS version,
      provider version, failing resource, the request/response (redact passwords).
- [ ] CHANGELOG entry for 2.0.0.

## 5. Publish (Simon, in the browser, about 10 minutes)

1. registry.terraform.io → sign in with GitHub (simonzrln).
2. User settings → Signing keys → add the **public** key (ASCII armored).
3. Publish → Provider → select `simonzrln/terraform-provider-redfish`.
4. Push tag `v2.0.0`; the release workflow creates the GitHub release, the registry
   picks it up via webhook within minutes.
5. Check: `terraform init` in an empty dir with
   `redfish = { source = "simonzrln/redfish", version = "2.0.0" }`.

## 6. Homelab cleanup (in /opt/homelab, after the registry serves 2.0.0)

- [ ] `stacks/terraform/bmc/main.tf`: `version = "~> 2.0"`.
- [ ] Delete `stacks/terraform/build-redfish-provider.sh`, `stacks/terraform/bmc.tfrc`,
      the `providers/` mirror and its `.gitignore` line; in
      `stacks/terraform/docker-compose.yml` drop `TF_CLI_CONFIG_FILE` and the
      `bmc.tfrc` / `providers` volumes of `terraform-bmc`.
- [ ] `./tf.sh bmc init -upgrade`, then `./tf.sh bmc plan` must say "No changes".
      Commit the new `.terraform.lock.hcl` (now with registry hashes).
- [ ] Update `stacks/terraform/README.md` (section "BMC and BIOS"),
      `docs/runbooks/bmc-fan-control.md` and the Claude memory note
      `bmc-terraform-fork.md`.
