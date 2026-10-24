"use client";

import { useState, type ReactElement } from "react";
import { Button } from "@/components/button";
import { FIELD_CONTROL_CLASS_NAME, FormField } from "@/components/form-field";
import type { ScaleStoryPoints } from "@/types/poker";
import { STORY_POINTS_SCALE } from "@/utils/story-labels";

// Propiedades del diálogo de registro: la acción que registra el valor acordado elegido.
export type FinalizeDialogProps = {
  onFinalize: (storyPoints: ScaleStoryPoints) => void;
};

// Diálogo del facilitador para registrar la estimación acordada (005/US5). El selector ofrece solo
// los diez valores numéricos de la escala, sin "?" ni "Sin estimar", y arranca en el primero; el
// botón entrega el valor elegido como número. Que el valor no tenga que coincidir con ningún voto
// y las demás reglas las resuelve el backend (FR-049, FR-050, RN-18).
export function FinalizeDialog({
  onFinalize,
}: FinalizeDialogProps): ReactElement {
  const [value, setValue] = useState<ScaleStoryPoints>(STORY_POINTS_SCALE[0]);

  // Traduce la opción elegida al valor de la escala; una opción ajena a la escala se ignora.
  const handleChange = (selected: string) => {
    const points = STORY_POINTS_SCALE.find(
      (candidate) => String(candidate) === selected,
    );
    if (points !== undefined) {
      setValue(points);
    }
  };

  return (
    <section className="flex flex-col gap-3">
      <FormField label="Estimación acordada">
        {(control) => (
          <select
            {...control}
            name="agreedStoryPoints"
            className={`h-11 ${FIELD_CONTROL_CLASS_NAME}`}
            value={String(value)}
            onChange={(event) => handleChange(event.target.value)}
          >
            {STORY_POINTS_SCALE.map((points) => (
              <option key={points} value={String(points)}>
                {points}
              </option>
            ))}
          </select>
        )}
      </FormField>
      <Button onClick={() => onFinalize(value)}>Registrar estimación</Button>
    </section>
  );
}
