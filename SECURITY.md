# Security Policy

Bekçi is a security tool; a flaw in it can give users false confidence. We take reports seriously.

## Supported versions

Until v1.0, only the **latest release** receives security fixes.

## Reporting a vulnerability

**Do not open a public issue.** Use GitHub's private vulnerability reporting:
[Report a vulnerability](https://github.com/cemilalp02/bekci/security/advisories/new).

Please include the Bekçi version (`bekci version`), your OS, a minimal **defanged**
reproduction and the impact you observed.

## What counts as a vulnerability

Report privately:

- Bekçi executes, evaluates or fetches anything based on scanned content.
- Scanned content changes Bekçi's behaviour in untrusted mode (config, suppression, rules).
- A crafted input crashes or hangs Bekçi, or makes it report **clean / exit 0** while
  analysis was incomplete (fail-open).
- Path traversal, symlink / junction escape, or terminal-escape injection through output.
- Problems in our release artefacts, signatures or GitHub Action.

Usually fine to report publicly with the *Missed detection* issue template:

- A new evasion technique or an attack pattern no rule covers yet (see the known limitations
  in [`docs/THREAT_MODEL.md`](docs/THREAT_MODEL.md) §6).
  If the technique is being **actively exploited in the wild**, report it privately instead.

## Response targets

| Step | Target |
|---|---|
| Acknowledge report | 3 business days |
| Initial assessment | 7 days |
| Fix or mitigation for high/critical issues | 30 days |

We credit reporters in the advisory unless they prefer otherwise.

## Safe harbour

Good-faith research on your own systems, consistent with this policy, is welcome. Do not test
against systems you do not own, and never include working malicious payloads in reports.
