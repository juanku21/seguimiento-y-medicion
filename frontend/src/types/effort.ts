// Tipos de las respuestas del registro de esfuerzo, según contracts/openapi.yaml (spec 006).
// Firma mínima de T018: la implementación corresponde a T032.
import type { SprintStatus, UserRef } from "@/types/sprint";

// Registro de esfuerzo devuelto por el alta (EffortEntry); sprint llega en null si no tiene sprint.
export type EffortEntry = {
  id: string;
  entryDate: string;
  activity: string;
  hours: number;
  author: UserRef;
  story: { id: string; title: string };
  sprint: { id: string; name: string; status: SprintStatus } | null;
  definitive: boolean;
  createdAt: string;
  updatedAt: string;
};
