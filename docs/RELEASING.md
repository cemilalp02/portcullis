# Releasing Portcullis

A release is cut by pushing a version tag. Everything after that is automated, signed and
verifiable; nothing is uploaded by hand.

## What the release workflow does

[`.github/workflows/release.yml`](../.github/workflows/release.yml) runs on a `vX.Y.Z` tag:

1. Runs the tests with the pinned Go toolchain (1.27.2).
2. Builds with [GoReleaser](https://goreleaser.com) ([`.goreleaser.yaml`](../.goreleaser.yaml)):
   static binaries for Linux, macOS and Windows on amd64 and arm64, one archive each, an SPDX
   SBOM per archive (syft) and `checksums.txt` (SHA-256).
3. Signs `checksums.txt` keylessly with [Sigstore](https://www.sigstore.dev) (cosign); the
   bundle `checksums.txt.sigstore.json` holds the signature, the short-lived certificate that
   names the workflow and tag, and the transparency-log entry.
4. Creates the GitHub release as a **draft**, with the notes taken from this version's section
   of [`CHANGELOG.md`](../CHANGELOG.md) (`scripts/release/notes.sh`; the release fails if the
   section is missing).
5. Attests **SLSA build provenance** for every archive and SBOM
   (`actions/attest-build-provenance`), then publishes the release. Nothing is downloadable
   before its provenance exists.

The GitHub Action uses these files: at a release tag (`cemilalp02/portcullis@v0.1.0`) it
downloads the archive for the runner and runs it only if `gh attestation verify` confirms it
was built by this workflow for that tag and its SHA-256 matches `checksums.txt`
([`scripts/action/install.sh`](../scripts/action/install.sh), tested offline by
`install_test.sh`). At any other ref it builds from its own source.

## Builds are reproducible

Binaries carry no build-machine data: `-trimpath`, version, commit and date injected from the
tag, file times from the commit (`mod_timestamp`, archive `mtime`). Building the same tag with the
same GoReleaser and Go versions gives byte-identical archives, which is checked before each
release by running the snapshot twice. The SBOM documents are not byte-identical: syft writes
its own creation time and document ID into each one.

## Before tagging

1. Every check passes locally: `make check` (or the commands it lists on Windows), plus
   `make release-snapshot` twice with identical `dist/checksums.txt` for the archives.
2. `CHANGELOG.md` has a `## [X.Y.Z] - YYYY-MM-DD` section and the comparison links at the end.
3. Documentation that names a version (the README, `docs/GITHUB_ACTION.md`) uses the new tag.
4. `docs/ROADMAP.md` and `docs/THREAT_MODEL.md` were reviewed for the release.

## Tagging

```bash
git switch main && git pull --ff-only
git tag -a vX.Y.Z -m "Portcullis vX.Y.Z"
git push origin vX.Y.Z
```

A published tag is permanent: the Go checksum database records it, so `go install …@vX.Y.Z`
always resolves to the same code. Never move or delete a released tag; fix forward with the
next patch version.

If GitHub Actions cannot run, the tag still gives `go install …@vX.Y.Z` and makes the Action
build from source at that tag; signed binaries follow when the workflow can run (re-run it for
the tag). Unsigned binaries are never attached by hand.

## Verifying a release

```bash
# Provenance of one archive (GitHub CLI)
gh attestation verify portcullis_X.Y.Z_linux_amd64.tar.gz --repo cemilalp02/portcullis \
  --signer-workflow cemilalp02/portcullis/.github/workflows/release.yml

# Signature of the checksums, then the archives against them (cosign)
cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/cemilalp02/portcullis/.github/workflows/release.yml@refs/tags/vX.Y.Z" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --ignore-missing -c checksums.txt
```

The release also carries `portcullis-vscode_X.Y.Z.vsix`, the VS Code extension built by the
release workflow at that tag (`npm version` sets its version from the tag). It is listed in
`checksums.txt`, so the signature and the provenance cover it. The archive is not byte-for-byte
reproducible (a ZIP with build times). Publishing to the Visual Studio Marketplace needs a
publisher account and token of the project owner (`npx @vscode/vsce publish`); it is not part
of the workflow.

## Website

The project website is the static page in `site/`. `scripts/site/build.sh` assembles it with
the fonts of the HTML report and the images in `docs/assets/` into `_site/`; serve that
directory with any static file server to preview it. `.github/workflows/pages.yml` publishes it
on every change to `main`. It needs, once, **Settings → Pages → Source: GitHub Actions**, and
GitHub Actions able to start jobs. The page loads nothing from other hosts;
`internal/archtest` checks that every file it refers to is published.
