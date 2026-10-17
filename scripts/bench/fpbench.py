#!/usr/bin/env python3
"""False-positive benchmark (REQUIREMENTS §8, Phase 6).

Selects popular public repositories that contain agent context files, pins
each to a commit, fetches only the files Portcullis looks at, scans them and
reports the triaged false-positive rate and per-rule precision.

    fpbench.py select --out testdata/benchmark/repos.tsv [--count 1000] [--exclude REPOS_TSV]
    fpbench.py fetch  --repos testdata/benchmark/repos.tsv --cache DIR [--jobs 8]
    fpbench.py scan   --repos testdata/benchmark/repos.tsv --cache DIR --bin portcullis --out results.jsonl
    fpbench.py report --results results.jsonl --triage testdata/benchmark/triage.tsv

Selection: repositories found by GitHub search, sorted by stars, not forks or
archived, with at least one agent context file at the root (see AGENT_PATHS).
Only standard library; `gh` (authenticated) and `git` must be on PATH.
Scans run in the default untrusted mode, as for a freshly cloned repository.
"""
import argparse
import concurrent.futures
import json
import os
import subprocess
import sys
import time

# Root-level paths that mark a repository as using AI agent context.
AGENT_PATHS = {
    "claude_md": "CLAUDE.md", "agents_md": "AGENTS.md", "gemini_md": "GEMINI.md",
    "claude_dir": ".claude", "cursorrules": ".cursorrules", "cursor_dir": ".cursor",
    "copilot": ".github/copilot-instructions.md", "gh_instructions": ".github/instructions",
    "mcp": ".mcp.json", "windsurf": ".windsurfrules", "cline": ".clinerules",
}

# Sparse-checkout patterns: every file a Portcullis target can match
# (rules/targets.yaml). Skill companions are added in a second pass.
SPARSE = [
    "**/CLAUDE.md", "**/CLAUDE.local.md", "**/AGENTS.md", "**/GEMINI.md", "**/SKILL.md",
    "**/.claude/", "**/.cursorrules", "**/.cursor/rules/", "**/.github/copilot-instructions.md",
    "**/.github/instructions/", "**/.github/prompts/", "**/.windsurfrules", "**/.clinerules",
    "**/.vscode/tasks.json", "**/.devcontainer/", "**/.devcontainer.json", "**/.mcp.json",
    "**/mcp.json", "**/package.json", "**/.portcullis.yaml",
    "**/.vscode/settings.json", "**/.gemini/", "**/.github/agents/", "**/.github/chatmodes/",
]

STAR_BANDS = ["stars:>=100000", "stars:50000..99999", "stars:30000..49999", "stars:20000..29999",
              "stars:15000..19999", "stars:10000..14999", "stars:8000..9999", "stars:6000..7999",
              "stars:5000..5999", "stars:4000..4999", "stars:3000..3999", "stars:2500..2999",
              "stars:2000..2499", "stars:1700..1999", "stars:1500..1699", "stars:1300..1499",
              "stars:1150..1299", "stars:1000..1149"]


def gh(*args, stdin=None):
    """Run gh and decode its JSON output, retrying rate limits and network
    errors with back-off; other API errors are raised at once."""
    err = ""
    for attempt in range(8):
        p = subprocess.run(["gh", *args], input=stdin, capture_output=True, text=True, encoding="utf-8", errors="replace")
        if p.returncode == 0:
            return json.loads(p.stdout)
        # GraphQL answers with partial data when some repositories cannot be
        # resolved (renamed, deleted); keep what came back.
        try:
            partial = json.loads(p.stdout)
            if isinstance(partial, dict) and partial.get("data"):
                return partial
        except json.JSONDecodeError:
            pass
        err = p.stderr.strip()
        low = err.lower()
        if "http 4" in low and "rate limit" not in low and "secondary" not in low:
            raise RuntimeError(err)
        time.sleep(min(15 * (attempt + 1), 90))
    raise RuntimeError(err)


