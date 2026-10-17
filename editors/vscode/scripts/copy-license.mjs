// vsce packages the LICENSE next to package.json; the repository keeps one copy at its root.
import { copyFileSync } from "node:fs";
copyFileSync(new URL("../../../LICENSE", import.meta.url), new URL("../LICENSE", import.meta.url));
