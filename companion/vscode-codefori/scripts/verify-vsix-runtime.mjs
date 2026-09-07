import { execFileSync } from "node:child_process";
import console from "node:console";
import { existsSync, mkdtempSync, mkdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { basename, join, resolve } from "node:path";
import process from "node:process";
import { pathToFileURL } from "node:url";

const workspace = process.cwd();
const temporaryDirectory = mkdtempSync(join(tmpdir(), "nexus-companion-vsix-"));
const requestedVSIX = process.argv[2];

try {
  const vsixPath = requestedVSIX
    ? resolve(workspace, requestedVSIX)
    : packageVSIX(temporaryDirectory);
  const extensionEntrypoint = extractEntrypoint(vsixPath, temporaryDirectory);

  await import(pathToFileURL(extensionEntrypoint).href);
  console.log(`Verified packaged runtime import closure for ${basename(vsixPath)}.`);
} finally {
  rmSync(temporaryDirectory, { force: true, recursive: true });
}

function packageVSIX(directory) {
  const vsixPath = join(directory, "nexus-codefori-companion.vsix");
  execFileSync("npm", ["run", "package:vsix", "--", "--out", vsixPath], {
    cwd: workspace,
    stdio: "inherit",
  });
  return vsixPath;
}

function extractEntrypoint(vsixPath, directory) {
  if (!existsSync(vsixPath)) {
    throw new Error(`VSIX file does not exist: ${vsixPath}`);
  }

  const extractedDirectory = join(directory, "extracted");
  mkdirSync(extractedDirectory);
  execFileSync("unzip", ["-q", vsixPath, "-d", extractedDirectory], {
    stdio: "inherit",
  });

  const entrypoint = join(extractedDirectory, "extension", "dist", "extension.js");
  if (!existsSync(entrypoint)) {
    throw new Error("Packaged VSIX does not contain extension/dist/extension.js.");
  }
  return entrypoint;
}
