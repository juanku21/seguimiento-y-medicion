"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useRef, useState, type ReactElement } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import {
  getStoryEstimationHistory,
  openEstimationSession,
} from "@/features/poker/api";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";
import type { ApiError } from "@/lib/api-client";
import type {
  EstimationSessionSummary,
  OpenEstimationSession,
} from "@/types/poker";
import { formatInstant } from "@/utils/format-date";
import {
  CANCEL_REASON_LABELS,
  ROUND_STATUS_LABELS,
  SESSION_STATUS_LABELS,
} from "@/utils/poker-labels";

// Propiedades de la sección "Planning Poker" del detalle de una historia: el proyecto y la historia
// tomados de la ruta.
export type StoryEstimationSectionProps = {
  projectId: string;
  storyId: string;
};

// Arma la ruta de la sala en vivo de una sesión de la historia, codificando cada identificador.
function roomHref(projectId: string, storyId: string, sessionId: string) {
  return `/projects/${encodeURIComponent(projectId)}/backlog/${encodeURIComponent(storyId)}/poker/${encodeURIComponent(sessionId)}`;
}

// Cantidad de rondas de una sesión con el sustantivo en singular o plural.
function roundCountLabel(count: number): string {
  return count === 1 ? "1 ronda" : `${count} rondas`;
}

// Sesión en curso de la historia con su facilitador, su número de ronda y el estado de la ronda, y
// el enlace para unirse a su sala (005/US8, FR-081). No muestra votos: el historial no los trae.
function OpenSessionNotice({
  session,
  href,
}: {
  session: OpenEstimationSession;
  href: string;
}): ReactElement {
  return (
    <div className="flex flex-col gap-1 rounded-md border border-zinc-300 px-4 py-3 text-sm dark:border-zinc-700">
      <p className="font-medium">Sesión en curso</p>
      <p>Facilitador: {session.facilitator.fullName}</p>
      <p>
        Ronda {session.roundNumber} ({ROUND_STATUS_LABELS[session.roundStatus]})
      </p>
      <Link
        href={href}
        className="self-start font-medium underline underline-offset-4"
      >
        Unirse
      </Link>
    </div>
  );
}

// Elemento del historial con una sesión cerrada: estado en español, valor acordado solo si lo hay,
// motivo de una cancelación del sistema, cantidad de rondas, quién la abrió y cuándo cerró, y el
// enlace a la vista de la sesión con sus rondas y votos (005/US8, FR-079, FR-084).
function SessionSummaryItem({
  session,
  href,
  timeZone,
}: {
  session: EstimationSessionSummary;
  href: string;
  timeZone: string;
}): ReactElement {
  return (
    <li className="flex flex-col gap-1 rounded-md border border-zinc-300 px-4 py-3 text-sm dark:border-zinc-700">
      <p className="font-medium">
        {SESSION_STATUS_LABELS[session.status]}
        {session.agreedStoryPoints !== null &&
          ` · ${session.agreedStoryPoints} Story Points`}
      </p>
      {session.cancelReason !== null && (
        <p>{CANCEL_REASON_LABELS[session.cancelReason]}</p>
      )}
      <p>{roundCountLabel(session.roundCount)}</p>
      <p className="text-zinc-600 dark:text-zinc-400">
        Abierta por {session.openedBy.fullName} el{" "}
        <time dateTime={session.openedAt}>
          {formatInstant(session.openedAt, timeZone)}
        </time>
        , cerrada el{" "}
        <time dateTime={session.closedAt}>
          {formatInstant(session.closedAt, timeZone)}
        </time>
      </p>
      <Link
        href={href}
        className="self-start font-medium underline underline-offset-4"
      >
        Ver sesión
      </Link>
    </li>
  );
}

// Sección "Planning Poker" del detalle de una historia: abre una sesión o se une a la que está en
// curso (005/US1). No envía nada hasta que se pulsa el botón. Con el 201 lleva a la sala de la
// sesión creada (FR-001, FR-002); con el 409 de sesión en curso muestra el mensaje y ofrece el
// enlace para unirse a esa sesión (FR-006); con cualquier otro rechazo, como la historia Completada
// o el proyecto Finalizado, muestra solo su mensaje (FR-007, FR-008). Al montarse pide el historial
// de estimación (005/US8): muestra la sesión en curso con el enlace para unirse, las sesiones
// cerradas en el orden recibido o la indicación de que la historia no tiene sesiones (FR-079 a
// FR-083). Un fallo del historial se informa como texto, sin role="alert", para no confundirlo con
// el rechazo de la apertura.
export function StoryEstimationSection({
  projectId,
  storyId,
}: StoryEstimationSectionProps): ReactElement {
  const router = useRouter();
  const [error, setError] = useState<ApiError | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);
  const loadHistory = useCallback(
    () => getStoryEstimationHistory(storyId),
    [storyId],
  );
  const history = useAuthorizedQuery(loadHistory);
  const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;

  // Envía la apertura. La referencia descarta un segundo envío mientras el primero sigue en curso.
  async function handleOpen() {
    if (submittingRef.current) {
      return;
    }
    submittingRef.current = true;
    setSubmitting(true);
    setError(null);
    const result = await openEstimationSession(storyId);
    submittingRef.current = false;
    setSubmitting(false);

    if (result.ok) {
      router.push(roomHref(projectId, storyId, result.data.id));
      return;
    }
    setError(result.error);
  }

  return (
    <section
      aria-labelledby="planning-poker-title"
      className="flex flex-col gap-3"
    >
      <h2 id="planning-poker-title" className="text-lg font-semibold">
        Planning Poker
      </h2>

      {/* Rechazo de la apertura: el mensaje del backend y, si ya hay una sesión en curso, el
          enlace para unirse a ella en lugar de abrir otra. */}
      {error && (
        <Alert>
          <p>{error.message}</p>
          {error.sessionId !== undefined && (
            <Link
              href={roomHref(projectId, storyId, error.sessionId)}
              className="mt-2 inline-block font-medium underline underline-offset-4"
            >
              Unirse
            </Link>
          )}
        </Alert>
      )}

      <Button onClick={handleOpen} disabled={submitting}>
        Abrir Planning Poker
      </Button>

      {/* Historial de estimación: la sesión en curso, las sesiones cerradas o la indicación de que
          la historia no tiene sesiones; un fallo de la consulta se informa como texto. */}
      {history.status === "error" && (
        <p className="text-sm text-zinc-600 dark:text-zinc-400">
          {history.message}
        </p>
      )}
      {history.status === "loaded" && (
        <>
          {history.data.openSession !== null && (
            <OpenSessionNotice
              session={history.data.openSession}
              href={roomHref(projectId, storyId, history.data.openSession.id)}
            />
          )}
          {history.data.sessions.length > 0 && (
            <ul
              aria-label="Historial de estimación"
              className="flex flex-col gap-2"
            >
              {history.data.sessions.map((session) => (
                <SessionSummaryItem
                  key={session.id}
                  session={session}
                  href={roomHref(projectId, storyId, session.id)}
                  timeZone={timeZone}
                />
              ))}
            </ul>
          )}
          {history.data.emptyReason === "no_sessions" && (
            <p className="text-sm text-zinc-600 dark:text-zinc-400">
              La historia no tiene sesiones de estimación.
            </p>
          )}
        </>
      )}
    </section>
  );
}
