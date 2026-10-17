# Using Portcullis in CI

## GitHub Actions

Add `.github/workflows/portcullis.yml`:

```yaml
name: Portcullis
on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  scan:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      security-events: write   # upload results to code scanning
    steps:
      - uses: actions/checkout@v7
      - uses: cemilalp02/portcullis@v0.2.0
```

What it does on every run:

1. Installs Portcullis. At a release tag it downloads the release archive for the runner and runs
   it only after `gh attestation verify` confirms it was built by this repository's release
   workflow for that tag and its SHA-256 matches the signed `checksums.txt`; a failed check stops
   the job. At a branch or commit SHA, or with `from-source: true`, it builds from the action's
   own source.
2. Scans the repository for malicious or risky agent context and **annotates findings on the
   pull request diff** (works for private repositories and forks; no GitHub Advanced Security needed).
3. Uploads SARIF to **code scanning** (Security tab) when the token may do so.
4. Writes a Markdown summary to the job summary, including the auto-execution inventory.
5. Uploads a self-contained **HTML report** as a workflow artifact (`portcullis-report.html`):
   one file, opens offline in any browser, with severity filters and search.
6. Runs `portcullis verify` when the repository has a `context.lock`.
7. Fails the job when a finding is at or above the threshold (default `high`) or when an approved
   file changed without updating `context.lock`.

### Inputs

| Input | Default | Description |
|---|---|---|
| `path` | `.` | Directory to scan, relative to the repository root |
| `fail-on` | *(config or `high`)* | `info`, `low`, `medium`, `high`, `critical` or `none` |
| `exclude` | | Directory globs to skip, one per line |
| `verify-lock` | `auto` | `auto` (if `context.lock` exists), `true` or `false` |
| `upload-sarif` | `true` | Upload to code scanning (skipped automatically for pull requests from forks) |
| `sarif-category` | `portcullis` | Code scanning category, to tell several runs apart; also names the HTML report (`<category>-report.html`) |
| `upload-report` | `true` | Upload the HTML report as a workflow artifact |
| `from-source` | `false` | Build from the action's source even at a release tag |
| `go-version` | `stable` | Go version used when Portcullis is built from source |

Outputs: `exit-code` (0, 1 or 2), `sarif-file`, `html-file` and `report-url` (download link of the
uploaded report).

When you run the action more than once in a workflow, give each run its own `sarif-category`;
artifact names must be unique within a workflow run.

### Pull requests cannot weaken their own review

In pull requests the action reads `.portcullis.yaml`, `.portcullis-baseline.yaml`,
`context.lock` and the `.portcullis/` directory **from the base commit**. A pull request that
relaxes the threshold, disables a rule, adds a baseline entry or adds a custom rule does not
change how that same pull request is scanned; the change itself is reported as `PCL-ENG-009` so
reviewers see it. The checked-out tree is always scanned as data: Portcullis never executes
repository content.

Use the `pull_request` event. With the default `actions/checkout` settings the base commit is
fetched on demand; if you set `persist-credentials: false` on a private repository, also set
`fetch-depth: 0`.

### Private repositories without GitHub Advanced Security

Code scanning uploads need GitHub Advanced Security on private repositories. Set
`upload-sarif: false`; annotations, the job summary and the pass/fail result still work.

## Approving agent context with `context.lock`

```bash
portcullis lock            # scan, then record the hashes of every agent context file
git add context.lock
```

From then on `portcullis verify` (run automatically by the action) fails if any agent context
file is added, modified or removed without a new `portcullis lock`. In a pull request the
diff of `context.lock` shows reviewers exactly which files are being re-approved.
Protect the file with `CODEOWNERS` so changes need a security reviewer.

## Adopting Portcullis in an existing repository

```bash
portcullis scan --trusted                      # see what is reported
portcullis baseline --reason "reviewed on 2026-10-15 by @you"
git add .portcullis-baseline.yaml
```

The baseline accepts the current findings by fingerprint; new findings are still reported, and
entries that no longer match anything are reported as stale (`PCL-ENG-013`). Configuration
lives in `.portcullis.yaml` (see [FILE_FORMATS.md](FILE_FORMATS.md)).

## pre-commit

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/cemilalp02/portcullis
    rev: v0.2.0
    hooks:
      - id: portcullis          # scan agent context files before each commit
      - id: portcullis-verify   # optional: require context.lock to be up to date
```

## Other CI systems

Run the CLI directly; it needs no network access and no configuration:

```bash
portcullis scan --trusted --format sarif -o portcullis.sarif .
```

Exit codes: `0` clean, `1` findings at or above the threshold, `2` usage error or incomplete
analysis. `--format github` prints GitHub workflow annotations, `--summary FILE` appends a
Markdown summary, `--path-prefix DIR` fixes paths when you scan a subdirectory, and `--html FILE` writes the
offline HTML report for your CI system to archive.
