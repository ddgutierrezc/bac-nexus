import { describe, expect, it } from "vitest";

import { CANONICAL_PROOF_QUERY, canonicalizeProofQuery } from "./query.js";

describe("canonicalizeProofQuery", () => {
  it("normalizes permitted ASCII case and whitespace", () => {
    expect(
      canonicalizeProofQuery("\tselect\r\ncurrent_user FROM sysibm.sysdummy1 "),
    ).toBe(CANONICAL_PROOF_QUERY);
  });

  it("rejects a statement with a semicolon", () => {
    expect(canonicalizeProofQuery("SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1;")).toBeNull();
  });

  it.each([
    "SELECT CURRENT_USER FROM SYSIBM .SYSDUMMY1",
    "SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1 -- comment",
    "SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1\u00a0",
    "SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1 extra",
    "SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1?",
    `${" ".repeat(89)}SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1`,
  ])("rejects broader input: %j", (input) => {
    expect(canonicalizeProofQuery(input)).toBeNull();
  });
});
