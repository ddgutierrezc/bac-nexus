import type { CatalogCandidate, CatalogResolveResult, QueryState, SessionState } from "./protocol.js";
import { canonicalizeProofQuery } from "./query.js";
import { createProgramInspection, ProgramResolveFailure, type FindProgramSourceRequest, type FindProgramSourceResult, type ProgramInspection, type ResolveProgramRequest, type ResolveProgramResult } from "./programInspection.js";
import { acquireCatalogSource, type MemberContentProvider, type SourceAcquisitionResult } from "./sourceAcquisition.js";

const MAX_VALUE_BYTES = 256;
const encoder = new TextEncoder();
const decoder = new TextDecoder("utf-8", { fatal: true });
const CATALOG_ROW_LIMIT = 51;
const SYSTEM_NAME = /^[A-Z$#@][A-Z0-9_$#@]{0,9}$/;
const CATALOG_COLUMNS = ["ITEM", "TIPO_DE_FUENTE", "TIPO_OBJETO", "APLICACION", "VERSION", "BIBLIOTECA_PRODUCCION", "BIBLIOTECA_FUENTES", "ARCHIVO_FUENTES", "DESCRIPCION"] as const;
const catalogSQL = (filtered: boolean): string => `SELECT SHSNAM AS Item, SHSTYP AS Tipo_de_Fuente, SHOTYP AS Tipo_Objeto, PDAPPL AS Aplicacion, PDVERS AS Version, PDNAME AS Biblioteca_Produccion, PDSLIB AS Biblioteca_Fuentes, PDSFIL AS Archivo_Fuentes, PDDESC AS Descripcion FROM pde400.SAHDR, pde400.SIHDR, pde400.prdmst WHERE SHIIDÑ = SIIIDÑ AND SHENDR = 99999 AND SIDCOD = '' AND SHAVPÑ = PDAVPÑ AND UPPER(SHSNAM) LIKE UPPER(?) AND ${filtered ? "UPPER(PDNAME) = UPPER(?)" : "UPPER(PDNAME) = UPPER(PDNAME)"} ORDER BY SHSNAM, PDSLIB, PDSFIL, SHOTYP, SHSTYP, PDNAME, PDAPPL, PDVERS FETCH FIRST 51 ROWS ONLY`;

export interface CodeForIConnection {
  readonly enableSQL?: boolean;
  runSQL(sql: string | string[], options: { bindings?: unknown[]; rows?: number }): Promise<unknown[]>;
  getConfig?(): unknown;
  getContent?(): {
    getObjectList(filters: { library: string; object: string; types: string[] }): Promise<Array<{ library: string; name: string; type: string; text: string }>>;
    downloadMemberContent?(library: string, file: string, member: string, localPath?: string): Promise<string | undefined>;
  };
}

export interface CodeForIInstance {
  getConnection(): CodeForIConnection;
  subscribe(context: unknown, event: "connected" | "disconnected", name: string, callback: () => void): void;
}

export interface CodeForIExports {
  instance: CodeForIInstance;
}

export type QueryResult =
  | { state: "ok"; rows: [{ value: string }] }
  | { state: Exclude<QueryState, "ok"> };
export type SessionStatusResult = { state: SessionState };

export interface AdapterDiagnosticSnapshot {
  readonly instance: "available" | "unavailable";
  readonly subscriptions: "registered" | "unavailable";
  readonly getConnection: "available" | "unavailable" | "threw";
  readonly sqlCapability: "available" | "unavailable" | "unknown";
  readonly operationFailure: OperationFailureDiagnostic | undefined;
}

export interface OperationFailureDiagnostic {
  readonly operation: "program.resolve";
  readonly stage:
    | "preflight_connection_unavailable"
    | "preflight_connection_threw"
    | "preflight_inspection_api_unavailable"
    | "preflight_accessor_failed"
    | "get_object_list"
    | "resolve"
    | "handler";
}

export interface CodeForIAdapter {
  sessionStatus(): SessionStatusResult;
  query(sql: string): Promise<QueryResult>;
  resolveCatalogCandidates(request: { item: string; productionLibrary?: string }): Promise<CatalogResolveResult>;
  acquireCatalogSource(candidate: CatalogCandidate): Promise<SourceAcquisitionResult>;
  resolveProgram(request: ResolveProgramRequest): Promise<ResolveProgramResult>;
  findProgramSource(request: FindProgramSourceRequest): Promise<FindProgramSourceResult>;
  diagnostics(): AdapterDiagnosticSnapshot;
  recordOperationFailure(operation: "program.resolve", stage: OperationFailureDiagnostic["stage"]): void;
  onSessionChange(callback: () => void): () => void;
  deactivate(): void;
}

export function createCodeForIAdapter(
  codeForI: CodeForIExports | undefined,
  context: unknown,
): CodeForIAdapter {
  let instance = codeForI?.instance;
  let active = true;
  let connectionAvailable = instance !== undefined;
  let connectionGeneration = 0;
  let subscriptionsRegistered = false;
  let getConnection: AdapterDiagnosticSnapshot["getConnection"] = "unavailable";
  let sqlCapability: AdapterDiagnosticSnapshot["sqlCapability"] = "unknown";
  let operationFailure: OperationFailureDiagnostic | undefined;
  const listeners = new Set<() => void>();

  const refreshConnection = (): boolean => {
    if (!instance) {
      getConnection = "unavailable";
      sqlCapability = "unknown";
      return false;
    }
    try {
      const connection = instance.getConnection();
      getConnection = connection === undefined ? "unavailable" : "available";
      sqlCapability = connection === undefined ? "unknown" : classifySQLCapability(connection);
      return connection !== undefined;
    } catch {
      getConnection = "threw";
      sqlCapability = "unknown";
      return false;
    }
  };
  const notify = (): void => { for (const listener of listeners) listener(); };

  if (instance) {
    try {
      instance.subscribe(context, "connected", "BAC Nexus Companion connected", () => {
        connectionAvailable = refreshConnection();
        connectionGeneration += 1;
        operationFailure = undefined;
        notify();
      });
      instance.subscribe(context, "disconnected", "BAC Nexus Companion disconnected", () => {
        connectionAvailable = false;
        getConnection = "unavailable";
        sqlCapability = "unknown";
        connectionGeneration += 1;
        operationFailure = undefined;
        notify();
      });
      subscriptionsRegistered = true;
      connectionAvailable = refreshConnection();
    } catch {
      connectionAvailable = false;
      subscriptionsRegistered = false;
      getConnection = "threw";
      sqlCapability = "unknown";
    }
  }

  return {
    sessionStatus(): SessionStatusResult {
      if (!active || !instance) {
        return { state: "codefori_extension_unavailable" };
      }
      connectionAvailable = refreshConnection();
      return connectionAvailable
        ? { state: "connected" }
        : { state: "connection_unavailable" };
    },
    async query(sql: string): Promise<QueryResult> {
      const canonicalSQL = canonicalizeProofQuery(sql);
      if (!canonicalSQL) {
        return { state: "invalid_query" };
      }
      if (!active || !instance) {
        return { state: "unavailable" };
      }
      connectionAvailable = refreshConnection();
      if (!connectionAvailable) {
        return { state: "unavailable" };
      }

      const startedGeneration = connectionGeneration;
      let connection: CodeForIConnection;
      try {
        connection = instance.getConnection();
      } catch {
        connectionAvailable = false;
        return { state: "unavailable" };
      }

      try {
        const rows = await connection.runSQL(canonicalSQL, { rows: 1 });
        if (!active || !connectionAvailable || connectionGeneration !== startedGeneration) {
          return { state: "unavailable" };
        }
        return normalizeRows(rows);
      } catch {
        return { state: "failed" };
      }
    },
    async resolveCatalogCandidates(request): Promise<CatalogResolveResult> {
      const item = normalizeSystemName(request.item);
      const productionLibrary = request.productionLibrary === undefined ? undefined : normalizeSystemName(request.productionLibrary);
      if (!item || (request.productionLibrary !== undefined && !productionLibrary)) return { state: "invalid_request" };
      if (!active || !instance || !(connectionAvailable = refreshConnection())) return { state: "unavailable" };
      const startedGeneration = connectionGeneration;
      let connection: CodeForIConnection;
      try { connection = instance.getConnection(); } catch { connectionAvailable = false; return { state: "unavailable" }; }
      try {
        const rows = await connection.runSQL(catalogSQL(productionLibrary !== undefined), { bindings: productionLibrary === undefined ? [`%${item}%`] : [`%${item}%`, productionLibrary], rows: CATALOG_ROW_LIMIT });
        if (!validGeneration(startedGeneration)) return { state: "unavailable" };
        return normalizeCatalogRows(rows);
      } catch { return { state: "failed" }; }
    },
    async acquireCatalogSource(candidate: CatalogCandidate): Promise<SourceAcquisitionResult> {
      if (!active || !instance || !(connectionAvailable = refreshConnection())) return { state: "unavailable" };
      const startedGeneration = connectionGeneration;
      let provider: MemberContentProvider | undefined;
      try {
        const content = instance.getConnection().getContent?.();
        if (content?.downloadMemberContent) {
          const downloadMemberContent = content.downloadMemberContent;
          provider = { downloadMemberContent: (library, file, member, localPath) => downloadMemberContent.call(content, library, file, member, localPath) };
        }
      } catch {
        connectionAvailable = false;
        return { state: "unavailable" };
      }
      return acquireCatalogSource(candidate, provider, () => validGeneration(startedGeneration));
    },
    async resolveProgram(request: ResolveProgramRequest): Promise<ResolveProgramResult> {
      const bound = getProgramInspection();
      if (!bound) return unavailableProgramResult(request.library);
      try {
        const result = await bound.inspection.resolveProgram(request);
        operationFailure = undefined;
        return validGeneration(bound.generation) ? result : unavailableProgramResult(request.library, "stale_session");
      } catch (error) {
        operationFailure = {
          operation: "program.resolve",
          stage: error instanceof ProgramResolveFailure ? error.stage : "resolve",
        };
        return unavailableProgramResult(request.library);
      }
    },
    async findProgramSource(request: FindProgramSourceRequest): Promise<FindProgramSourceResult> {
      const bound = getProgramInspection();
      if (!bound) return unavailableSourceResult();
      try {
        const result = await bound.inspection.findProgramSource(request);
        return validGeneration(bound.generation) ? result : unavailableSourceResult("stale_session");
      } catch { return unavailableSourceResult(); }
    },
    diagnostics(): AdapterDiagnosticSnapshot {
      if (active && instance) {
        connectionAvailable = refreshConnection();
      }
      return Object.freeze({
        instance: active && instance ? "available" : "unavailable",
        subscriptions: active && subscriptionsRegistered ? "registered" : "unavailable",
        getConnection: active ? getConnection : "unavailable",
        sqlCapability: active ? sqlCapability : "unknown",
        operationFailure: active ? operationFailure : undefined,
      });
    },
    recordOperationFailure(operation, stage): void {
      operationFailure = { operation, stage };
    },
    onSessionChange(callback: () => void): () => void {
      listeners.add(callback);
      return () => listeners.delete(callback);
    },
    deactivate(): void {
      active = false;
      connectionAvailable = false;
      instance = undefined;
      operationFailure = undefined;
      listeners.clear();
    },
  };

  function getProgramInspection(): { inspection: ProgramInspection; generation: number } | undefined {
    if (!active || !instance) {
      recordPreflightFailure("preflight_connection_unavailable");
      return undefined;
    }

    let connection: CodeForIConnection | undefined;
    try {
      connection = instance.getConnection();
    } catch {
      connectionAvailable = false;
      getConnection = "threw";
      sqlCapability = "unknown";
      recordPreflightFailure("preflight_connection_threw");
      return undefined;
    }
    if (!connection) {
      connectionAvailable = false;
      getConnection = "unavailable";
      sqlCapability = "unknown";
      recordPreflightFailure("preflight_connection_unavailable");
      return undefined;
    }

    connectionAvailable = true;
    getConnection = "available";
    sqlCapability = classifySQLCapability(connection);

    let getConfig: CodeForIConnection["getConfig"];
    let getContent: CodeForIConnection["getContent"];
    try {
      getConfig = connection.getConfig;
      getContent = connection.getContent;
    } catch {
      recordPreflightFailure("preflight_accessor_failed");
      return undefined;
    }
    if (!getConfig || !getContent) {
      recordPreflightFailure("preflight_inspection_api_unavailable");
      return undefined;
    }

    return {
      generation: connectionGeneration,
      inspection: createProgramInspection({
        getConfig: () => getConfig.call(connection),
        getObjectList: (filters) => getContent.call(connection).getObjectList(filters),
      }),
    };
  }

  function recordPreflightFailure(stage: Extract<OperationFailureDiagnostic["stage"], `preflight_${string}`>): void {
    operationFailure = { operation: "program.resolve", stage };
  }

  function validGeneration(generation: number): boolean { return active && connectionAvailable && connectionGeneration === generation; }
}

function unavailableProgramResult(library?: string, reason = "codefori_extension_unavailable"): ResolveProgramResult {
  return { state: "unavailable", searchStrategy: library ? "explicit_library" : "code_for_i_configured_context", librariesSearched: [], matches: [], completeness: "complete", truncated: false, runtimeLiblVerified: false, reason };
}

function unavailableSourceResult(reason: FindProgramSourceResult["reason"] = "compiled_object_source_metadata_unsupported"): FindProgramSourceResult {
  return { state: "unavailable", reason, nextStep: "configure_documented_compile_provenance_api", certainty: "unavailable", completeness: "complete", runtimeLiblVerified: false };
}

function normalizeRows(rows: unknown): QueryResult {
  if (!Array.isArray(rows) || rows.length !== 1) {
    return { state: "failed" };
  }

  const row = rows[0];
  if (!isRecord(row)) {
    return { state: "failed" };
  }
  const keys = Reflect.ownKeys(row).filter((key) =>
    Object.prototype.propertyIsEnumerable.call(row, key),
  );
  if (keys.length !== 1 || typeof keys[0] !== "string") {
    return { state: "failed" };
  }

  const value = row[keys[0]];
  return isBoundedUTF8String(value)
    ? { state: "ok", rows: [{ value }] }
    : { state: "failed" };
}

function normalizeCatalogRows(rows: unknown): CatalogResolveResult {
  if (!Array.isArray(rows)) return { state: "failed" };
  if (rows.length > 50) return { state: "candidate_limit_exceeded" };
  const candidates: CatalogCandidate[] = [];
  for (const row of rows) {
    if (!isRecord(row)) return { state: "failed" };
    const values: Record<string, string> = {};
    for (const key of CATALOG_COLUMNS) {
      const value = row[key];
      if (value === null && !["ITEM", "TIPO_DE_FUENTE", "TIPO_OBJETO", "BIBLIOTECA_FUENTES", "ARCHIVO_FUENTES"].includes(key)) values[key] = "";
      else if (typeof value === "string" && isBoundedUTF8String(value)) values[key] = value.trim();
      else return { state: "failed" };
    }
    if (Reflect.ownKeys(row).filter((key) => Object.prototype.propertyIsEnumerable.call(row, key)).length !== CATALOG_COLUMNS.length || CATALOG_COLUMNS.some((key) => !(key in row))) return { state: "failed" };
    const candidate = { item: values.ITEM!, sourceType: values.TIPO_DE_FUENTE!, objectType: values.TIPO_OBJETO!, application: values.APLICACION!, version: values.VERSION!, productionLibrary: values.BIBLIOTECA_PRODUCCION!, sourceLibrary: values.BIBLIOTECA_FUENTES!, sourceFileBase: values.ARCHIVO_FUENTES!, description: values.DESCRIPCION! };
    if (!candidate.item || !candidate.sourceType || !candidate.objectType || !candidate.sourceLibrary || !candidate.sourceFileBase) return { state: "failed" };
    candidates.push(candidate);
  }
  return { state: "ok", candidates };
}

function normalizeSystemName(value: string): string | undefined {
  const normalized = value.trim().toUpperCase();
  return SYSTEM_NAME.test(normalized) ? normalized : undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function classifySQLCapability(connection: CodeForIConnection): AdapterDiagnosticSnapshot["sqlCapability"] {
  try {
    if (connection.enableSQL === true) return "available";
    if (connection.enableSQL === false) return "unavailable";
  } catch {
    // Code for IBM i versions may not expose this getter safely.
  }
  return "unknown";
}

function isBoundedUTF8String(value: unknown): value is string {
  if (typeof value !== "string" || encoder.encode(value).byteLength > MAX_VALUE_BYTES) {
    return false;
  }
  try {
    return decoder.decode(encoder.encode(value)) === value;
  } catch {
    return false;
  }
}
