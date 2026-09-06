import { createServer, type IncomingMessage, type ServerResponse } from "node:http";

import type { BrokerRequest, BrokerResponse, FixedLoopbackServer } from "./broker.js";
import { MAX_REQUEST_BYTES, unavailableResponse } from "./protocol.js";

const browserOriginRejected = new TextEncoder().encode('{"state":"browser_origin_rejected"}');

export function createHTTPServer(handler: (request: BrokerRequest) => Promise<BrokerResponse>): FixedLoopbackServer {
  const server = createServer((request, response) => {
    void handleRequest(request, response, handler);
  });
  let listening = false;

  return {
    async listen(host: string, port: number): Promise<void> {
      await new Promise<void>((resolve, reject) => {
        const onError = (error: Error) => {
          server.off("listening", onListening);
          reject(error);
        };
        const onListening = () => {
          server.off("error", onError);
          listening = true;
          resolve();
        };
        server.once("error", onError);
        server.once("listening", onListening);
        server.listen(port, host);
      });
    },
    async close(): Promise<void> {
      if (!listening) {
        return;
      }
      listening = false;
      await new Promise<void>((resolve, reject) => {
        server.close((error) => (error ? reject(error) : resolve()));
      });
    },
  };
}

export function hasOriginHeader(headers: Readonly<Record<string, unknown>>): boolean {
  return Object.keys(headers).some((name) => name.toLowerCase() === "origin");
}

async function handleRequest(
  request: IncomingMessage,
  response: ServerResponse,
  handler: (request: BrokerRequest) => Promise<BrokerResponse>,
): Promise<void> {
  if (hasOriginHeader(request.headers)) {
    request.resume();
    write(response, 403, browserOriginRejected);
    return;
  }

  const body = await readBoundedBody(request);
  if (!body) {
    write(response, 400, unavailableResponse());
    return;
  }

  const result = await handler({
    method: request.method ?? "",
    path: request.url ?? "",
    headers: {},
    body,
  });
  write(response, result.status, result.body, result.headers);
}

function readBoundedBody(request: IncomingMessage): Promise<Uint8Array | undefined> {
  return new Promise((resolve) => {
    const chunks: Buffer[] = [];
    let size = 0;
    let settled = false;
    const finish = (body: Uint8Array | undefined) => {
      if (!settled) {
        settled = true;
        resolve(body);
      }
    };

    request.on("data", (chunk: Buffer) => {
      size += chunk.byteLength;
      if (size > MAX_REQUEST_BYTES) {
        finish(undefined);
        request.resume();
        return;
      }
      chunks.push(chunk);
    });
    request.once("end", () => finish(Buffer.concat(chunks)));
    request.once("error", () => finish(undefined));
  });
}

function write(
  response: ServerResponse,
  status: number,
  body: Uint8Array,
  headers: Readonly<Record<string, string>> = { "content-type": "application/json" },
): void {
  response.writeHead(status, headers);
  response.end(body);
}
