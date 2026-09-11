import { createServer, type IncomingMessage, type ServerResponse } from "node:http";

import type { BrokerRequest, BrokerResponse, FixedLoopbackServer } from "./broker.js";
import type { RequestAuthenticator } from "./tokenState.js";
import { MAX_REQUEST_BYTES, unavailableResponse } from "./protocol.js";

const browserOriginRejected = new TextEncoder().encode('{"state":"browser_origin_rejected"}');
const unauthorized = new TextEncoder().encode('{"state":"unauthorized"}');

export function createHTTPServer(handler: (request: BrokerRequest) => Promise<BrokerResponse>, authenticate: RequestAuthenticator): FixedLoopbackServer {
  const server = createServer((request, response) => {
    void handleRequest(request, response, handler, authenticate);
  });
  let listening = false;

  return {
    async listen(host: string, port: number): Promise<string> {
      return await new Promise<string>((resolve, reject) => {
        const onError = (error: Error) => {
          server.off("listening", onListening);
          reject(error);
        };
        const onListening = () => {
          server.off("error", onError);
          listening = true;
          const address = server.address();
          if (!address || typeof address === "string" || address.address !== host || address.port < 1 || address.port > 65535) {
            reject(new Error("loopback listener unavailable"));
            return;
          }
          resolve(`${host}:${address.port}`);
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
  authenticate: RequestAuthenticator,
): Promise<void> {
  if (hasOriginHeader(request.headers)) {
    request.resume();
    write(response, 403, browserOriginRejected);
    return;
  }

  if (!authenticate(request.headers)) {
    writeAndClose(response, 401, unauthorized);
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
    headers: request.headers,
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

function writeAndClose(response: ServerResponse, status: number, body: Uint8Array): void {
  response.writeHead(status, { "content-type": "application/json", connection: "close" });
  response.end(body);
}
