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

- [x] `terraform-registry-manifest.json`: `protocol_versions` must be `["6.0"]`.
      `main.go` uses `providerserver.Serve`, which speaks protocol 6; the file still
      says `5.0` from upstream.
- [x] `main.go`: drop the commented-out old `main()` block and the `go:generate`
      lines that point at the deleted `examples/` tree, or replace them by the
      docs generation below.

## 2. Docs the registry renders

The registry shows one page per resource from `docs/`. Generate it instead of
writing it by hand:

- [x] `examples/provider/provider.tf` plus one `examples/resources/<name>/resource.tf`
      per resource (reuse the README example and the homelab's
      `/opt/homelab/stacks/terraform/bmc/board.tf`, with `192.0.2.10` as endpoint).
- [x] `templates/index.md.tmpl` with the AMI focus, the tested hardware and the
      conventions from the README (destroy only forgets, read-back check, no reboot
      without `reset_type`).
- [x] `go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name redfish`
      (pin a current version), commit `docs/`. Re-run whenever a schema
      description changes; add a CI check that `docs/` is up to date.

## 3. Release pipeline

- [x] `.goreleaser.yml` from HashiCorp's provider scaffolding
      (github.com/hashicorp/terraform-provider-scaffolding-framework): builds
      `CGO_ENABLED=0` for linux/darwin/windows/freebsd × amd64/arm64 (drop
      windows/arm as upstream did), zips named
      `terraform-provider-redfish_{{ .Version }}_{{ .Os }}_{{ .Arch }}.zip`,
      `terraform-provider-redfish_{{ .Version }}_SHA256SUMS` plus `.sig`, and
      `terraform-registry-manifest.json` renamed to
      `terraform-provider-redfish_{{ .Version }}_manifest.json` as extra file.
- [x] `.github/workflows/release.yml`: on tag `v*`, checkout, setup-go from
      `go.mod`, import the key with `crazy-max/ghaction-import-gpg`, run
      `goreleaser/goreleaser-action` with `release --clean`, env `GPG_FINGERPRINT`.
      Pin action versions.
- [x] GPG key: generated 2026-10-10, fingerprint
      `5255755CC67B5804B87FD5DA2FF6ACBC957777E5`, rsa4096, no expiry. Generated in
      the session scratchpad
      `/tmp/claude-1000/-home-simon-terraform-provider-redfish/700259c5-4cb5-4f6e-840f-d0c64866c77a/scratchpad/`
      (`gpg-private.asc`, `gpg-pass`, `gpg-public.asc`, keyring `gnupg/`).
      GitHub secrets `GPG_PRIVATE_KEY` and `PASSPHRASE` are set, private key +
      passphrase are in the safe (`terraform-registry-gpg`, line 1 = passphrase).
      Delete the scratchpad copies once the public key is on the registry.
- [x] Dry run: `goreleaser release --snapshot --clean` locally (docker
      `goreleaser/goreleaser`), check the archive names and the manifest file.

## 4. Repo polish for other users

- [x] README: section "Compatibility" (tested hardware above, AMI firmware differs
      between boards; things that may differ: string-typed service fields, SNMP
      writability, settings object path).
- [x] `.github/ISSUE_TEMPLATE/bug_report.md`: board, BMC firmware, BIOS version,
      provider version, failing resource, the request/response (redact passwords).
- [x] CHANGELOG entry for 2.0.0.

## 5. Publish (Simon, about 15 minutes)

The registry only accepts a repo that already has a GitHub release, so the release
comes first and "Publish" second.

1. Secrets set (section 3, `gh secret list` shows `GPG_PRIVATE_KEY` and `PASSPHRASE`).
2. `git push origin ami`, wait for the `test` workflow (incl. the docs check) to pass.
3. `git tag -a v2.0.0 -m "v2.0.0" && git push origin v2.0.0` (only this tag, never
   `--tags`). The `Release` workflow builds, signs and creates the GitHub release:
   8 zips, `_SHA256SUMS`, `_SHA256SUMS.sig`, `_manifest.json`, not a draft.
4. registry.terraform.io → sign in with GitHub (simonzrln) → User settings → Signing
   keys → add the **public** key (`gpg-public.asc`, ASCII armored).
5. Publish → Provider → select `simonzrln/terraform-provider-redfish`. The registry
   ingests v2.0.0 then; later releases arrive via webhook.
6. Check: `terraform init` in an empty dir with
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
