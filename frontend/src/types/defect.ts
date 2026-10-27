// Tipos de las peticiones y respuestas de los defectos, según contracts/openapi.yaml (spec 007).
// Los errores sin código propio (ErrorResponse, ValidationErrorResponse) se reutilizan de
// @/types/auth.
import type { SprintStatus, UserRef } from "@/types/sprint";

// Severidad del defecto, de la más grave a la menos grave (RN-04).
export type DefectSeverity = "critical" | "high" | "medium" | "low";

// Estado del defecto en su ciclo de vida (RN-11).
export type DefectStatus = "open" | "in_progress" | "resolved" | "discarded";

// Cuerpo del alta de un defecto (DefectInput, FR-001 a FR-007).
export type DefectInput = {
  description: string;
  severity: DefectSeverity;
  storyId: string;
  detectedSprintId: string;
};

// Cuerpo de la modificación parcial de un defecto (DefectPatch, FR-033): cada campo es opcional y
// solo viajan los que cambian.
export type DefectPatch = {
  description?: string;
  severity?: DefectSeverity;
  storyId?: string;
  detectedSprintId?: string;
};

// Cuerpo del cambio de estado de un defecto (DefectStatusChange); resolvedSprintId solo viaja al
// resolver (FR-023).
export type DefectStatusChange = {
  status: DefectStatus;
  resolvedSprintId?: string;
};

// Identificación mínima de la historia del defecto (StoryRef).
export type DefectStoryRef = {
  id: string;
  title: string;
};

// Sprint de detección o de resolución del defecto (SprintRef de la spec 007).
export type DefectSprintRef = {
  id: string;
  name: string;
  status: SprintStatus;
  startDate: string;
};

// Defecto registrado (Defect); resolvedSprint llega en null si el defecto no está resuelto.
export type Defect = {
  id: string;
  projectId: string;
  description: string;
  severity: DefectSeverity;
  status: DefectStatus;
  story: DefectStoryRef;
  detectedSprint: DefectSprintRef;
  resolvedSprint: DefectSprintRef | null;
  createdBy: UserRef;
  updatedBy: UserRef;
  statusChangedAt: string;
  statusChangedBy: UserRef;
  createdAt: string;
  updatedAt: string;
};

// Motivo de una lista de defectos vacía (ProjectDefects, StoryDefects, FR-045).
export type DefectEmptyReason = "no_defects" | "no_matches";

// Motivo del rechazo 409 de una escritura de defecto (DefectConflictResponse).
export type DefectConflictCode =
  | "read_only"
  | "no_detection_sprint"
  | "invalid_transition"
  | "defect_discarded"
  | "links_frozen";

// Filtros del listado de defectos de un proyecto (FR-040 a FR-042): estado y severidad admiten
// varios valores; historia y sprint, uno o ninguno (cadena vacía).
export type DefectFilterValues = {
  status: DefectStatus[];
  severity: DefectSeverity[];
  storyId: string;
  sprintId: string;
};

// Lista de defectos de un proyecto; emptyReason es null si la lista tiene elementos (ProjectDefects).
export type ProjectDefects = {
  defects: Defect[];
  emptyReason: DefectEmptyReason | null;
};

// Lista de defectos de una historia; solo admite el motivo no_defects (StoryDefects).
export type StoryDefects = {
  defects: Defect[];
  emptyReason: "no_defects" | null;
};

// Cuerpo del 409 de una escritura de defecto (DefectConflictResponse).
export type DefectConflictResponse = {
  code: DefectConflictCode;
  error: string;
  currentStatus: DefectStatus | null;
  allowedStatuses: DefectStatus[];
};
