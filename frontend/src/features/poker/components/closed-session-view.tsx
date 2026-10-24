"use client";

import { useCallback, type ReactElement } from "react";
import { Alert } from "@/components/alert";
import { getEstimationSession } from "@/features/poker/api";
import { RoundHistory } from "@/features/poker/components/round-history";
import { RoundResult } from "@/features/poker/components/round-result";
import { SessionClosedNotice } from "@/features/poker/components/session-closed-notice";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";
import { formatInstant } from "@/utils/format-date";

// Propiedades de la vista de una sesión cerrada: la sesión de estimación tomada de la ruta.
export type ClosedSessionViewProps = {
  sessionId: string;
};

// Vista de una sesión de estimación ya cerrada (005/US8, US8-1). Pide la sesión con todas sus
// rondas sin unirse a ella y muestra la historia, quién la abrió y quién la cerró con sus momentos,
// el aviso de cierre con el estado, el valor acordado y el motivo, y todas las rondas con sus votos
// y su resultado de la primera a la última: las anteriores en el historial y después la última. Los
// votos de quien dejó el proyecto llegan con su nombre desde el backend (FR-084). Si la consulta se
// rechaza muestra solo el mensaje recibido.
export function ClosedSessionView({
  sessionId,
}: ClosedSessionViewProps): ReactElement {
  const loadSession = useCallback(
    () => getEstimationSession(sessionId),
    [sessionId],
  );
  const query = useAuthorizedQuery(loadSession);

  if (query.status === "error") {
    return <Alert>{query.message}</Alert>;
  }
  if (query.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando la sesión…
      </p>
    );
  }

  const session = query.data;
  const lastRound = session.rounds.at(-1);
  const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  return (
    <div className="flex flex-col gap-6">
      {/* Historia estimada y datos de apertura y cierre de la sesión. */}
      <article className="flex flex-col gap-3 rounded-md border border-zinc-300 px-5 py-5 dark:border-zinc-700">
        <h2 className="text-xl font-semibold break-words">
          {session.story.title}
        </h2>
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
          <dt className="text-zinc-600 dark:text-zinc-400">Abierta por</dt>
          <dd>
            {session.openedBy.fullName} el{" "}
            <time dateTime={session.openedAt}>
              {formatInstant(session.openedAt, timeZone)}
            </time>
          </dd>
          {session.closedAt !== null && (
            <>
              <dt className="text-zinc-600 dark:text-zinc-400">Cerrada por</dt>
              <dd>
                {session.closedBy?.fullName ?? "El sistema"} el{" "}
                <time dateTime={session.closedAt}>
                  {formatInstant(session.closedAt, timeZone)}
                </time>
              </dd>
            </>
          )}
        </dl>
      </article>

      {/* Aviso de cierre con el estado, el valor acordado y el motivo de una cancelación del
          sistema. Solo se muestra si la sesión efectivamente está cerrada. */}
      {session.status !== "open" && (
        <SessionClosedNotice
          status={session.status}
          agreedStoryPoints={session.agreedStoryPoints}
          cancelReason={session.cancelReason}
        />
      )}

      {/* Rondas anteriores a la última con sus votos y su resultado, en orden ascendente. */}
      <RoundHistory rounds={session.rounds} />

      {/* Última ronda de la sesión con sus votos y su resultado, a continuación de las anteriores
          para que todas las rondas se lean de la primera a la última. */}
      {lastRound !== undefined && (
        <section
          aria-label={`Ronda ${lastRound.number}`}
          className="flex flex-col gap-3"
        >
          <h2 className="text-lg font-semibold">Ronda {lastRound.number}</h2>
          <RoundResult round={lastRound} />
        </section>
      )}
    </div>
  );
}
