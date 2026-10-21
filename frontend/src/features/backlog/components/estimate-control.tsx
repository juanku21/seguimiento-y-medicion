"use client";

import { useRef, useState } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { FIELD_CONTROL_CLASS_NAME, FormField } from "@/components/form-field";
import { estimateStory } from "@/features/backlog/api";
import type { ApiError } from "@/lib/api-client";
import type { StoryDetail, StoryPoints } from "@/types/story";
import { STORY_POINTS_SCALE } from "@/utils/story-labels";

// Propiedades del control de estimación: la historia que se estima, su valor vigente y el aviso a
// la pantalla de detalle con el StoryDetail que devuelve el backend tras estimar.
export type EstimateControlProps = {
  storyId: string;
  storyPoints: StoryPoints;
  onEstimated: (story: StoryDetail) => void;
};

const UNESTIMATED_LABEL = "Sin estimar";

// Convierte el valor del selector en Story Points de la escala; "" queda sin estimar (null).
function toStoryPoints(value: string): StoryPoints {
  return STORY_POINTS_SCALE.find((points) => String(points) === value) ?? null;
}

// Elige el motivo del rechazo que se muestra: en un 400, el detalle del campo storyPoints, que
// indica la escala admitida; en cualquier otro caso, el mensaje general del backend.
function rejectionMessage(error: ApiError): string {
  const field = error.fields.find((item) => item.field === "storyPoints");
  return field?.message ?? error.message;
}

// Control de estimación de una historia en Story Points (003/US4). El selector ofrece solo
// "Sin estimar" y la escala, arranca con el valor vigente y el botón envía la elección. Con la
// respuesta exitosa avisa a la pantalla de detalle; con un rechazo muestra el motivo y no avisa, de
// modo que el valor vigente no cambia (FR-014 a FR-019).
export function EstimateControl({
  storyId,
  storyPoints,
  onEstimated,
}: EstimateControlProps) {
  const [value, setValue] = useState(
    storyPoints === null ? "" : String(storyPoints),
  );
  const [error, setError] = useState<string | null>(null);
  const submittingRef = useRef(false);

  // Envía la estimación elegida. La referencia descarta un segundo envío mientras el primero sigue
  // en curso.
  async function handleSave() {
    if (submittingRef.current) {
      return;
    }
    submittingRef.current = true;
    setError(null);
    const result = await estimateStory(storyId, toStoryPoints(value));
    submittingRef.current = false;

    if (result.ok) {
      onEstimated(result.data);
      return;
    }
    setError(rejectionMessage(result.error));
  }

  return (
    <section className="flex flex-col gap-3">
      {error && <Alert>{error}</Alert>}
      <FormField label="Story Points">
        {(control) => (
          <select
            {...control}
            name="storyPoints"
            className={`h-11 ${FIELD_CONTROL_CLASS_NAME}`}
            value={value}
            onChange={(event) => setValue(event.target.value)}
          >
            <option value="">{UNESTIMATED_LABEL}</option>
            {STORY_POINTS_SCALE.map((points) => (
              <option key={points} value={String(points)}>
                {points}
              </option>
            ))}
          </select>
        )}
      </FormField>
      <Button onClick={handleSave}>Guardar estimación</Button>
    </section>
  );
}
