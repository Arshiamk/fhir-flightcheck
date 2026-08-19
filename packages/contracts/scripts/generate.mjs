import { spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import { fileURLToPath } from "node:url";
import path from "node:path";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(scriptDir, "..");
const schema = path.join(root, "schema", "flightcheck.schema.json");
const quicktype = path.resolve(root, "..", "..", "node_modules", "quicktype", "dist", "index.js");
const contract = JSON.parse(fs.readFileSync(schema, "utf8"));
const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "flightcheck-contracts-"));
const schemaNames = ["Rule", "RunManifest", "Finding", "Evidence", "Report", "ApiProblem"];
const sources = schemaNames.map((name) => {
  const source = path.join(tempDir, `${name}.schema.json`);
  fs.writeFileSync(
    source,
    JSON.stringify({
      $schema: contract.$schema,
      title: name,
      $ref: `#/$defs/${name}`,
      $defs: contract.$defs,
    }),
  );
  return source;
});
const targets = [
  ["typescript", path.join(root, "generated", "typescript", "contracts.ts"), []],
  ["python", path.join(root, "generated", "python", "contracts.py"), ["--python-version", "3.7"]],
  ["go", path.join(root, "generated", "go", "contracts.go"), ["--package", "contracts"]],
];

for (const [language, output, extra] of targets) {
  const result = spawnSync(
    process.execPath,
    [
      quicktype,
      "--src-lang",
      "schema",
      "--lang",
      language,
      "--out",
      output,
      ...sources.flatMap((source) => ["--src", source]),
      ...extra,
    ],
    { cwd: path.resolve(root, "..", ".."), encoding: "utf8" },
  );

  if (result.status !== 0) {
    process.stderr.write(result.stderr || result.stdout || result.error?.message || "Contract generation failed.");
    process.exit(result.status ?? 1);
  }
}

fs.rmSync(tempDir, { recursive: true, force: true });
console.log("Generated Go, Python, and TypeScript contracts.");
