export const MAX_CONFIGURED_LIBRARIES = 16;
export const MAX_PROGRAM_MATCHES = 8;

export type ProgramInspectionState = "resolved" | "ambiguous" | "not_found" | "truncated" | "unavailable";
export type ProgramProvenance =
  | "explicit_library"
  | "code_for_i_current_library"
  | "code_for_i_configured_library_list";

export interface ProgramObject {
  library: string;
  name: string;
  type: string;
  text: string;
}

export interface ProgramInspectionConnection {
  getConfig(): unknown;
  getObjectList(filters: { library: string; object: string; types: string[] }): Promise<ProgramObject[]>;
}

export interface ResolveProgramRequest {
  name: string;
  library?: string;
}

export interface ProgramMatch {
  library: string;
  name: string;
  objectType: "*PGM";
  provenance: ProgramProvenance;
  matchPosition: number;
}

export interface ResolveProgramResult {
  state: ProgramInspectionState;
  searchStrategy: "explicit_library" | "code_for_i_configured_context";
  librariesSearched: string[];
  matches: ProgramMatch[];
  completeness: "complete" | "truncated";
  truncated: boolean;
  runtimeLiblVerified: false;
  reason?: string;
}

export interface FindProgramSourceRequest {
  library: string;
  name: string;
  objectType: "*PGM";
}

export interface FindProgramSourceResult {
  state: "unavailable";
  reason: "compiled_object_source_metadata_unsupported" | "stale_session";
  nextStep: "configure_documented_compile_provenance_api";
  certainty: "unavailable";
  completeness: "complete";
  runtimeLiblVerified: false;
}

export interface ProgramInspection {
  resolveProgram(request: ResolveProgramRequest): Promise<ResolveProgramResult>;
  findProgramSource(request: FindProgramSourceRequest): Promise<FindProgramSourceResult>;
}

export type ProgramResolveFailureStage = "get_object_list" | "resolve";

export class ProgramResolveFailure extends Error {
  constructor(readonly stage: ProgramResolveFailureStage) {
    super();
  }
}

export function createProgramInspection(connection: ProgramInspectionConnection): ProgramInspection {
  return {
    async resolveProgram(request) {
      const name = normalizeIdentifier(request.name);
      const explicitLibrary = request.library === undefined ? undefined : normalizeIdentifier(request.library);
      const configured = explicitLibrary ? { libraries: [explicitLibrary], truncated: false } : configuredLibraries(connection.getConfig());
      if (!configured) return unavailableResult("invalid_configured_library", explicitLibrary);
      const libraries = configured.libraries;
      const matches: ProgramMatch[] = [];

      for (let index = 0; index < libraries.length; index += 1) {
        const library = libraries[index]!;
        let objects: ProgramObject[];
        try {
          objects = await connection.getObjectList({ library, object: name, types: ["*PGM"] });
        } catch {
          throw new ProgramResolveFailure("get_object_list");
        }
        for (const object of objects) {
          if (object.type !== "*PGM" || normalizeIdentifier(object.name) !== name) continue;
          if (matches.length === MAX_PROGRAM_MATCHES) {
            return result("truncated", explicitLibrary, libraries.slice(0, index + 1), matches, true);
          }
          matches.push({
            library,
            name,
            objectType: "*PGM",
            provenance: explicitLibrary
              ? "explicit_library"
              : index === 0
                ? "code_for_i_current_library"
                : "code_for_i_configured_library_list",
            matchPosition: index + 1,
          });
        }
      }
      return result(configured.truncated ? "truncated" : matches.length === 0 ? "not_found" : matches.length === 1 ? "resolved" : "ambiguous", explicitLibrary, libraries, matches, configured.truncated);
    },
    async findProgramSource() {
      return {
        state: "unavailable",
        reason: "compiled_object_source_metadata_unsupported",
        nextStep: "configure_documented_compile_provenance_api",
        certainty: "unavailable",
        completeness: "complete",
        runtimeLiblVerified: false,
      };
    },
  };
}

function result(
  state: ProgramInspectionState,
  explicitLibrary: string | undefined,
  librariesSearched: string[],
  matches: ProgramMatch[],
  truncated: boolean,
): ResolveProgramResult {
  return {
    state,
    searchStrategy: explicitLibrary ? "explicit_library" : "code_for_i_configured_context",
    librariesSearched,
    matches,
    completeness: truncated ? "truncated" : "complete",
    truncated,
    runtimeLiblVerified: false,
  };
}

function configuredLibraries(config: unknown): { libraries: string[]; truncated: boolean } | undefined {
  if (!isRecord(config) || typeof config.currentLibrary !== "string" || !Array.isArray(config.libraryList) || !config.libraryList.every((value) => typeof value === "string")) return undefined;
  const ordered: string[] = [];
  for (const value of [config.currentLibrary, ...config.libraryList]) {
    if (value === "") continue;
    try { ordered.push(normalizeIdentifier(value)); } catch { return undefined; }
  }
  const deduplicated = [...new Set(ordered)];
  return { libraries: deduplicated.slice(0, MAX_CONFIGURED_LIBRARIES), truncated: deduplicated.length > MAX_CONFIGURED_LIBRARIES };
}

function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === "object" && value !== null && !Array.isArray(value); }

function unavailableResult(reason: string, explicitLibrary?: string): ResolveProgramResult {
  return { state: "unavailable", searchStrategy: explicitLibrary ? "explicit_library" : "code_for_i_configured_context", librariesSearched: [], matches: [], completeness: "complete", truncated: false, runtimeLiblVerified: false, reason };
}

function normalizeIdentifier(value: string): string {
  const normalized = value.toUpperCase();
  if (!/^[A-Z@$#][A-Z0-9@$#]{0,9}$/.test(normalized)) {
    throw new Error("invalid IBM i system identifier");
  }
  return normalized;
}
