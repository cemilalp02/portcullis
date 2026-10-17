# Portcullis for VS Code

Shows what [Portcullis](https://github.com/cemilalp02/portcullis) finds in the files AI coding
agents read and run (`CLAUDE.md`, `AGENTS.md`, skills, Copilot and Cursor rules, hooks, tasks,
dev containers, MCP servers and the scripts they run) as problems in the editor: hidden
instructions, commands that run without asking, invisible Unicode and look-alike letters.

- Opens a repository you do not trust yet? The extension works in Restricted Mode: it only
  reads files, and the scan ignores the repository's own `.portcullis.yaml`.
- Each problem links to its rule in the [catalogue](https://github.com/cemilalp02/portcullis/blob/main/docs/RULES.md)
  and says how to fix it. Hidden Unicode Tag text is decoded in the message.
- The workspace is scanned when it opens and again when a file is saved; the status bar shows
  how many findings are at or above `high`.

## Requirements

The `portcullis` executable, version 0.1 or newer, on your `PATH`
([install](https://github.com/cemilalp02/portcullis#install)), or its location in the
`portcullis.path` setting.

## Settings

| Setting | Default | |
|---|---|---|
| `portcullis.path` | `portcullis` | The executable. Only user and machine settings are read, so a workspace cannot choose the program the extension runs. |
| `portcullis.scanOnSave` | `true` | Scan again when a file is saved. |
| `portcullis.minimumSeverity` | `info` | Lowest severity shown. `info` findings list the commands that run without asking. |

## Commands

- **Portcullis: Scan workspace**
- **Portcullis: Show output**

## Development

```bash
npm ci
npm test                                          # unit tests
node scripts/integration.mjs path/to/portcullis /path/to/Code.exe   # end-to-end in an isolated VS Code
npm run package                                   # portcullis.vsix
```

Apache License 2.0.
