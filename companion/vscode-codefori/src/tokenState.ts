import { randomBytes, timingSafeEqual } from "node:crypto";
import { chmod, lstat, mkdir, open, readFile, rename, rm } from "node:fs/promises";
import { homedir, platform } from "node:os";
import { isAbsolute, join } from "node:path";

export const COMPANION_TOKEN_HEADER = "x-nexus-companion-token";
const REGISTRY_DIRECTORY = "codefori-loopback-v2";
const MAX_STATE_BYTES = 1024;
const LEASE_MS = 30_000;
const POSIX = platform() !== "win32";

export type RequestAuthenticator = (headers: Readonly<Record<string, string | string[] | undefined>>) => boolean;

export interface TokenPublisher {
  publish(endpoint: string, eligibility: RegistrationEligibility): Promise<PublishedRegistration>;
}

export interface RegistrationEligibility { connected: boolean; focused: boolean; generation: number; }
export interface PublishedRegistration {
  readonly authenticate: RequestAuthenticator;
  update(eligibility: RegistrationEligibility): Promise<void>;
  remove(): Promise<void>;
}

interface TokenState {
  version: 2;
  instance: string;
  endpoint: string;
  token: string;
  generation: number;
  connected: boolean;
  focused: boolean;
  updatedAt: number;
  leaseMs: number;
}

export interface TokenStateOptions {
  configDirectory?: () => string | undefined;
  // Test seam for the ownership transition immediately before atomic quarantine.
  beforeQuarantine?: (path: string) => Promise<void>;
}

export function createTokenPublisher(options: TokenStateOptions = {}): TokenPublisher {
  const configDirectory = options.configDirectory ?? defaultConfigDirectory;
  return {
    async publish(endpoint: string, eligibility: RegistrationEligibility): Promise<PublishedRegistration> {
      const root = configDirectory();
      if (!root || !isAbsolute(root)) throw new Error("private token state unavailable");
      if (!validEndpoint(endpoint) || !Number.isSafeInteger(eligibility.generation) || eligibility.generation < 0) throw new Error("private token state unavailable");
      const directory = join(root, "bac-nexus", REGISTRY_DIRECTORY);
      try {
        await mkdir(directory, { recursive: true, mode: 0o700 });
        await requirePrivateDirectory(directory);
      } catch {
        throw new Error("private token state unavailable");
      }
      const instance = randomBytes(32).toString("base64url");
      const path = join(directory, `${instance}.json`);
      const token = randomBytes(32).toString("base64url");
      let active = true;
      const write = async (next: RegistrationEligibility): Promise<void> => {
        if (!active || !Number.isSafeInteger(next.generation) || next.generation < 0) throw new Error("private token state unavailable");
        const state: TokenState = { version: 2, instance, endpoint, token, generation: next.generation, connected: next.connected, focused: next.focused, updatedAt: Date.now(), leaseMs: LEASE_MS };
        const content = JSON.stringify(state);
        if (Buffer.byteLength(content, "utf8") > MAX_STATE_BYTES) throw new Error("private token state unavailable");
        const temporary = join(directory, `.${instance}.${randomBytes(16).toString("hex")}.tmp`);
        try {
          const handle = await open(temporary, "wx", 0o600);
          try { await handle.writeFile(content); await handle.sync(); } finally { await handle.close(); }
          if (!active) throw new Error("private token state unavailable");
          if (POSIX) await chmod(temporary, 0o600);
          await rename(temporary, path);
        } catch (error) { await rm(temporary, { force: true }).catch(() => undefined); throw error; }
      };
      await write(eligibility);
      return { authenticate: tokenAuthenticator(token), update: write, remove: async () => {
        active = false;
        await options.beforeQuarantine?.(path);
        const quarantine = join(directory, `.${instance}.${randomBytes(16).toString("hex")}.removing`);
        try {
          // rename moves exactly the path entry observed by this operation. Any
          // replacement before the move is quarantined and then retained unless
          // its authenticated random owner identity and token still match.
          await rename(path, quarantine);
        } catch {
          return;
        }
        const current = await readFile(quarantine, "utf8").catch(() => "");
        if (!ownsRegistration(current, instance, token)) return;
        await rm(quarantine, { force: true }).catch(() => undefined);
      } };
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

export function tokenStatePath(configDirectory: string): string {
  return join(configDirectory, "bac-nexus", REGISTRY_DIRECTORY);
}

function validEndpoint(endpoint: string): boolean {
  const match = /^127\.0\.0\.1:([0-9]{1,5})$/.exec(endpoint);
  return match !== null && Number(match[1]) >= 1 && Number(match[1]) <= 65535;
}

function ownsRegistration(content: string, instance: string, token: string): boolean {
  try {
    const state = JSON.parse(content) as { version?: unknown; instance?: unknown; token?: unknown };
    return state.version === 2 && state.instance === instance && state.token === token;
  } catch {
    return false;
  }
}
