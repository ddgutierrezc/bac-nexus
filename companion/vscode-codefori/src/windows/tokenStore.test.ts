import { EventEmitter } from "node:events";
import { readFile } from "node:fs/promises";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  randomBytes: vi.fn((size: number) => Buffer.alloc(size, 7)),
  spawn: vi.fn(),
}));

vi.mock("node:child_process", () => ({ spawn: mocks.spawn }));
vi.mock("node:crypto", () => ({ randomBytes: mocks.randomBytes }));

import { createWindowsTokenStore } from "./tokenStore.js";

class FakeInput extends EventEmitter {
  readonly writes: Buffer[] = [];
  ended = false;

  write(value: string | Uint8Array): boolean {
    this.writes.push(Buffer.from(value));
    return true;
  }

  end(): void {
    this.ended = true;
  }
}

class FakeChild extends EventEmitter {
  readonly stdin = new FakeInput();
  readonly stdout = new EventEmitter();
  readonly stderr = new EventEmitter();
  killed = false;

  kill(): boolean {
    this.killed = true;
    return true;
  }

  ready(): void {
    this.stdout.emit("data", Buffer.from("PROCESS_ENTRY\nDIRECTORY_VALIDATED\nREADY\n"));
  }

  cleaned(): void {
    this.stdout.emit("data", Buffer.from("CLEANED\n"));
  }

  close(code = 0): void {
    this.emit("close", code);
  }
}

function scriptName(call: unknown): string {
  const argumentsList = (call as [string, string[]])[1];
  return argumentsList[argumentsList.length - 1] ?? "";
}

