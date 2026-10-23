import type {
  CancelReason,
  OutcomeKind,
  RoundStatus,
  SessionStatus,
} from "@/types/poker";

// Etiquetas en español del estado de la sesión de estimación; el backend envía los valores en
// inglés (data-model.md sección 5, Principio VIII).
export const SESSION_STATUS_LABELS: Record<SessionStatus, string> = {
  open: "Abierta",
  finished: "Finalizada",
  canceled: "Cancelada",
};

// Etiquetas en español del estado de una ronda de votación.
export const ROUND_STATUS_LABELS: Record<RoundStatus, string> = {
  voting: "En votación",
  revealed: "Revelada",
};

// Etiquetas en español del motivo de una cancelación hecha por el sistema (FR-061, FR-063).
export const CANCEL_REASON_LABELS: Record<CancelReason, string> = {
  inactivity: "Por inactividad",
  story_completed: "Historia completada",
  project_finished: "Proyecto finalizado",
};

// Etiquetas en español del tipo de resultado de una ronda revelada (FR-036 a FR-041).
export const OUTCOME_KIND_LABELS: Record<OutcomeKind, string> = {
  consensus: "Consenso",
  divergence: "Divergencia",
  no_numeric_votes: "Sin votos numéricos",
};
