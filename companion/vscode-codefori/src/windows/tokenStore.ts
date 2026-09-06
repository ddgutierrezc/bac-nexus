import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { fileURLToPath } from "node:url";
import { isAbsolute, win32 } from "node:path";

const PRE_READY_DEADLINE_MS = 10_000;
const POST_READY_DEADLINE_MS = 2_000;
const PUBLISH_PROCESS_DEADLINE_MS = PRE_READY_DEADLINE_MS + POST_READY_DEADLINE_MS;
const CLEANUP_DEADLINE_MS = 10_000;
const MAX_STDOUT_BYTES = 104;
const MAX_STDERR_BYTES = 256;
const MAX_DESCRIPTOR_BYTES = 512;
const PUBLISH_STAGES = [
  { output: "PROCESS_ENTRY\n", stage: "process_entry" },
  { output: "SID_READY\n", stage: "sid_ready" },
  { output: "ROOT_READY\n", stage: "root_ready" },
  { output: "DIRECTORY_SECURITY_READY\n", stage: "directory_security_ready" },
  { output: "DIRECTORY_CREATED\n", stage: "directory_created" },
  { output: "DIRECTORY_VALIDATED\n", stage: "directory_validated" },
  { output: "READY\n", stage: "ready" },
] as const;
const PUBLISH_TRANSCRIPT = PUBLISH_STAGES.map(({ output }) => output).join("");
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

type FixedStage = (typeof PUBLISH_STAGES)[number];
type FixedStageObserver = (stage: FixedStage["stage"]) => void;

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

export function createWindowsTokenStore(onStage?: FixedStageObserver): WindowsTokenStore {
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
        expectedOutput: PUBLISH_TRANSCRIPT,
        onStage,
        deadlineMs: PRE_READY_DEADLINE_MS,
        postReadyDeadlineMs: POST_READY_DEADLINE_MS,
        processDeadlineMs: PUBLISH_PROCESS_DEADLINE_MS,
        script: inboxScript("publish.ps1"),
        stageTranscript: PUBLISH_STAGES,
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
          deadlineMs: CLEANUP_DEADLINE_MS,
          processDeadlineMs: CLEANUP_DEADLINE_MS,
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
  readonly deadlineMs: number;
  readonly environment: Readonly<Record<string, string>>;
  readonly expectedOutput: string;
  readonly input?: Uint8Array;
  readonly onStage?: FixedStageObserver | undefined;
  readonly postReadyDeadlineMs?: number;
  readonly processDeadlineMs: number;
  readonly script: string;
  readonly stageTranscript?: readonly FixedStage[];
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
        timeout: operation.processDeadlineMs,
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
    let reportedStages = 0;
    let deadline = setTimeout(() => finish(false), operation.deadlineMs);

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
      const output = stdout.toString("utf8");
      while (operation.stageTranscript && reportedStages < operation.stageTranscript.length) {
        const stage = operation.stageTranscript[reportedStages];
        if (!stage) {
          break;
        }
        const transcript = operation.stageTranscript.slice(0, reportedStages + 1).map(({ output: part }) => part).join("");
        if (!output.startsWith(transcript)) {
          break;
        }
        try {
          operation.onStage?.(stage.stage);
        } catch {
          finish(false);
          return;
        }
        reportedStages += 1;
      }
      if (stdout.byteLength > MAX_STDOUT_BYTES || !operation.expectedOutput.startsWith(output)) {
        finish(false);
        return;
      }
      if (output !== operation.expectedOutput) {
        return;
      }
      if (!inputWritten && operation.writeInputAfterReady) {
        inputWritten = true;
        if (operation.postReadyDeadlineMs !== undefined) {
          clearTimeout(deadline);
          deadline = setTimeout(() => finish(false), operation.postReadyDeadlineMs);
        }
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
