import { createRequire } from "node:module";

import { createBroker, createCodeForIBrokerHandler, type BrokerRequest, type BrokerResponse, type FixedLoopbackServer } from "./broker.js";
import { createCodeForIAdapter, type CodeForIExports } from "./codeforiAdapter.js";
import { createDiagnosticsUI, DIAGNOSTICS_COMMAND, type OutputChannel, type StatusBarItem } from "./diagnostics.js";
import { createHTTPServer } from "./httpServer.js";
import { createTokenPublisher, type RequestAuthenticator, type TokenPublisher } from "./tokenState.js";

const CODE_FOR_I_EXTENSION_ID = "halcyontechltd.code-for-ibmi";
const COMPANION_VERSION = "0.2.6";
type ServerFactory = (handler: (request: BrokerRequest) => Promise<BrokerResponse>, authenticate: RequestAuthenticator) => FixedLoopbackServer;

interface Extension<T> {
  activate(): Promise<T | undefined>;
  exports: T | undefined;
  packageJSON?: { version?: unknown };
}

export interface ExtensionHost {
  getExtension<T>(identifier: string): Extension<T> | undefined;
}

interface VSCodeHost {
  extensions: ExtensionHost;
  window: {
    createStatusBarItem(id: string, alignment: number, priority: number): StatusBarItem;
    createOutputChannel(name: string): OutputChannel;
  };
  commands: {
    registerCommand(command: string, callback: () => void): { dispose(): void };
  };
  StatusBarAlignment: { Left: number };
}

export interface ActivationOptions {
  extensionHost?: ExtensionHost;
  serverFactory?: ServerFactory;
  vscodeHost?: VSCodeHost;
  tokenPublisher?: TokenPublisher;
}

let owned: { broker: ReturnType<typeof createBroker>; deactivate(): void; dispose(): void } | undefined;

export async function activate(context: unknown, options: ActivationOptions = {}): Promise<void> {
  await deactivate();
  const vscode = options.vscodeHost ?? loadVSCodeHost();
  const extension = (options.extensionHost ?? vscode?.extensions)?.getExtension<CodeForIExports>(
    CODE_FOR_I_EXTENSION_ID,
  );
  let exports = extension?.exports;
  let activation = "unavailable" as "active" | "unavailable";
  try {
    exports = (await extension?.activate()) ?? exports;
    activation = exports ? "active" : "unavailable";
  } catch {
    exports = undefined;
  }

  const adapter = createCodeForIAdapter(exports, context);
  const broker = createBroker({
    serverFactory: options.serverFactory ?? createHTTPServer,
    handler: createCodeForIBrokerHandler(adapter),
    tokenPublisher: options.tokenPublisher ?? createTokenPublisher(),
  });
  const listening = await broker.start();
  if (!vscode) {
    owned = { broker, deactivate: adapter.deactivate, dispose: () => undefined };
    return;
  }

  const diagnostics = createDiagnosticsUI(
    vscode.window.createStatusBarItem("nexus-codefori-companion.status", vscode.StatusBarAlignment.Left, 100),
    vscode.window.createOutputChannel("Nexus Companion"),
    () => ({
      listener: listening ? "listening" : "unavailable",
      codeForIExtension: extension ? "found" : "unavailable",
      codeForIActivation: activation,
      codeForIVersion: extensionVersion(extension?.packageJSON?.version),
      companionVersion: COMPANION_VERSION,
      adapter: adapter.diagnostics(),
    }),
  );
  const command = vscode.commands.registerCommand(DIAGNOSTICS_COMMAND, diagnostics.show);
  const unsubscribe = adapter.onSessionChange(diagnostics.refresh);
  owned = {
    broker,
    deactivate: adapter.deactivate,
    dispose: () => { unsubscribe(); command.dispose(); diagnostics.dispose(); },
  };
}

export async function deactivate(): Promise<void> {
  const active = owned;
  owned = undefined;
  if (!active) {
    return;
  }
  try {
    await active.broker.stop();
  } finally {
    active.deactivate();
    active.dispose();
  }
}

function loadVSCodeHost(): VSCodeHost | undefined {
  try {
    return createRequire(import.meta.url)("vscode") as VSCodeHost;
  } catch {
    return undefined;
  }
}

function extensionVersion(value: unknown): string | undefined {
  return typeof value === "string" && /^[A-Za-z0-9._-]{1,64}$/.test(value) ? value : undefined;
}
