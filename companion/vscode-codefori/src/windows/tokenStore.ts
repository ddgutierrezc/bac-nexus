import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { fileURLToPath } from "node:url";
import { isAbsolute, win32 } from "node:path";

const PROCESS_DEADLINE_MS = 1_000;
const MAX_STDOUT_BYTES = 64;
const MAX_STDERR_BYTES = 256;
const MAX_DESCRIPTOR_BYTES = 512;
const POWERSHELL_ARGUMENTS = [
  "-NoLogo",
  "-NoProfile",
  "-NonInteractive",
  "-ExecutionPolicy",
  "Bypass",
  "-File",
] as const;

export interface TokenPublication {
  readonly generation: string;
  readonly token: string;
  cleanup(): Promise<void>;
}

export interface WindowsTokenStore {
  publish(): Promise<TokenPublication | undefined>;
}

interface SpawnedProcess {
  readonly stdin: {
    end(): void;
    write(value: Uint8Array): boolean;
  };
  readonly stdout: {
    on(event: "data", listener: (chunk: Uint8Array) => void): unknown;
  };
  readonly stderr: {
    on(event: "data", listener: (chunk: Uint8Array) => void): unknown;
  };
  kill(): boolean;
  once(event: "close", listener: (code: number | null) => void): unknown;
  once(event: "error", listener: () => void): unknown;
}

export function createWindowsTokenStore(): WindowsTokenStore {
  return {
    async publish(): Promise<TokenPublication | undefined> {
      const environment = fixedEnvironment();
      if (!environment) {
        return undefined;
      }

      let descriptor: Uint8Array | undefined;
      let generation = "";
      let token = "";
      const published = await runFixedOperation({
        environment,
        expectedOutput: "READY\n",
        script: inboxScript("publish.ps1"),
        writeInputAfterReady: () => {
          generation = randomBytes(16).toString("hex");
          token = randomBytes(32).toString("hex");
          descriptor = encodeInput({ version: 1, generation, token });
          return descriptor;
        },
      });
      const cleanup = async (): Promise<void> => {
        await runFixedOperation({
          environment,
          expectedOutput: "CLEANED\n",
          input: encodeInput({ generation }),
          script: inboxScript("cleanup.ps1"),
        });
      };
      if (!published || !descriptor) {
        if (descriptor) {
          await cleanup();
        }
        return undefined;
      }

      return {
        generation,
        token,
        cleanup,
      };
    },
  };
}

function fixedEnvironment(): Readonly<Record<string, string>> | undefined {
  const systemRoot = process.env.SystemRoot;
  const localApplicationData = process.env.LOCALAPPDATA;
  if (
    !systemRoot ||
    !localApplicationData ||
    !win32.isAbsolute(systemRoot) ||
    !win32.isAbsolute(localApplicationData)
  ) {
    return undefined;
  }
  return { LOCALAPPDATA: localApplicationData, SystemRoot: systemRoot };
}

function inboxScript(name: "publish.ps1" | "cleanup.ps1"): string {
  const script = fileURLToPath(new URL(`./${name}`, import.meta.url));
  if (!isAbsolute(script)) {
    throw new Error("inbox script resolution failed");
  }
  return script;
}

function encodeInput(value: object): Uint8Array {
  const input = new TextEncoder().encode(JSON.stringify(value));
  if (input.byteLength > MAX_DESCRIPTOR_BYTES) {
    throw new Error("fixed descriptor input exceeded its bound");
  }
  return input;
}

interface FixedOperation {
  readonly environment: Readonly<Record<string, string>>;
  readonly expectedOutput: "READY\n" | "CLEANED\n";
  readonly input?: Uint8Array;
  readonly script: string;
  readonly writeInputAfterReady?: () => Uint8Array;
}

function runFixedOperation(operation: FixedOperation): Promise<boolean> {
  const systemRoot = operation.environment.SystemRoot;
  if (!systemRoot) {
    return Promise.resolve(false);
  }
  let child: SpawnedProcess;
  try {
    child = spawn(
      win32.join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe"),
      [...POWERSHELL_ARGUMENTS, operation.script],
      {
        env: operation.environment,
        shell: false,
        stdio: ["pipe", "pipe", "pipe"],
        timeout: PROCESS_DEADLINE_MS,
        windowsHide: true,
      },
    ) as unknown as SpawnedProcess;
  } catch {
    return Promise.resolve(false);
  }

  return new Promise((resolve) => {
    let complete = false;
    let stdout = Buffer.alloc(0);
    let stderrBytes = 0;
    let inputWritten = false;
    const deadline = setTimeout(() => finish(false), PROCESS_DEADLINE_MS);

    const finish = (result: boolean): void => {
      if (complete) {
        return;
      }
      complete = true;
      clearTimeout(deadline);
      if (!result) {
        child.kill();
        child.stdin.end();
      }
      resolve(result);
    };

    child.stdout.on("data", (chunk) => {
      if (complete) {
        return;
      }
      stdout = Buffer.concat([stdout, Buffer.from(chunk)]);
      if (stdout.byteLength > MAX_STDOUT_BYTES || stdout.toString("utf8") !== operation.expectedOutput) {
        if (!operation.expectedOutput.startsWith(stdout.toString("utf8"))) {
          finish(false);
        }
        return;
      }
      if (!inputWritten && operation.writeInputAfterReady) {
        inputWritten = true;
        try {
          child.stdin.write(operation.writeInputAfterReady());
          child.stdin.end();
        } catch {
          finish(false);
        }
      }
    });
    child.stderr.on("data", (chunk) => {
      stderrBytes += Buffer.byteLength(chunk);
      if (stderrBytes > 0 && stderrBytes <= MAX_STDERR_BYTES) {
        finish(false);
        return;
      }
      if (stderrBytes > MAX_STDERR_BYTES) {
        finish(false);
      }
    });
    child.once("error", () => finish(false));
    child.once("close", (code) =>
      finish(
        code === 0 &&
          stdout.toString("utf8") === operation.expectedOutput &&
          (operation.input !== undefined || inputWritten),
      ),
    );

    if (operation.input) {
      try {
        child.stdin.write(operation.input);
        child.stdin.end();
      } catch {
        finish(false);
      }
    }
  });
}
