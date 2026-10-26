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

// Cuerpo de la corrección de un registro (EffortEntryPatch, 006/US3): solo los campos que cambian,
// al menos uno. Autor, historia y sprint no forman parte de la corrección (FR-019).
export type EffortEntryPatch = Partial<EffortEntryInput>;

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

// Horas de una persona sobre la historia (MemberEffort, FR-032): una línea por persona con
// registros, incluidas las que ya no son integrantes.
export type MemberEffort = {
  user: UserRef;
  hours: number;
};

// Esfuerzo de una historia (StoryEffort, 006/US2): total, desglose por persona y registros en fecha
// descendente (FR-035). Sin registros, las listas llegan vacías y el total en 0.
export type StoryEffort = {
  storyId: string;
  totalHours: number;
  byMember: MemberEffort[];
  entries: EffortEntry[];
};

// Filtros del esfuerzo del proyecto (006/US4), como los elige la interfaz: cada uno vale "" cuando
// no se filtra por él. sprintId admite "none" para los registros sin sprint (FR-039) y las fechas
// van en AAAA-MM-DD, inclusivas (FR-038).
export type ProjectEffortFilter = {
  userId: string;
  storyId: string;
  sprintId: string;
  from: string;
  to: string;
};

// Motivo de la lista vacía del proyecto: sin ningún registro (US4-8) o con registros que no cumplen
// los filtros (US4-7).
export type ProjectEffortEmptyReason = "no_effort" | "no_matches";

// Esfuerzo del proyecto filtrado (ProjectEffort, 006/US4): registros en fecha descendente (FR-045),
// total del conjunto y motivo del vacío, que llega en null si hay registros.
export type ProjectEffort = {
  totalHours: number;
  entries: EffortEntry[];
  emptyReason: ProjectEffortEmptyReason | null;
};
