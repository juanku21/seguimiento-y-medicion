import { useId, type ReactElement } from "react";
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

// Etiquetas en español del estado final de la sesión (Principio VIII).
const CLOSED_STATUS_LABELS: Record<SessionClosedNoticeProps["status"], string> =
  {
    finished: "Finalizada",
    canceled: "Cancelada",
  };

// Aviso de que la sesión de estimación está cerrada, con su resultado (005/US5): el estado en
// español y, si quedó Finalizada, el valor acordado en Story Points, incluido el 0 (FR-052,
// RC-05). La presentación del motivo de una cancelación la completa 005/US7.
export function SessionClosedNotice({
  status,
  agreedStoryPoints,
}: SessionClosedNoticeProps): ReactElement {
  const headingId = useId();
  return (
    <section
      aria-labelledby={headingId}
      className="flex flex-col gap-2 rounded-md border border-zinc-300 px-5 py-4 dark:border-zinc-700"
    >
      <h2 id={headingId} className="text-lg font-semibold">
        Sesión cerrada
      </h2>
      <p className="text-sm">{CLOSED_STATUS_LABELS[status]}</p>
      {agreedStoryPoints !== null && (
        <p className="text-sm font-medium">{agreedStoryPoints} Story Points</p>
      )}
    </section>
  );
}
