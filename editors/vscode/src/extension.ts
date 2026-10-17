import { execFile } from "node:child_process";
import { readFileSync } from "node:fs";
import * as path from "node:path";
import * as vscode from "vscode";
import { parseReport, problems, ruleURL, type Report, type Severity } from "./report";

// Scans each workspace folder with the portcullis binary and shows the
// findings as problems. The binary is run directly (no shell) in untrusted
// mode, so the repository's own .portcullis.yaml cannot weaken the scan, and
// its path can only come from user or machine settings, never from the
// workspace that is being scanned.

let collection: vscode.DiagnosticCollection;
let status: vscode.StatusBarItem;
let output: vscode.OutputChannel;
const timers = new Map<string, NodeJS.Timeout>();
const running = new Map<string, Promise<void>>();

export function activate(context: vscode.ExtensionContext): void {
  collection = vscode.languages.createDiagnosticCollection("portcullis");
  output = vscode.window.createOutputChannel("Portcullis");
  status = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 50);
  status.command = "workbench.actions.view.problems";
  context.subscriptions.push(
    collection,
    output,
    status,
    vscode.commands.registerCommand("portcullis.scan", () => scanAll(true)),
    vscode.commands.registerCommand("portcullis.showOutput", () => output.show()),
    vscode.workspace.onDidSaveTextDocument((doc) => {
      if (setting<boolean>("scanOnSave", true)) {
        const folder = vscode.workspace.getWorkspaceFolder(doc.uri);
        if (folder) {
          schedule(folder);
        }
      }
    }),
    vscode.workspace.onDidChangeWorkspaceFolders((e) => {
      for (const f of e.removed) {
        clearFolder(f);
      }
      for (const f of e.added) {
        void scanFolder(f, false);
      }
    }),
    vscode.workspace.onDidChangeConfiguration((e) => {
      if (e.affectsConfiguration("portcullis")) {
        void scanAll(false);
      }
    }),
  );
  void scanAll(false);
}

export function deactivate(): void {
  for (const t of timers.values()) {
    clearTimeout(t);
  }
}

function setting<T>(key: string, fallback: T): T {
  return vscode.workspace.getConfiguration("portcullis").get<T>(key, fallback);
}

/** The binary to run: the portcullis.path setting from user or machine scope only. */
function binary(): string {
  const inspected = vscode.workspace.getConfiguration("portcullis").inspect<string>("path");
  return inspected?.globalValue || inspected?.defaultValue || "portcullis";
}

function schedule(folder: vscode.WorkspaceFolder): void {
  const key = folder.uri.toString();
  clearTimeout(timers.get(key));
  timers.set(key, setTimeout(() => void scanFolder(folder, false), 400));
}

async function scanAll(interactive: boolean): Promise<void> {
  const folders = (vscode.workspace.workspaceFolders ?? []).filter((f) => f.uri.scheme === "file");
  if (interactive && folders.length === 0) {
    void vscode.window.showInformationMessage("Portcullis scans folders on this machine; open a folder first.");
  }
  await Promise.all(folders.map((f) => scanFolder(f, interactive)));
}

async function scanFolder(folder: vscode.WorkspaceFolder, interactive: boolean): Promise<void> {
  const key = folder.uri.toString();
  const previous = running.get(key);
  if (previous) {
    await previous;
  }
  const job = run(folder, interactive).finally(() => running.delete(key));
  running.set(key, job);
  return job;
}

function exec(file: string, args: string[]): Promise<{ stdout: string; stderr: string; code: number }> {
  return new Promise((resolve, reject) => {
    execFile(file, args, { maxBuffer: 256 * 1024 * 1024, timeout: 120_000, windowsHide: true }, (err, stdout, stderr) => {
      const code = err && typeof (err as NodeJS.ErrnoException).code === "number" ? Number((err as NodeJS.ErrnoException).code) : 0;
      if (err && typeof (err as NodeJS.ErrnoException).code !== "number") {
        reject(err);
        return;
      }
      resolve({ stdout, stderr, code });
    });
  });
}

