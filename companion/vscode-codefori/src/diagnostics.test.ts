import { describe, expect, it } from "vitest";

import { createDiagnosticsUI, diagnosticLines, type CompanionDiagnosticSnapshot } from "./diagnostics.js";

function snapshot(overrides: Partial<CompanionDiagnosticSnapshot> = {}): CompanionDiagnosticSnapshot {
  return {
    listener: "listening",
    codeForIExtension: "found",
    codeForIActivation: "active",
    codeForIVersion: "3.0.12",
    companionVersion: "0.2.2",
    adapter: { instance: "available", subscriptions: "registered", getConnection: "available", sqlCapability: "unknown", operationFailure: undefined },
    ...overrides,
  };
}

describe("Nexus Companion diagnostics", () => {
  it.each([
    ["connected", snapshot(), "$(plug) Nexus Companion: Connected"],
    ["no IBM i connection", snapshot({ adapter: { instance: "available", subscriptions: "registered", getConnection: "unavailable", sqlCapability: "unknown", operationFailure: undefined } }), "$(circle-slash) Nexus Companion: No IBM i connection"],
    ["Code for IBM i unavailable", snapshot({ codeForIExtension: "unavailable", adapter: { instance: "unavailable", subscriptions: "unavailable", getConnection: "unavailable", sqlCapability: "unknown", operationFailure: undefined } }), "$(warning) Nexus Companion: Code for IBM i unavailable"],
    ["Companion failure", snapshot({ listener: "unavailable" }), "$(error) Nexus Companion: Failure"],
  ])("reports %s in the always-visible status bar", (_name, current, text) => {
    const statusBar = { text: "", tooltip: "", command: "", show: () => undefined, dispose: () => undefined };
    const output = { clear: () => undefined, appendLine: () => undefined, show: () => undefined, dispose: () => undefined };

    createDiagnosticsUI(statusBar, output, () => current);

    expect(statusBar.text).toBe(text);
  });

  it("renders a bounded sanitized snapshot without sensitive details", () => {
    const lines = diagnosticLines(snapshot({ codeForIVersion: "host.example/QUSER/secret" }));
    const output: string[] = [];
    const ui = createDiagnosticsUI(
      { text: "", tooltip: "", command: "", show: () => undefined, dispose: () => undefined },
      { clear: () => { output.length = 0; }, appendLine: (line) => output.push(line), show: () => undefined, dispose: () => undefined },
      () => snapshot({ codeForIVersion: "host.example/QUSER/secret" }),
    );

    ui.show();

    expect(lines).toHaveLength(12);
    expect(output.join("\n")).not.toContain("host.example");
    expect(output.join("\n")).not.toContain("QUSER");
    expect(output.join("\n")).not.toContain("secret");
    expect(output).toContain("Code for IBM i version: unavailable");
  });

  it("renders the last operation failure without raw exception content", () => {
    const output = diagnosticLines(snapshot({
      adapter: {
        instance: "available",
        subscriptions: "registered",
        getConnection: "available",
        sqlCapability: "unavailable",
        operationFailure: { operation: "program.resolve", stage: "get_object_list" },
      },
    }));

    expect(output).toContain("Last operation failure: program.resolve");
    expect(output).toContain("Last operation failure stage: get_object_list");
    expect(output).toContain("SQL capability: unavailable");
    expect(output.join("\n")).not.toContain("host.example");
    expect(output.join("\n")).not.toContain("QUSER");
    expect(output.join("\n")).not.toContain("secret");
  });
});