def candidates(per_band, cache):
    """Search results per star band; finished bands are kept in `cache` so an
    interrupted run resumes where it stopped."""
    done = {}
    if os.path.exists(cache):
        with open(cache, encoding="utf-8") as f:
            done = json.load(f)
    seen = []
    for band in STAR_BANDS:
        if band in done:
            seen += done[band]
            continue
        names = []
        for page in range(1, per_band // 100 + 1):
            q = f"{band} fork:false archived:false pushed:>2025-10-01"
            r = gh("api", "-X", "GET", "search/repositories", "-f", f"q={q}", "-f", "sort=stars",
                   "-f", "order=desc", "-f", "per_page=100", "-f", f"page={page}")
            items = r.get("items", [])
            names += [i["full_name"] for i in items]
            print(f"  {band} page {page}: {len(items)}", file=sys.stderr)
            if len(items) < 100:
                break
            time.sleep(2.2)  # search API: 30 requests per minute
        done[band] = names
        seen += names
        with open(cache, "w", encoding="utf-8") as f:
            json.dump(done, f)
    return list(dict.fromkeys(seen))


def probe(names):
    """Stars, pinned commit and root agent files for a batch of repositories."""
    parts = []
    for i, full in enumerate(names):
        owner, name = full.split("/", 1)
        objs = " ".join(f'{k}: object(expression: "HEAD:{p}") {{ id }}' for k, p in AGENT_PATHS.items())
        parts.append(f'r{i}: repository(owner: {json.dumps(owner)}, name: {json.dumps(name)}) '
                     f'{{ nameWithOwner stargazerCount isFork isArchived defaultBranchRef {{ target {{ oid }} }} {objs} }}')
    query = "query { " + " ".join(parts) + " }"
    r = gh("api", "graphql", "-f", f"query={query}")
    out = []
    for v in (r.get("data") or {}).values():
        if not v or v["isFork"] or v["isArchived"] or not v["defaultBranchRef"]:
            continue
        found = [k for k in AGENT_PATHS if v.get(k)]
        out.append((v["nameWithOwner"], v["defaultBranchRef"]["target"]["oid"], v["stargazerCount"], found))
    return out


def cmd_select(a):
    names = candidates(a.per_band, a.out + ".candidates.json")
    # A held-out set excludes the repositories the rules were tuned on.
    excluded = {repo.lower() for path in a.exclude for repo, _ in read_repos(path)}
    names = [n for n in names if n.lower() not in excluded]
    print(f"{len(names)} candidate repositories ({len(excluded)} excluded)", file=sys.stderr)
    # Candidates come in descending star order, so probing stops once enough
    # repositories with agent files are found.
    picked = []
    batches = [names[i:i + 20] for i in range(0, len(names), 20)]
    with concurrent.futures.ThreadPoolExecutor(4) as ex:
        for n, result in enumerate(ex.map(probe, batches), 1):
            picked += [(repo, sha, stars, found) for repo, sha, stars, found in result if found]
            if n % 10 == 0:
                print(f"  probed {min(n * 20, len(names))}/{len(names)}, {len(picked)} with agent files", file=sys.stderr)
            if len(picked) >= a.count * 1.1:
                break
    picked.sort(key=lambda r: (-r[2], r[0].lower()))
    picked = picked[:a.count]
    with open(a.out, "w", encoding="utf-8", newline="\n") as f:
        f.write("# Portcullis false-positive benchmark: popular repositories with agent context files.\n")
        f.write(f"# Selected {time.strftime('%Y-%m-%d')} by scripts/bench/fpbench.py select; pinned commits.\n")
        for path in a.exclude:
            f.write(f"# Excludes the repositories in {os.path.basename(path)}.\n")
        f.write("# repo\tcommit\tstars\troot agent files\n")
        for repo, sha, stars, found in picked:
            f.write(f"{repo}\t{sha}\t{stars}\t{','.join(found)}\n")
    print(f"wrote {len(picked)} repositories to {a.out}", file=sys.stderr)


def read_repos(path):
    with open(path, encoding="utf-8") as f:
        return [line.rstrip("\n").split("\t")[:2] for line in f if line.strip() and not line.startswith("#")]


def git(cwd, *args):
    return subprocess.run(["git", "-c", "core.longpaths=true", "-c", "advice.detachedHead=false", *args],
                          cwd=cwd, capture_output=True, text=True, encoding="utf-8", errors="replace")


def fetch_one(cache, repo, sha):
    d = os.path.join(cache, repo.replace("/", "__"))
    done = os.path.join(d, ".fpbench-done")
    if os.path.exists(done):
        return repo, "cached"
    os.makedirs(d, exist_ok=True)
    git(d, "init", "-q")
    git(d, "remote", "add", "origin", f"https://github.com/{repo}.git")
    git(d, "config", "core.sparseCheckout", "true")
    git(d, "config", "core.protectNTFS", "true")
    git(d, "config", "core.symlinks", "false")
    with open(os.path.join(d, ".git", "info", "sparse-checkout"), "w", encoding="utf-8") as f:
        f.write("\n".join(SPARSE) + "\n")
    r = git(d, "fetch", "-q", "--depth", "1", "--filter=blob:none", "origin", sha)
    if r.returncode != 0:
        return repo, "fetch failed: " + r.stderr.strip()[:200]
    git(d, "checkout", "-q", "FETCH_HEAD")
    # Second pass: everything next to a SKILL.md is part of the skill.
    skills = [os.path.relpath(root, d).replace(os.sep, "/") for root, _, files in os.walk(d)
              if "SKILL.md" in files and ".git" not in root.split(os.sep)]
    if skills:
        with open(os.path.join(d, ".git", "info", "sparse-checkout"), "a", encoding="utf-8") as f:
            f.write("\n".join(f"/{s}/" for s in skills) + "\n")
        git(d, "checkout", "-q", "FETCH_HEAD")
    open(done, "w").close()
    return repo, "ok"


def cmd_fetch(a):
    repos = read_repos(a.repos)
    os.makedirs(a.cache, exist_ok=True)
    with concurrent.futures.ThreadPoolExecutor(a.jobs) as ex:
        for i, (repo, status) in enumerate(ex.map(lambda r: fetch_one(a.cache, *r), repos), 1):
            if status not in ("ok", "cached"):
                print(f"{repo}: {status}", file=sys.stderr)
            if i % 50 == 0:
                print(f"  fetched {i}/{len(repos)}", file=sys.stderr)


def cmd_scan(a):
    repos = read_repos(a.repos)
    with open(a.out, "w", encoding="utf-8", newline="\n") as out:
        for i, (repo, sha) in enumerate(repos, 1):
            d = os.path.join(a.cache, repo.replace("/", "__"))
            if not os.path.exists(os.path.join(d, ".fpbench-done")):
                out.write(json.dumps({"repo": repo, "commit": sha, "error": "not fetched"}) + "\n")
                continue
            t0 = time.perf_counter()
            p = subprocess.run([a.bin, "scan", "--format", "json", "--fail-on", "none", "--exclude", ".git", d],
                               capture_output=True, text=True, encoding="utf-8")
            secs = time.perf_counter() - t0
            try:
                res = json.loads(p.stdout)
            except json.JSONDecodeError:
                out.write(json.dumps({"repo": repo, "commit": sha, "error": p.stderr.strip()[:300]}) + "\n")
                continue
            res.update(repo=repo, commit=sha, seconds=round(secs, 3), exit=p.returncode)
            out.write(json.dumps(res) + "\n")
            if i % 100 == 0:
                print(f"  scanned {i}/{len(repos)}", file=sys.stderr)


SEV = {"info": 0, "low": 1, "medium": 2, "high": 3, "critical": 4}


def read_triage(path):
    t = {}
    if path and os.path.exists(path):
        with open(path, encoding="utf-8") as f:
            for line in f:
                if line.strip() and not line.startswith("#"):
                    repo, fp, rule, verdict, *note = line.rstrip("\n").split("\t")
                    t[(repo, fp)] = (rule, verdict, note[0] if note else "")
    return t


def cmd_report(a):
    triage = read_triage(a.triage)
    rows = [json.loads(l) for l in open(a.results, encoding="utf-8")]
    scanned = [r for r in rows if "error" not in r]
    untriaged, fp_repos, per_rule, by_rule_all = [], set(), {}, {}
    for r in scanned:
        for f in r.get("findings", []):
            by_rule_all[f["ruleId"]] = by_rule_all.get(f["ruleId"], 0) + 1
            if SEV[f["severity"]] < SEV["high"]:
                continue
            key = (r["repo"], f["fingerprint"])
            if key not in triage:
                untriaged.append((r["repo"], f))
                continue
            verdict = triage[key][1]
            tp, fp = per_rule.get(f["ruleId"], (0, 0))
            per_rule[f["ruleId"]] = (tp + (verdict == "tp"), fp + (verdict == "fp"))
            if verdict == "fp":
                fp_repos.add(r["repo"])
    if a.dump_untriaged:
        with open(a.dump_untriaged, "w", encoding="utf-8", newline="\n") as f:
            for repo, x in untriaged:
                f.write("\t".join([repo, x["fingerprint"], x["ruleId"], x["severity"], f'{x["path"]}:{x["line"]}',
                                   x["message"].replace("\t", " "), x.get("excerpt", "").replace("\t", " ")]) + "\n")
    secs = sorted(r.get("seconds", 0) for r in scanned)
    print(f"repositories scanned: {len(scanned)} (errors: {len(rows) - len(scanned)})")
    print(f"high+ findings: {sum(tp + fp for tp, fp in per_rule.values()) + len(untriaged)}, untriaged: {len(untriaged)}")
    print(f"repositories with a false high+ finding: {len(fp_repos)} -> FP rate {100 * len(fp_repos) / max(1, len(scanned)):.2f}%")
    if secs:
        print(f"scan time per repository: median {secs[len(secs) // 2]:.3f}s, max {secs[-1]:.3f}s")
    print("per-rule precision (high+):")
    for rule, (tp, fp) in sorted(per_rule.items()):
        print(f"  {rule}\ttp {tp}\tfp {fp}\tprecision {100 * tp / max(1, tp + fp):.0f}%")
    print("findings at all severities by rule:")
    for rule, n in sorted(by_rule_all.items(), key=lambda kv: -kv[1]):
        print(f"  {rule}\t{n}")
    if untriaged:
        sys.exit(1)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)
    s = sub.add_parser("select")
    s.add_argument("--out", required=True)
    s.add_argument("--count", type=int, default=1000)
    s.add_argument("--per-band", type=int, default=1000)
    s.add_argument("--exclude", action="append", default=[], metavar="REPOS_TSV")
    s.set_defaults(fn=cmd_select)
    s = sub.add_parser("fetch")
    s.add_argument("--repos", required=True)
    s.add_argument("--cache", required=True)
    s.add_argument("--jobs", type=int, default=8)
    s.set_defaults(fn=cmd_fetch)
    s = sub.add_parser("scan")
    s.add_argument("--repos", required=True)
    s.add_argument("--cache", required=True)
    s.add_argument("--bin", required=True)
    s.add_argument("--out", required=True)
    s.set_defaults(fn=cmd_scan)
    s = sub.add_parser("report")
    s.add_argument("--results", required=True)
    s.add_argument("--triage")
    s.add_argument("--dump-untriaged")
    s.set_defaults(fn=cmd_report)
    a = ap.parse_args()
    a.fn(a)


if __name__ == "__main__":
    main()
