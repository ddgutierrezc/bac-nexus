import { randomBytes, timingSafeEqual } from "node:crypto";
import { chmod, lstat, mkdir, open, rename, rm } from "node:fs/promises";
import { homedir, platform } from "node:os";
import { isAbsolute, join } from "node:path";

export const COMPANION_TOKEN_HEADER = "x-nexus-companion-token";
const STATE_FILENAME = "codefori-loopback-token-v1.json";
const MAX_STATE_BYTES = 256;
const POSIX = platform() !== "win32";

export type RequestAuthenticator = (headers: Readonly<Record<string, string | string[] | undefined>>) => boolean;

export interface TokenPublisher {
  publish(): Promise<RequestAuthenticator>;
}

interface TokenState {
  version: 1;
  token: string;
}

export interface TokenStateOptions {
  configDirectory?: () => string | undefined;
}

export function createTokenPublisher(options: TokenStateOptions = {}): TokenPublisher {
  const configDirectory = options.configDirectory ?? defaultConfigDirectory;
  return {
    async publish(): Promise<RequestAuthenticator> {
      const root = configDirectory();
      if (!root || !isAbsolute(root)) throw new Error("private token state unavailable");
      const directory = join(root, "bac-nexus");
      try {
        await mkdir(directory, { recursive: true, mode: 0o700 });
        await requirePrivateDirectory(directory);
      } catch {
        throw new Error("private token state unavailable");
      }
      const path = join(directory, STATE_FILENAME);
      await rejectUnsafeExisting(path);
      const token = randomBytes(32).toString("base64url");
      const temporary = join(directory, `.${STATE_FILENAME}.${randomBytes(16).toString("hex")}.tmp`);
      const content = JSON.stringify({ version: 1, token } satisfies TokenState);
      if (Buffer.byteLength(content, "utf8") > MAX_STATE_BYTES) throw new Error("private token state unavailable");
      try {
        const handle = await open(temporary, "wx", 0o600);
        try {
          await handle.writeFile(content);
          await handle.sync();
        } finally {
          await handle.close();
        }
        if (POSIX) await chmod(temporary, 0o600);
        await rename(temporary, path);
      } catch (error) {
        await rm(temporary, { force: true }).catch(() => undefined);
        throw error;
      }
      return tokenAuthenticator(token);
    },
  };
}

export function tokenAuthenticator(token: string): RequestAuthenticator {
  const expected = Buffer.from(token, "utf8");
  return (headers): boolean => {
    const provided = headers[COMPANION_TOKEN_HEADER];
    if (typeof provided !== "string") return false;
    const actual = Buffer.from(provided, "utf8");
    return actual.byteLength === expected.byteLength && timingSafeEqual(actual, expected);
  };
}

function defaultConfigDirectory(): string | undefined {
  if (platform() === "win32") return process.env.APPDATA;
  if (platform() === "darwin") return join(homedir(), "Library", "Application Support");
  return process.env.XDG_CONFIG_HOME || join(homedir(), ".config");
}

async function requirePrivateDirectory(path: string): Promise<void> {
  const info = await lstat(path);
  if (!info.isDirectory() || info.isSymbolicLink() || (POSIX && (info.mode & 0o077) !== 0)) {
    throw new Error("private token state unavailable");
  }
  if (POSIX && typeof process.getuid === "function" && info.uid !== process.getuid()) {
    throw new Error("private token state unavailable");
  }
}

async function rejectUnsafeExisting(path: string): Promise<void> {
  try {
    const info = await lstat(path);
    if (!info.isFile() || info.isSymbolicLink() || (POSIX && (info.mode & 0o077) !== 0)) {
      throw new Error("private token state unavailable");
    }
    if (POSIX && typeof process.getuid === "function" && info.uid !== process.getuid()) {
      throw new Error("private token state unavailable");
    }
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
  }
}

export function tokenStatePath(configDirectory: string): string {
  return join(configDirectory, "bac-nexus", STATE_FILENAME);
}
