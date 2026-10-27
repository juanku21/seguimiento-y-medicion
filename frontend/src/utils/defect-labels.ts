import type {
  DefectConflictCode,
  DefectEmptyReason,
  DefectSeverity,
  DefectStatus,
} from "@/types/defect";

// Etiquetas en español de la severidad; el backend envía los valores en inglés (RN-04,
// Principio VIII).
export const DEFECT_SEVERITY_LABELS: Record<DefectSeverity, string> = {
  critical: "",
  high: "",
  medium: "",
  low: "",
};

// Etiquetas en español del estado del defecto (RN-11).
export const DEFECT_STATUS_LABELS: Record<DefectStatus, string> = {
  open: "",
  in_progress: "",
  resolved: "",
  discarded: "",
};

// Mensajes en español de la lista de defectos vacía según su motivo (FR-045).
export const DEFECT_EMPTY_REASON_LABELS: Record<DefectEmptyReason, string> = {
  no_defects: "",
  no_matches: "",
};

// Títulos en español de los cinco motivos de rechazo 409 de una escritura de defecto.
export const DEFECT_CONFLICT_LABELS: Record<DefectConflictCode, string> = {
  read_only: "",
  no_detection_sprint: "",
  invalid_transition: "",
  defect_discarded: "",
  links_frozen: "",
};
