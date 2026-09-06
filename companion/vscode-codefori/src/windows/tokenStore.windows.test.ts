import { execFile } from "node:child_process";
import { access, lstat, readFile } from "node:fs/promises";
import { join, win32 } from "node:path";
import { promisify } from "node:util";
import { afterEach, describe, expect, it } from "vitest";

import { createWindowsTokenStore, type TokenPublication } from "./tokenStore.js";

const executeFile = promisify(execFile);
const runNativeWindows = process.platform === "win32" && process.env.CODEFORI_WINDOWS_NATIVE_TESTS === "1";
const descriptorDirectory = process.env.LOCALAPPDATA
  ? win32.join(process.env.LOCALAPPDATA, "BAC Nexus", "companion-v1")
  : "";
const descriptorPath = descriptorDirectory ? win32.join(descriptorDirectory, "descriptor.json") : "";

const aclProbe = String.raw`
$path = $args[0]
$directory = [System.Convert]::ToBoolean($args[1])
$item = Get-Item -LiteralPath $path -Force
$sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
$acl = $item.GetAccessControl()
$rules = $acl.GetAccessRules($true, $true, [System.Security.Principal.SecurityIdentifier])
$expectedFlags = if ($directory) { [System.Security.AccessControl.InheritanceFlags]'ContainerInherit, ObjectInherit' } else { [System.Security.AccessControl.InheritanceFlags]::None }
if (-not $acl.AreAccessRulesProtected -or
    $acl.GetOwner([System.Security.Principal.SecurityIdentifier]).Value -ne $sid.Value -or
    $item.PSIsContainer -ne $directory -or
    ($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -or
    $rules.Count -ne 1) { exit 1 }
$rule = $rules[0]
if ($rule.IsInherited -or
    $rule.IdentityReference.Value -ne $sid.Value -or
    $rule.AccessControlType -ne [System.Security.AccessControl.AccessControlType]::Allow -or
    $rule.FileSystemRights -ne [System.Security.AccessControl.FileSystemRights]::FullControl -or
    $rule.InheritanceFlags -ne $expectedFlags) { exit 1 }
`;

async function assertExactCurrentUserOnlyACL(path: string, directory: boolean): Promise<void> {
  const systemRoot = process.env.SystemRoot;
  if (!systemRoot) {
    throw new Error("native Windows test host has no SystemRoot");
  }
  await executeFile(
    join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe"),
    ["-NoLogo", "-NoProfile", "-NonInteractive", "-Command", aclProbe, path, String(directory)],
    { shell: false, windowsHide: true },
  );
}

async function mustNotExist(path: string): Promise<void> {
  await expect(access(path)).rejects.toBeDefined();
}

const nativeDescribe = runNativeWindows ? describe : describe.skip;

nativeDescribe("native Windows descriptor producer security", () => {
  let publication: TokenPublication | undefined;

  afterEach(async () => {
    await publication?.cleanup();
    publication = undefined;
  });

  it("creates a clean protected descriptor, then accepts the existing valid directory without a transient readable file", async () => {
    expect(descriptorPath).not.toBe("");
    await mustNotExist(descriptorPath);

    publication = await createWindowsTokenStore().publish();
    expect(publication).toBeDefined();
    const initial = publication;
    if (!initial) {
      return;
    }
    const descriptor = await readFile(descriptorPath, "utf8");
    expect(Buffer.byteLength(descriptor, "utf8")).toBeLessThanOrEqual(512);
    expect(JSON.parse(descriptor)).toEqual({ generation: initial.generation, token: initial.token, version: 1 });
    const directoryInfo = await lstat(descriptorDirectory);
    const fileInfo = await lstat(descriptorPath);
    expect(directoryInfo.isDirectory()).toBe(true);
    expect(directoryInfo.isSymbolicLink()).toBe(false);
    expect(fileInfo.isFile()).toBe(true);
    expect(fileInfo.isSymbolicLink()).toBe(false);
    await assertExactCurrentUserOnlyACL(descriptorDirectory, true);
    await assertExactCurrentUserOnlyACL(descriptorPath, false);

    await initial.cleanup();
    publication = undefined;
    await mustNotExist(descriptorPath);

    publication = await createWindowsTokenStore().publish();
    expect(publication).toBeDefined();
    await assertExactCurrentUserOnlyACL(descriptorDirectory, true);
    await assertExactCurrentUserOnlyACL(descriptorPath, false);
  });

  it("retains generation-owned cleanup and leaves no descriptor after the owner closes it", async () => {
    publication = await createWindowsTokenStore().publish();
    expect(publication).toBeDefined();
    const owned = publication;
    if (!owned) {
      return;
    }
    await owned.cleanup();
    publication = undefined;
    await mustNotExist(descriptorPath);
  });

  it.skipIf(process.env.CODEFORI_WINDOWS_CROSS_USER_TEST !== "1")(
    "requires the native cross-user harness to prove another ordinary user cannot read the descriptor",
    async () => {
      publication = await createWindowsTokenStore().publish();
      expect(publication).toBeDefined();
      expect(process.env.CODEFORI_WINDOWS_CROSS_USER_TEST).toBe("1");
      // The hosted runner must provide a second ordinary-user probe command.
      // This test deliberately has no credential, account, or fallback mechanism.
    },
  );
});
