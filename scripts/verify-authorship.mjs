import { execFileSync } from "node:child_process";

const expectedName = "Arshiamk";
const expectedEmail = "arshia@live.co.uk";
const format = "%H%x09%an%x09%ae%x09%B%x00";

let output;
try {
  execFileSync("git", ["rev-parse", "--verify", "HEAD"], { stdio: "ignore" });
  output = execFileSync("git", ["log", "--format=" + format], {
    encoding: "utf8",
    stdio: ["ignore", "pipe", "inherit"],
  });
} catch {
  // A new repository has no commits to inspect.
  process.exit(0);
}

const violations = [];
for (const record of output.split("\0").filter((value) => value.trim())) {
  const [hash, name, email, ...messageParts] = record.trimStart().split("\t");
  const message = messageParts.join("\t");
  if (name !== expectedName || email.toLowerCase() !== expectedEmail) {
    violations.push(`${hash}: unexpected author ${name} <${email}>`);
  }
  if (/^co-authored-by:/im.test(message)) {
    violations.push(`${hash}: Co-authored-by trailer is not permitted`);
  }
}

if (violations.length > 0) {
  console.error(violations.join("\n"));
  process.exit(1);
}

console.log(`All commits are authored by ${expectedName} <${expectedEmail}>.`);
