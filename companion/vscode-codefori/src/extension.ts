import { createRequire } from "node:module";

import { createBroker, createCodeForIBrokerHandler, type BrokerRequest, type BrokerResponse, type FixedLoopbackServer } from "./broker.js";
import { createCodeForIAdapter, type CodeForIExports } from "./codeforiAdapter.js";
import { createHTTPServer } from "./httpServer.js";

const CODE_FOR_I_EXTENSION_ID = "halcyontechltd.code-for-ibmi";
type ServerFactory = (handler: (request: BrokerRequest) => Promise<BrokerResponse>) => FixedLoopbackServer;

export interface ExtensionHost {
  getExtension<T>(identifier: string): { activate(): Promise<T | undefined>; exports: T | undefined } | undefined;
}

export interface ActivationOptions {
  extensionHost?: ExtensionHost;
  serverFactory?: ServerFactory;
}

let owned: { broker: ReturnType<typeof createBroker>; deactivate(): void } | undefined;

export async function activate(context: unknown, options: ActivationOptions = {}): Promise<void> {
  await deactivate();
  const extension = (options.extensionHost ?? loadExtensionHost())?.getExtension<CodeForIExports>(
    CODE_FOR_I_EXTENSION_ID,
  );
  let exports = extension?.exports;
  try {
    exports = (await extension?.activate()) ?? exports;
  } catch {
    exports = undefined;
  }

  const adapter = createCodeForIAdapter(exports, context);
  const broker = createBroker({
    serverFactory: options.serverFactory ?? createHTTPServer,
    handler: createCodeForIBrokerHandler(adapter),
  });
  await broker.start();
  owned = { broker, deactivate: adapter.deactivate };
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
  }
}

function loadExtensionHost(): ExtensionHost | undefined {
  try {
    return createRequire(import.meta.url)("vscode").extensions as ExtensionHost;
  } catch {
    return undefined;
  }
}
