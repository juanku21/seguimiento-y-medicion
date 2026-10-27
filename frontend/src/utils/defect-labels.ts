import type {
  DefectConflictCode,
  DefectEmptyReason,
  DefectSeverity,
  DefectStatus,
} from "@/types/defect";

// Etiquetas en español de la severidad; el backend envía los valores en inglés (RN-04,
// Principio VIII).
export const DEFECT_SEVERITY_LABELS: Record<DefectSeverity, string> = {
  critical: "Crítica",
  high: "Alta",
  medium: "Media",
  low: "Baja",
};

// Etiquetas en español del estado del defecto (RN-11).
export const DEFECT_STATUS_LABELS: Record<DefectStatus, string> = {
  open: "Abierto",
  in_progress: "En progreso",
  resolved: "Resuelto",
  discarded: "Descartado",
};

// Mensajes en español de la lista de defectos vacía según su motivo (FR-045).
export const DEFECT_EMPTY_REASON_LABELS: Record<DefectEmptyReason, string> = {
  no_defects: "Todavía no hay defectos registrados.",
  no_matches: "Ningún defecto cumple los filtros elegidos.",
};

// Títulos en español de los cinco motivos de rechazo 409 de una escritura de defecto.
export const DEFECT_CONFLICT_LABELS: Record<DefectConflictCode, string> = {
  read_only: "Proyecto de solo lectura",
  no_detection_sprint: "Sin sprint de detección disponible",
  invalid_transition: "Transición no admitida",
  defect_discarded: "Defecto descartado",
  links_frozen: "Vínculos congelados",
};
