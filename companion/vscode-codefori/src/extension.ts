export function activate(): { state: "companion_unavailable" } {
  return { state: "companion_unavailable" };
}

export function deactivate(): void {
  // Descriptor-backed activation is intentionally deferred to Slice 6.
}
