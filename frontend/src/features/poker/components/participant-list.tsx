import type { ReactElement } from "react";
import type { EstimationParticipant } from "@/types/poker";

// Propiedades de la lista de participantes: los participantes tal como llegan en la instantánea.
export type ParticipantListProps = {
  participants: EstimationParticipant[];
};

// Lista de participantes de la sesión con su conexión, la marca de facilitador (005/US1) y la marca
// "Ya votó" (005/US2). Es de presentación: muestra a cada participante en el orden recibido, y quién
// está conectado, quién es el facilitador y quién ya votó lo resuelve el backend en cada instantánea.
// Quien se desconectó sigue en la lista, marcado como desconectado (FR-013, FR-014); de quien votó
// no se muestra la carta, que sigue en secreto (FR-023).
export function ParticipantList({
  participants,
}: ParticipantListProps): ReactElement {
  return (
    <ul aria-label="Participantes" className="flex flex-col">
      {participants.map(({ user, connected, isFacilitator, hasVoted }) => (
        <li
          key={user.id}
          className="flex flex-wrap items-center justify-between gap-3 border-t border-border py-3 first:border-t-0 first:pt-0 last:pb-0"
        >
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-base font-medium break-words">
              {user.fullName}
            </span>
            {isFacilitator && (
              <span className="rounded-full bg-surface-muted px-2.5 py-0.5 text-xs font-medium text-foreground-muted">
                Facilitador
              </span>
            )}
            {hasVoted && (
              <span className="rounded-full bg-success-soft px-2.5 py-0.5 text-xs font-medium text-success">
                Ya votó
              </span>
            )}
          </div>
          <span
            className={`text-sm ${
              connected ? "text-success" : "text-foreground-muted"
            }`}
          >
            {connected ? "Conectado" : "Desconectado"}
          </span>
        </li>
      ))}
    </ul>
  );
}
