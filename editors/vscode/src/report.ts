// Turns a Portcullis JSON report (schemas/report-v1.json) into editor
// problems. Nothing here depends on the VS Code API, so it is tested with
// plain Node (test/report.test.mjs).

export type Severity = "critical" | "high" | "medium" | "low" | "info";

export interface Finding {
  ruleId: string;
  title: string;
  severity: Severity;
  confidence: string;
  path: string;
  line: number; // 1-based
  column: number; // 1-based, in Unicode code points
  endLine?: number;
  endColumn?: number;
  message: string;
  remediation?: string;
}

export interface Report {
  tool: { name: string; version: string; ruleset: string };
  complete: boolean;
  findings: Finding[];
  stats: { files: number; targets: number; findings: number; aboveThreshold: number };
}

const order: Severity[] = ["info", "low", "medium", "high", "critical"];

/** Problem severities, as VS Code's DiagnosticSeverity numbers. */
export const enum Level {
  Error = 0,
  Warning = 1,
  Information = 2,
  Hint = 3,
}

export function level(s: Severity): Level {
  switch (s) {
    case "critical":
    case "high":
      return Level.Error;
    case "medium":
      return Level.Warning;
    case "low":
      return Level.Information;
    default:
      return Level.Hint;
  }
}

export function atLeast(s: Severity, min: Severity): boolean {
  return order.indexOf(s) >= order.indexOf(min);
}

/** Parses the output of `portcullis scan --format json`, rejecting anything else. */
export function parseReport(text: string): Report {
  const r = JSON.parse(text) as Report;
  if (!r || typeof r !== "object" || !Array.isArray(r.findings) || !r.tool || r.tool.name !== "portcullis") {
    throw new Error("not a Portcullis JSON report");
  }
  for (const f of r.findings) {
    if (typeof f.path !== "string" || !Number.isInteger(f.line) || !order.includes(f.severity)) {
      throw new Error("malformed finding in the report");
    }
  }
  return r;
}

/**
 * Converts a 1-based column in Unicode code points (what Portcullis reports,
 * like SARIF's "unicodeCodePoints") to a 0-based offset in UTF-16 code units
 * (what the editor uses). Characters outside the BMP, such as the Unicode Tag
 * characters of a smuggled instruction, take two units each.
 */
export function utf16Offset(lineText: string, column: number): number {
  let units = 0;
  let points = 1;
  for (const ch of lineText) {
    if (points >= column) {
      break;
    }
    units += ch.length;
    points++;
  }
  return units;
}

export interface Problem {
  path: string;
  startLine: number; // 0-based
  startChar: number; // UTF-16
  endLine: number;
  endChar: number;
  level: Level;
  code: string;
  message: string;
}

/**
 * Builds the problems of a report. lines(path) returns the lines of a file
 * (or undefined when it cannot be read), to place columns exactly; without
 * them a finding covers its whole line.
 */
export function problems(r: Report, min: Severity, lines: (path: string) => string[] | undefined): Problem[] {
  const out: Problem[] = [];
  for (const f of r.findings) {
    if (!atLeast(f.severity, min)) {
      continue;
    }
    const text = lines(f.path);
    const startLine = Math.max(f.line, 1) - 1;
    const endLine = Math.max(f.endLine ?? f.line, f.line) - 1;
    const startText = text?.[startLine];
    const endText = text?.[endLine];
    const startChar = startText === undefined ? 0 : utf16Offset(startText, f.column || 1);
    let endChar = endText === undefined ? Number.MAX_SAFE_INTEGER : utf16Offset(endText, f.endColumn || f.column || 1);
    if (endLine === startLine && endChar <= startChar) {
      // A point (a file-level finding, or an empty match): mark the rest of the line.
      endChar = startText === undefined ? Number.MAX_SAFE_INTEGER : startText.length;
    }
    let message = `${f.message} [${f.severity}]`;
    if (f.remediation) {
      message += `\nFix: ${f.remediation}`;
    }
    out.push({ path: f.path, startLine, startChar, endLine, endChar, level: level(f.severity), code: f.ruleId, message });
  }
  return out;
}

/** The rule's entry in the published catalogue. */
export function ruleURL(ruleId: string): string {
  return `https://github.com/cemilalp02/portcullis/blob/main/docs/RULES.md#${ruleId.toLowerCase()}`;
}
