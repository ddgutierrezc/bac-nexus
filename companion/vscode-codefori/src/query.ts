export const CANONICAL_PROOF_QUERY = "SELECT CURRENT_USER FROM SYSIBM.SYSDUMMY1";

const MAX_INPUT_BYTES = 128;
const permittedWhitespace = /^[ \t\r\n]+$/;
const proofTokens = ["SELECT", "CURRENT_USER", "FROM", "SYSIBM.SYSDUMMY1"];
const encoder = new TextEncoder();

export function canonicalizeProofQuery(input: string): string | null {
  if (encoder.encode(input).byteLength > MAX_INPUT_BYTES || !isASCII(input)) {
    return null;
  }

  const tokens = input.trim().split(/[ \t\r\n]+/);
  if (tokens.length !== proofTokens.length || !hasOnlyPermittedWhitespace(input)) {
    return null;
  }

  return tokens.every((token, index) => token.toUpperCase() === proofTokens[index])
    ? CANONICAL_PROOF_QUERY
    : null;
}

function isASCII(value: string): boolean {
  return [...value].every((character) => character.codePointAt(0)! <= 0x7f);
}

function hasOnlyPermittedWhitespace(value: string): boolean {
  const pieces = value.split(/[^ \t\r\n]+/);
  return pieces.every((piece) => piece === "" || permittedWhitespace.test(piece));
}
