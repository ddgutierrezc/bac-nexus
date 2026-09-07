import type { AdapterDiagnosticSnapshot } from "./codeforiAdapter.js";

export const DIAGNOSTICS_COMMAND = "nexus-codefori-companion.showDiagnostics";

export interface CompanionDiagnosticSnapshot {
  readonly listener: "listening" | "unavailable";
  readonly codeForIExtension: "found" | "unavailable";
  readonly codeForIActivation: "active" | "unavailable";
  readonly codeForIVersion: string | undefined;
  readonly companionVersion: string;
  readonly adapter: AdapterDiagnosticSnapshot;
}

export interface StatusBarItem {
  text: string;
  tooltip: string;
  command: string;
  show(): void;
  dispose(): void;
}

export interface OutputChannel {
  clear(): void;
  appendLine(value: string): void;
  show(preserveFocus?: boolean): void;
  dispose(): void;
}

export interface DiagnosticsUI {
  statusBar: StatusBarItem;
  output: OutputChannel;
  refresh(): void;
  show(): void;
  dispose(): void;
}

export function createDiagnosticsUI(
  statusBar: StatusBarItem,
  output: OutputChannel,
  snapshot: () => CompanionDiagnosticSnapshot,
): DiagnosticsUI {
  const refresh = (): void => {
    const current = snapshot();
    const state = statusState(current);
    statusBar.text = state.text;
    statusBar.tooltip = `${state.tooltip} Select to open Nexus Companion diagnostics.`;
    statusBar.command = DIAGNOSTICS_COMMAND;
    statusBar.show();
  };
  const show = (): void => {
    refresh();
    output.clear();
    for (const line of diagnosticLines(snapshot())) {
      output.appendLine(line);
    }
    output.show(true);
  };

  refresh();
  return { statusBar, output, refresh, show, dispose: () => { statusBar.dispose(); output.dispose(); } };
}

export function diagnosticLines(snapshot: CompanionDiagnosticSnapshot): readonly string[] {
  return [
    "Nexus Companion Diagnostics",
    `Companion version: ${safeVersion(snapshot.companionVersion)}`,
    `Companion listener: ${snapshot.listener}`,
    `Code for IBM i extension: ${snapshot.codeForIExtension}`,
    `Code for IBM i activation: ${snapshot.codeForIActivation}`,
    `Code for IBM i version: ${safeVersion(snapshot.codeForIVersion)}`,
    `API instance: ${snapshot.adapter.instance}`,
    `Subscriptions: ${snapshot.adapter.subscriptions}`,
    `getConnection: ${snapshot.adapter.getConnection}`,
  ];
}

function statusState(snapshot: CompanionDiagnosticSnapshot): { text: string; tooltip: string } {
  if (snapshot.listener === "unavailable") {
    return { text: "$(error) Nexus Companion: Failure", tooltip: "The Nexus Companion listener is unavailable." };
  }
  if (snapshot.codeForIExtension === "unavailable" || snapshot.adapter.instance === "unavailable") {
    return { text: "$(warning) Nexus Companion: Code for IBM i unavailable", tooltip: "Code for IBM i is unavailable." };
  }
  if (snapshot.adapter.getConnection !== "available") {
    return { text: "$(circle-slash) Nexus Companion: No IBM i connection", tooltip: "Code for IBM i has no available IBM i connection." };
  }
  return { text: "$(plug) Nexus Companion: Connected", tooltip: "Code for IBM i connection is available." };
}

function safeVersion(value: string | undefined): string {
  return value !== undefined && /^[A-Za-z0-9._-]{1,64}$/.test(value) ? value : "unavailable";
}
