// Tipos de las peticiones y respuestas del registro de esfuerzo, según contracts/openapi.yaml
// (spec 006). Las horas llegan como número con 2 decimales y "no calculable" como null. Los errores
// sin código propio (ErrorResponse, ValidationErrorResponse) se reutilizan de @/types/auth.
import type { SprintStatus, UserRef } from "@/types/sprint";

export type { UserRef };

// Cuerpo del alta de un registro (EffortEntryInput, FR-002 a FR-006); la fecha va en AAAA-MM-DD.
export type EffortEntryInput = {
  entryDate: string;
  activity: string;
  hours: number;
};

// Identificación mínima de la historia del registro (StoryRef).
export type StoryRef = {
  id: string;
  title: string;
};

// Sprint asociado al registro al crearlo, con su estado (SprintRef de la spec 006, FR-015).
export type SprintRef = {
  id: string;
  name: string;
  status: SprintStatus;
};

// Registro de esfuerzo (EffortEntry); sprint llega en null si no tiene sprint y definitive indica
// que el sprint está cerrado con el plazo de gracia vencido (FR-020).
export type EffortEntry = {
  id: string;
  entryDate: string;
  activity: string;
  hours: number;
  author: UserRef;
  story: StoryRef;
  sprint: SprintRef | null;
  definitive: boolean;
  createdAt: string;
  updatedAt: string;
};

// Motivo del rechazo 409 de una escritura de esfuerzo.
export type EffortConflictCode =
  "read_only" | "effort_finalized" | "daily_cap_exceeded";

// Rechazo 409 de una escritura de esfuerzo (EffortConflictResponse); availableHours trae las horas
// que quedan para la fecha solo con daily_cap_exceeded y llega en null en los demás casos.
export type EffortConflictResponse = {
  code: EffortConflictCode;
  error: string;
  availableHours: number | null;
};
