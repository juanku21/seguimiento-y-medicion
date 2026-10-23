"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRef, useState, type ReactElement } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { openEstimationSession } from "@/features/poker/api";
import type { ApiError } from "@/lib/api-client";

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

// Sección "Planning Poker" del detalle de una historia: abre una sesión o se une a la que está en
// curso (005/US1). No envía nada hasta que se pulsa el botón. Con el 201 lleva a la sala de la
// sesión creada (FR-001, FR-002); con el 409 de sesión en curso muestra el mensaje y ofrece el
// enlace para unirse a esa sesión (FR-006); con cualquier otro rechazo, como la historia Completada
// o el proyecto Finalizado, muestra solo su mensaje (FR-007, FR-008).
export function StoryEstimationSection({
  projectId,
  storyId,
}: StoryEstimationSectionProps): ReactElement {
  const router = useRouter();
  const [error, setError] = useState<ApiError | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const submittingRef = useRef(false);

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
    </section>
  );
}
