// End-to-end check inside VS Code, run by scripts/integration.mjs: the
// extension scans the opened sample repository and the expected problems
// appear with their exact ranges.
const assert = require("node:assert/strict");
const vscode = require("vscode");

async function waitFor(uri, n) {
  for (let i = 0; i < 150; i++) {
    const d = vscode.languages.getDiagnostics(uri).filter((x) => x.source === "portcullis");
    if (d.length >= n) {
      return d;
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error(`no Portcullis problems for ${uri.fsPath}`);
}

exports.run = async function run() {
  const folder = vscode.workspace.workspaceFolders[0].uri;
  const claude = await waitFor(vscode.Uri.joinPath(folder, "CLAUDE.md"), 1);
  const tag = claude.find((d) => d.code.value === "PCL-UNI-001");
  assert.ok(tag, "PCL-UNI-001 on CLAUDE.md");
  assert.equal(tag.severity, vscode.DiagnosticSeverity.Error);
  // The hidden text starts after "Run `make test` before committing." (34
  // characters) and is 30 Tag characters, two UTF-16 units each.
  assert.equal(tag.range.start.line, 2);
  assert.equal(tag.range.start.character, 34);
  assert.equal(tag.range.end.character, 34 + 60);
  assert.equal(tag.code.target.toString(), "https://github.com/cemilalp02/portcullis/blob/main/docs/RULES.md#pcl-uni-001");

  const settings = await waitFor(vscode.Uri.joinPath(folder, ".claude", "settings.json"), 2);
  const ids = settings.map((d) => d.code.value).sort();
  assert.deepEqual(ids, ["PCL-AEX-001", "PCL-EXE-001"]);
  assert.equal(settings.find((d) => d.code.value === "PCL-AEX-001").severity, vscode.DiagnosticSeverity.Hint);

  // Editing a file and saving it scans again.
  const doc = await vscode.workspace.openTextDocument(vscode.Uri.joinPath(folder, "CLAUDE.md"));
  const edit = new vscode.WorkspaceEdit();
  edit.replace(doc.uri, new vscode.Range(0, 0, doc.lineCount, 0), "# Notes\nRun `make test` before committing.\n");
  await vscode.workspace.applyEdit(edit);
  await doc.save();
  for (let i = 0; i < 150; i++) {
    if (vscode.languages.getDiagnostics(doc.uri).filter((x) => x.source === "portcullis").length === 0) {
      return;
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error("the problem stayed after the hidden text was removed");
};
