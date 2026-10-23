import type { ReactElement } from "react";
import type { EstimationParticipant } from "@/types/poker";

// Propiedades de la lista de participantes: los participantes tal como llegan en la instantánea.
export type ParticipantListProps = {
  participants: EstimationParticipant[];
};

// Lista de participantes de la sesión con su conexión y la marca de facilitador (005/US1). Es de
// presentación: muestra a cada participante en el orden recibido, y quién está conectado y quién es
// el facilitador lo resuelve el backend en cada instantánea. Quien se desconectó sigue en la lista,
// marcado como desconectado (FR-013, FR-014).
export function ParticipantList({
  participants,
}: ParticipantListProps): ReactElement {
  return (
    <ul aria-label="Participantes" className="flex flex-col gap-3">
      {participants.map(({ user, connected, isFacilitator }) => (
        <li
          key={user.id}
          className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-zinc-300 px-5 py-4 dark:border-zinc-700"
        >
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-base font-medium break-words">
              {user.fullName}
            </span>
            {isFacilitator && (
              <span className="rounded-full bg-zinc-100 px-2.5 py-0.5 text-xs font-medium text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">
                Facilitador
              </span>
            )}
          </div>
          <span
            className={`text-sm ${
              connected
                ? "text-green-700 dark:text-green-400"
                : "text-zinc-600 dark:text-zinc-400"
            }`}
          >
            {connected ? "Conectado" : "Desconectado"}
          </span>
        </li>
      ))}
    </ul>
  );
}
