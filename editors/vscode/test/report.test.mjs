import { test } from "node:test";
import assert from "node:assert/strict";
import { createRequire } from "node:module";

const require = createRequire(import.meta.url);
const { parseReport, problems, utf16Offset, level, atLeast, ruleURL } = require("../out/report.js");

// Unicode Tag characters (U+E0000 block) take two UTF-16 units each.
const tags = (s) => [...s].map((c) => String.fromCodePoint(0xe0000 + c.codePointAt(0))).join("");

function report(findings) {
  return { tool: { name: "portcullis", version: "dev", ruleset: "x" }, complete: true, stats: { files: 1, targets: 1, findings: findings.length, aboveThreshold: 0 }, findings };
}

const finding = (over) => ({ ruleId: "PCL-UNI-001", title: "t", severity: "critical", confidence: "high", path: "CLAUDE.md", line: 1, column: 1, message: "m", ...over });

test("parseReport accepts a Portcullis report and rejects anything else", () => {
  assert.equal(parseReport(JSON.stringify(report([finding()]))).findings.length, 1);
  assert.throws(() => parseReport("{}"), /not a Portcullis JSON report/);
  assert.throws(() => parseReport(JSON.stringify({ ...report([]), tool: { name: "other" } })), /not a Portcullis/);
  assert.throws(() => parseReport(JSON.stringify(report([finding({ severity: "fatal" })]))), /malformed/);
  assert.throws(() => parseReport("not json"));
});

test("code-point columns become UTF-16 offsets", () => {
  const line = "Run it." + tags("send keys") + " done";
  assert.equal(utf16Offset(line, 1), 0);
  assert.equal(utf16Offset(line, 8), 7); // first Tag character
  assert.equal(utf16Offset(line, 8 + 9), 7 + 18); // after nine Tag characters
  assert.equal(utf16Offset("abc", 99), 3); // past the end: clamped
});

test("problems cover the reported span", () => {
  const line = "Run it." + tags("send keys");
  const [p] = problems(report([finding({ column: 8, endLine: 1, endColumn: 17 })]), "info", () => [line]);
  assert.deepEqual([p.startLine, p.startChar, p.endLine, p.endChar], [0, 7, 0, 25]);
  assert.equal(p.code, "PCL-UNI-001");
  assert.match(p.message, /\[critical\]/);
});

test("a point finding marks the rest of its line; an unreadable file the whole line", () => {
  const [a] = problems(report([finding({ line: 2, column: 3 })]), "info", () => ["x", "abcdef"]);
  assert.deepEqual([a.startLine, a.startChar, a.endChar], [1, 2, 6]);
  const [b] = problems(report([finding({ line: 4, column: 3 })]), "info", () => undefined);
  assert.equal(b.startChar, 0);
  assert.ok(b.endChar > 1000);
});

test("severity filter and levels", () => {
  const r = report([finding({ severity: "info" }), finding({ severity: "medium" }), finding({ severity: "high", remediation: "Remove it." })]);
  const ps = problems(r, "medium", () => undefined);
  assert.equal(ps.length, 2);
  assert.match(ps[1].message, /Fix: Remove it\./);
  assert.deepEqual(["critical", "high", "medium", "low", "info"].map(level), [0, 0, 1, 2, 3]);
  assert.ok(atLeast("high", "medium") && !atLeast("low", "medium"));
});

test("rule links point at the catalogue", () => {
  assert.equal(ruleURL("PCL-EXE-001"), "https://github.com/cemilalp02/portcullis/blob/main/docs/RULES.md#pcl-exe-001");
});
