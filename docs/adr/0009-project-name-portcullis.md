# ADR-0009: Project name "Portcullis"

- Status: Accepted
- Date: 2026-10-14

## Context
The project started under the working name *Bekçi* (Turkish: "night watchman"). For an
English-speaking open-source audience the name is hard to pronounce, type and search for
(the `ç` is usually dropped, giving the unrelated word `bekci`). Before v0.1 is the cheapest
moment to rename: no releases, no users, no external links.

A good name must (1) convey "inspect before it gets in", (2) not collide with an existing
security tool, and (3) be usable as a CLI command and Go module path.

## Decision
The project is named **Portcullis** — the iron grille dropped in front of a castle gate so
nothing passes until it has been inspected.

| Item | Value |
|---|---|
| Repository / module | `github.com/cemilalp02/portcullis` |
| Binary | `portcullis` |
| Config / baseline | `.portcullis.yaml`, `.portcullis-baseline.yaml` |
| Lock header | `portcullis-lock 1` (file name stays `context.lock`) |
| Rule ID prefix | `PCL-` (e.g. `PCL-UNI-001`) |
| SARIF fingerprint key | `portcullis/v1` |

## Consequences
- All docs, code and specs are updated in one change; git history keeps the old name in
  earlier commit messages, which is expected and not rewritten.
- GitHub redirects the old repository URL after the rename.
- The command is longer to type than the old name; shell completion (v0.2) mitigates this.

## Alternatives considered
Checked on GitHub on 2026-10-14 for collisions, especially in the security space:

| Name | Reason rejected |
|---|---|
| Warden, Bastion, Tripwire | Widely used, including established security products |
| Rampart | Microsoft security project |
| Vigil | Existing LLM prompt-injection scanner — same niche, confusing |
| Palisade | GCHQ project |
| Airlock | Name of a security company / product |
| Barbican | OpenStack key-management service |
| Redoubt, Parapet | Free, but meaning less obvious to most readers |
| ctxscan | Free and descriptive, but generic as a brand |
