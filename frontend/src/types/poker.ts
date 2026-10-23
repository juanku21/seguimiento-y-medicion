// Tipos de las respuestas del Planning Poker, según contracts/openapi.yaml (spec 005). Los errores
// generales (ErrorResponse, ValidationErrorResponse) se reutilizan de @/types/auth.
import type { STORY_POINTS_SCALE } from "@/utils/story-labels";

// Valor numérico de la escala de Story Points de la spec 003 (RC-04).
export type ScaleStoryPoints = (typeof STORY_POINTS_SCALE)[number];

// Carta de votación: un valor de la escala como texto o "?" para decir que no se sabe (FR-020).
export type Card = `${ScaleStoryPoints}` | "?";

// Estado de la sesión de estimación.
export type SessionStatus = "open" | "finished" | "canceled";

// Estado de una ronda de votación.
export type RoundStatus = "voting" | "revealed";

// Motivo de una cancelación hecha por el sistema (FR-061, FR-063).
export type CancelReason =
  "inactivity" | "story_completed" | "project_finished";

// Tipo de resultado de una ronda revelada.
export type OutcomeKind = "consensus" | "divergence" | "no_numeric_votes";

// Persona identificada por su nombre (UserRef, FR-084).
export type UserRef = {
  id: string;
  fullName: string;
};

// Historia estimada con su texto vigente y sin sus Story Points (EstimationStory, FR-075).
export type EstimationStory = {
  id: string;
  projectId: string;
  title: string;
  description: string | null;
  acceptanceCriteria: string[];
};

// Participante de la sesión con su conexión, su rol y si ya votó (EstimationParticipant).
export type EstimationParticipant = {
  user: UserRef;
  connected: boolean;
  isFacilitator: boolean;
  hasVoted: boolean;
};

// Voto visible de una ronda revelada (EstimationVote).
export type EstimationVote = {
  user: UserRef;
  card: Card;
};

// Extremo del resultado con todos sus autores (OutcomeExtreme).
export type OutcomeExtreme = {
  value: ScaleStoryPoints;
  authors: UserRef[];
};

// Resultado de una ronda revelada (RoundOutcome).
export type RoundOutcome = {
  kind: OutcomeKind;
  value?: ScaleStoryPoints | null;
  min?: OutcomeExtreme | null;
  max?: OutcomeExtreme | null;
};

// Ronda de la sesión (EstimationRound).
export type EstimationRound = {
  number: number;
  status: RoundStatus;
  openedAt: string;
  revealedAt: string | null;
  votes: EstimationVote[] | null;
  outcome: RoundOutcome | null;
  notVoted: UserRef[];
};

// Sesión vista por quien pide: cuerpo de las acciones y de cada evento state (EstimationSession).
export type EstimationSession = {
  id: string;
  version: number;
  status: SessionStatus;
  story: EstimationStory;
  facilitator: UserRef;
  openedBy: UserRef;
  openedAt: string;
  closedBy: UserRef | null;
  closedAt: string | null;
  agreedStoryPoints: ScaleStoryPoints | null;
  cancelReason: CancelReason | null;
  cards: Card[];
  participants: EstimationParticipant[];
  rounds: EstimationRound[];
  myVote: Card | null;
  facilitatorAvailableAt: string | null;
};

// Rechazo de la apertura porque la historia ya tiene una sesión abierta (OpenSessionExistsResponse,
// FR-006).
export type OpenSessionExistsResponse = {
  error: string;
  sessionId: string;
};
