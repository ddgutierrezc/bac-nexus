import { describe, expect, it } from "vitest";

import { createProgramInspection } from "./programInspection.js";

describe("program inspection", () => {
  it("searches configured current library before de-duplicated library list", async () => {
    const calls: string[] = [];
    const inspection = createProgramInspection({
      getConfig: () => ({ currentLibrary: "curlib", libraryList: ["LIBA", "CURLIB", "libb"] }),
      getObjectList: async ({ library }) => {
        calls.push(library);
        return library === "LIBB" ? [{ library, name: "PISA061", type: "*PGM", text: "" }] : [];
      },
    });

    const result = await inspection.resolveProgram({ name: "pisa061" });

    expect(calls).toEqual(["CURLIB", "LIBA", "LIBB"]);
    expect(result).toMatchObject({
      state: "resolved",
      searchStrategy: "code_for_i_configured_context",
      librariesSearched: ["CURLIB", "LIBA", "LIBB"],
      runtimeLiblVerified: false,
      matches: [{ library: "LIBB", provenance: "code_for_i_configured_library_list", matchPosition: 3 }],
    });
  });

  it("does not use source member-name matching as compile provenance", async () => {
    const inspection = createProgramInspection({
      getConfig: () => ({ currentLibrary: "CURLIB", libraryList: [] }),
      getObjectList: async () => [],
    });

    await expect(inspection.findProgramSource({ library: "PRODLIB", name: "PISA061", objectType: "*PGM" })).resolves.toEqual({
      state: "unavailable",
      reason: "compiled_object_source_metadata_unsupported",
      nextStep: "configure_documented_compile_provenance_api",
      certainty: "unavailable",
      completeness: "complete",
      runtimeLiblVerified: false,
    });
  });

  it("reports invalid configured entries and omitted configured scope truthfully", async () => {
    const invalid = createProgramInspection({ getConfig: () => ({ currentLibrary: "", libraryList: ["bad name"] }), getObjectList: async () => [] });
    await expect(invalid.resolveProgram({ name: "PISA061" })).resolves.toMatchObject({ state: "unavailable", reason: "invalid_configured_library" });

    const libraries = Array.from({ length: 17 }, (_, index) => `LIB${index}`);
    const truncated = createProgramInspection({ getConfig: () => ({ currentLibrary: "", libraryList: libraries }), getObjectList: async () => [] });
    await expect(truncated.resolveProgram({ name: "PISA061" })).resolves.toMatchObject({ state: "truncated", truncated: true, completeness: "truncated", librariesSearched: libraries.slice(0, 16) });

    const malformed = createProgramInspection({ getConfig: () => ({ currentLibrary: "LIBA", libraryList: "LIBB" }), getObjectList: async () => [] });
    await expect(malformed.resolveProgram({ name: "PISA061" })).resolves.toMatchObject({ state: "unavailable", reason: "invalid_configured_library" });
  });
});
