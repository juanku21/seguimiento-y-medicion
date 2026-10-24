import type { ReactElement } from "react";
import type {
  CancelReason,
  ScaleStoryPoints,
  SessionStatus,
} from "@/types/poker";

// Propiedades del aviso de cierre: el estado final de la sesión, el valor acordado (solo si quedó
// Finalizada) y el motivo de una cancelación hecha por el sistema.
export type SessionClosedNoticeProps = {
  status: Exclude<SessionStatus, "open">;
  agreedStoryPoints: ScaleStoryPoints | null;
  cancelReason: CancelReason | null;
};

// Aviso de que la sesión de estimación está cerrada, con su resultado (005/US5). Firma mínima para
// las pruebas de T064; la implementación llega en T069.
export function SessionClosedNotice({
  status,
  agreedStoryPoints,
  cancelReason,
}: SessionClosedNoticeProps): ReactElement {
  void status;
  void agreedStoryPoints;
  void cancelReason;
  throw new Error("not implemented");
}