async function run(folder: vscode.WorkspaceFolder, interactive: boolean): Promise<void> {
  const root = folder.uri.fsPath;
  const bin = binary();
  status.text = "$(loading~spin) Portcullis";
  status.show();
  let report: Report;
  try {
    const { stdout, stderr, code } = await exec(bin, ["scan", "--format", "json", "--fail-on", "none", "--", root]);
    if (stderr.trim()) {
      output.appendLine(stderr.trim());
    }
    if (!stdout.trim()) {
      throw new Error(`portcullis exited with code ${code} and no report`);
    }
    report = parseReport(stdout);
  } catch (err) {
    const msg = err instanceof Error ? err.message : String(err);
    output.appendLine(`${folder.name}: ${msg}`);
    status.text = "$(warning) Portcullis";
    status.tooltip = "Portcullis could not scan this workspace; see the output.";
    const missing = (err as NodeJS.ErrnoException).code === "ENOENT";
    if (interactive || missing) {
      const action = missing ? "Install" : "Show output";
      const text = missing
        ? `Portcullis was not found ("${bin}"). Install it, or set portcullis.path in your user settings.`
        : `Portcullis could not scan ${folder.name}: ${msg}`;
      void vscode.window.showWarningMessage(text, action).then((a) => {
        if (a === "Install") {
          void vscode.env.openExternal(vscode.Uri.parse("https://github.com/cemilalp02/portcullis#install"));
        } else if (a) {
          output.show();
        }
      });
    }
    return;
  }
  publish(folder, report);
}

function clearFolder(folder: vscode.WorkspaceFolder): void {
  collection.forEach((uri) => {
    if (vscode.workspace.getWorkspaceFolder(uri)?.uri.toString() === folder.uri.toString()) {
      collection.delete(uri);
    }
  });
}

function publish(folder: vscode.WorkspaceFolder, report: Report): void {
  const min = setting<Severity>("minimumSeverity", "info");
  const cache = new Map<string, string[] | undefined>();
  const lines = (rel: string): string[] | undefined => {
    if (!cache.has(rel)) {
      let text: string | undefined;
      try {
        const open = vscode.workspace.textDocuments.find((d) => d.uri.fsPath === path.join(folder.uri.fsPath, rel));
        text = open ? open.getText() : readFileSync(path.join(folder.uri.fsPath, rel), "utf8");
      } catch {
        text = undefined;
      }
      cache.set(rel, text?.split(/\r\n|\n/));
    }
    return cache.get(rel);
  };
  const byFile = new Map<string, vscode.Diagnostic[]>();
  for (const p of problems(report, min, lines)) {
    const d = new vscode.Diagnostic(
      new vscode.Range(p.startLine, p.startChar, p.endLine, Math.min(p.endChar, 1_000_000)),
      p.message,
      p.level as number as vscode.DiagnosticSeverity,
    );
    d.source = "portcullis";
    d.code = { value: p.code, target: vscode.Uri.parse(ruleURL(p.code)) };
    const list = byFile.get(p.path) ?? [];
    list.push(d);
    byFile.set(p.path, list);
  }
  clearFolder(folder);
  for (const [rel, diags] of byFile) {
    collection.set(vscode.Uri.joinPath(folder.uri, ...rel.split("/")), diags);
  }

  const high = report.findings.filter((f) => f.severity === "high" || f.severity === "critical").length;
  const all = report.findings.length;
  status.text = high > 0 ? `$(error) Portcullis: ${high} high` : `$(pass) Portcullis${all ? `: ${all}` : ""}`;
  status.tooltip = `${all} findings, ${high} at or above high, in ${report.stats.targets} agent files (rule set ${report.tool.ruleset}).${report.complete ? "" : " The analysis is incomplete; see the problems."}`;
  status.show();
  output.appendLine(`${folder.name}: ${all} findings (${high} at or above high), ${report.stats.targets} agent files, rule set ${report.tool.ruleset}`);
}
