import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(scriptDir, "..");
const schemaPath = path.join(root, "schema", "flightcheck.schema.json");
const schema = JSON.parse(fs.readFileSync(schemaPath, "utf8"));

const ajv = new Ajv2020({
  allErrors: true,
  strict: true,
  validateFormats: true,
});
addFormats(ajv);
ajv.compile(schema);
const validateRule = ajv.compile({
  $schema: schema.$schema,
  $ref: "#/$defs/Rule",
  $defs: schema.$defs,
});

const packsDir = path.resolve(root, "..", "rule-packs");
for (const entry of fs.readdirSync(packsDir).filter((name) => name.endsWith(".json"))) {
  const pack = JSON.parse(fs.readFileSync(path.join(packsDir, entry), "utf8"));
  if (!Array.isArray(pack.rules) || pack.rules.length === 0) {
    throw new Error(`${entry} must contain a non-empty rules array`);
  }
  for (const rule of pack.rules) {
    if (!validateRule(rule)) {
      throw new Error(`${entry}:${rule.id ?? "unknown"} is invalid: ${ajv.errorsText(validateRule.errors)}`);
    }
  }
}

for (const relative of [
  "generated/typescript/contracts.ts",
  "generated/python/contracts.py",
  "generated/go/contracts.go",
]) {
  const generatedPath = path.join(root, relative);
  if (!fs.existsSync(generatedPath) || fs.statSync(generatedPath).size === 0) {
    throw new Error(`Missing generated contract: ${relative}`);
  }
}

console.log("Contract schema and generated artifacts are valid.");