describe("fixed Windows descriptor token store", () => {
  beforeEach(() => {
    mocks.spawn.mockReset();
    mocks.randomBytes.mockClear();
    process.env.SystemRoot = "C:\\Windows";
    process.env.LOCALAPPDATA = "C:\\Users\\operator\\AppData\\Local";
  });

  it("waits for the fixed publish readiness signal before generating and sending a stdin-only secret", async () => {
    const child = new FakeChild();
    mocks.spawn.mockReturnValueOnce(child);

    const publicationPromise = createWindowsTokenStore().publish();

    expect(mocks.spawn).toHaveBeenCalledTimes(1);
    const [executable, argumentsList, options] = mocks.spawn.mock.calls[0] as [
      string,
      string[],
      { env: Record<string, string>; shell: boolean; stdio: string[]; timeout: number },
    ];
    expect(executable).toBe("C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe");
    expect(argumentsList).toEqual([
      "-NoLogo",
      "-NoProfile",
      "-NonInteractive",
      "-ExecutionPolicy",
      "Bypass",
      "-File",
      expect.stringMatching(/publish\.ps1$/),
    ]);
    expect(scriptName(mocks.spawn.mock.calls[0])).toMatch(/\/windows\/publish\.ps1$/);
    expect(options).toEqual({
      env: { LOCALAPPDATA: "C:\\Users\\operator\\AppData\\Local", SystemRoot: "C:\\Windows" },
      shell: false,
      stdio: ["pipe", "pipe", "pipe"],
      timeout: 2_000,
      windowsHide: true,
    });
    expect(mocks.randomBytes).not.toHaveBeenCalled();
    expect(child.stdin.writes).toEqual([]);

    child.ready();
    child.close();

    const publication = await publicationPromise;
    expect(publication).toMatchObject({
      generation: "07070707070707070707070707070707",
      token: "0707070707070707070707070707070707070707070707070707070707070707",
    });
    expect(mocks.randomBytes).toHaveBeenCalledTimes(2);
    expect(child.stdin.ended).toBe(true);
    expect(JSON.parse(child.stdin.writes[0]?.toString() ?? "")).toEqual({
      generation: "07070707070707070707070707070707",
      token: publication?.token,
      version: 1,
    });
  });

  it("reports only the completed fixed stage when transcript output stops being a prefix", async () => {
    const child = new FakeChild();
    mocks.spawn.mockReturnValueOnce(child);
    const stages: string[] = [];

    const publicationPromise = createWindowsTokenStore((stage) => stages.push(stage)).publish();
    child.stdout.emit("data", Buffer.from("PROCESS_ENTRY\nunexpected child output\n"));
    child.close();

    await expect(publicationPromise).resolves.toBeUndefined();
    expect(mocks.spawn).toHaveBeenCalledTimes(1);
    expect(mocks.randomBytes).not.toHaveBeenCalled();
    expect(child.killed).toBe(true);
    expect(stages).toEqual(["process_entry"]);
  });

  it("accepts the exact fixed transcript across stdout chunk boundaries before sending a secret", async () => {
    const child = new FakeChild();
    mocks.spawn.mockReturnValueOnce(child);
    const stages: string[] = [];

    const publicationPromise = createWindowsTokenStore((stage) => stages.push(stage)).publish();
    child.stdout.emit("data", Buffer.from("PROCESS_ENTRY\nDIRECTORY_"));
    expect(stages).toEqual(["process_entry"]);
    expect(mocks.randomBytes).not.toHaveBeenCalled();

    child.stdout.emit("data", Buffer.from("VALIDATED\nREA"));
    expect(stages).toEqual(["process_entry", "directory_validated"]);
    expect(mocks.randomBytes).not.toHaveBeenCalled();

    child.stdout.emit("data", Buffer.from("DY\n"));
    child.close();
    await expect(publicationPromise).resolves.toMatchObject({
      generation: "07070707070707070707070707070707",
    });
    expect(stages).toEqual(["process_entry", "directory_validated", "ready"]);
  });

  it("terminates bounded stdio and deadline failures without exposing child details", async () => {
    vi.useFakeTimers();
    const outputChild = new FakeChild();
    const timeoutChild = new FakeChild();
    mocks.spawn.mockReturnValueOnce(outputChild).mockReturnValueOnce(timeoutChild);

    const outputPromise = createWindowsTokenStore().publish();
    outputChild.stderr.emit("data", Buffer.alloc(257, 1));
    outputChild.close(1);
    await expect(outputPromise).resolves.toBeUndefined();
    expect(outputChild.killed).toBe(true);

    const timeoutPromise = createWindowsTokenStore().publish();
    await vi.advanceTimersByTimeAsync(2_000);
    await expect(timeoutPromise).resolves.toBeUndefined();
    expect(timeoutChild.killed).toBe(true);
    expect(mocks.randomBytes).not.toHaveBeenCalled();
    vi.useRealTimers();
  });

  it("fails closed on any child stderr even when its fixed stdout and exit code look successful", async () => {
    const child = new FakeChild();
    const cleanupChild = new FakeChild();
    mocks.spawn.mockReturnValueOnce(child).mockReturnValueOnce(cleanupChild);

    const publicationPromise = createWindowsTokenStore().publish();
    child.ready();
    child.stderr.emit("data", Buffer.from("raw internal failure for activation-token"));
    child.close();

    for (let attempt = 0; attempt < 10 && mocks.spawn.mock.calls.length !== 2; attempt += 1) {
      await Promise.resolve();
    }
    cleanupChild.cleaned();
    cleanupChild.close();
    await expect(publicationPromise).resolves.toBeUndefined();
    expect(child.killed).toBe(true);
  });

  it("uses the fixed cleanup script and removes only the generated activation generation", async () => {
    const publishChild = new FakeChild();
    const cleanupChild = new FakeChild();
    mocks.spawn.mockReturnValueOnce(publishChild).mockReturnValueOnce(cleanupChild);

    const publicationPromise = createWindowsTokenStore().publish();
    publishChild.ready();
    publishChild.close();
    const publication = await publicationPromise;
    expect(publication).toBeDefined();

    const cleanupPromise = publication?.cleanup();
    expect(mocks.spawn).toHaveBeenCalledTimes(2);
    const [, cleanupArguments, cleanupOptions] = mocks.spawn.mock.calls[1] as [
      string,
      string[],
      { env: Record<string, string>; shell: boolean; stdio: string[]; timeout: number },
    ];
    expect(cleanupArguments).toEqual([
      "-NoLogo",
      "-NoProfile",
      "-NonInteractive",
      "-ExecutionPolicy",
      "Bypass",
      "-File",
      expect.stringMatching(/cleanup\.ps1$/),
    ]);
    expect(cleanupOptions).toEqual({
      env: { LOCALAPPDATA: "C:\\Users\\operator\\AppData\\Local", SystemRoot: "C:\\Windows" },
      shell: false,
      stdio: ["pipe", "pipe", "pipe"],
      timeout: 2_000,
      windowsHide: true,
    });
    expect(JSON.parse(cleanupChild.stdin.writes[0]?.toString() ?? "")).toEqual({
      generation: publication?.generation,
    });

    cleanupChild.cleaned();
    cleanupChild.close();

    await expect(cleanupPromise).resolves.toBeUndefined();
  });

  it("cleans up a generated descriptor after a failed publish without retrying publication", async () => {
    const publishChild = new FakeChild();
    const cleanupChild = new FakeChild();
    mocks.spawn.mockReturnValueOnce(publishChild).mockReturnValueOnce(cleanupChild);

    const publicationPromise = createWindowsTokenStore().publish();
    publishChild.ready();
    publishChild.close(1);

    for (let attempt = 0; attempt < 10 && mocks.spawn.mock.calls.length !== 2; attempt += 1) {
      await Promise.resolve();
    }
    expect(mocks.spawn).toHaveBeenCalledTimes(2);
    expect(scriptName(mocks.spawn.mock.calls[1])).toMatch(/cleanup\.ps1$/);
    expect(cleanupChild.stdin.writes[0]?.toString()).toContain('"generation"');
    expect(cleanupChild.stdin.writes[0]?.toString()).not.toContain('"token"');

    cleanupChild.cleaned();
    cleanupChild.close();

    await expect(publicationPromise).resolves.toBeUndefined();
  });

  it("does not begin a Windows process when the fixed host environment is unavailable", async () => {
    const originalSystemRoot = process.env.SystemRoot;
    const originalLocalApplicationData = process.env.LOCALAPPDATA;
    delete process.env.SystemRoot;
    delete process.env.LOCALAPPDATA;

    await expect(createWindowsTokenStore().publish()).resolves.toBeUndefined();
    expect(mocks.spawn).not.toHaveBeenCalled();

    process.env.SystemRoot = originalSystemRoot;
    process.env.LOCALAPPDATA = originalLocalApplicationData;
  });

  it("keeps the two inbox scripts limited to protected descriptor publication and owned cleanup", async () => {
    const publish = await readFile(new URL("./publish.ps1", import.meta.url), "utf8");
    const cleanup = await readFile(new URL("./cleanup.ps1", import.meta.url), "utf8");
    const nativeTest = await readFile(new URL("./tokenStore.windows.test.ts", import.meta.url), "utf8");

    expect(publish).toContain("[System.IO.Directory]::CreateDirectory($directoryPath, $directorySecurity)");
    expect(publish).toContain("$directorySecurity.SetAccessRuleProtection($true, $false)");
    expect(publish).toContain("$fileSecurity.SetAccessRuleProtection($true, $false)");
    expect(publish).toContain("[System.IO.FileMode]::CreateNew");
    expect(publish).toContain("$fileItem.Attributes -band [System.IO.FileAttributes]::ReparsePoint");
    expect(publish).toMatch(
      /\[Console\]::Out\.Write\("PROCESS_ENTRY`n"\)\r?\n\s*\[Console\]::Out\.Flush\(\)\r?\n[\s\S]*\[Console\]::Out\.Write\("DIRECTORY_VALIDATED`n"\)\r?\n\s*\[Console\]::Out\.Flush\(\)\r?\n\s*\[Console\]::Out\.Write\("READY`n"\)\r?\n\s*\[Console\]::Out\.Flush\(\)\r?\n\s*\$input = \[Console\]::In\.ReadToEnd\(\)/,
    );
    expect(publish).toContain('[Console]::Out.Write("READY`n")');
    expect(publish).toMatch(
      /\[Console\]::Out\.Write\("READY`n"\)\r?\n\s*\[Console\]::Out\.Flush\(\)\r?\n\s*\$input = \[Console\]::In\.ReadToEnd\(\)/,
    );
    expect(cleanup).toContain("$descriptor.generation -ne $request.generation");
    expect(cleanup).toContain('[Console]::Out.Write("CLEANED`n")');
    expect(nativeTest).toContain("async function publishWithFixedStageEvidence");
    expect(nativeTest).toContain("throw new Error(`windows fixed stage: ${lastStage}`)");
    expect(publish).not.toContain("param(");
    expect(cleanup).not.toContain("param(");
  });
});
