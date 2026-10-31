import type { ReportScope } from "@/types/report";

export type ReportFileNameInput = {
  projectName: string;
  scope: ReportScope;
  sprintName: string | null;
  generatedAt: string;
  timeZone: string;
};

// Firma mínima: la lógica se implementa en la tarea T026.
export function reportFileName(input: ReportFileNameInput): string {
  void input;
  throw new Error("not implemented");
}
