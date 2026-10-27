"use client";

import { useRef, useState, type FormEvent, type ReactElement } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { Textarea } from "@/components/textarea";
import { updateDefect } from "@/features/defects/api";
import { DefectSelectField } from "@/features/defects/components/defect-select-field";
import { useDefectOptions } from "@/features/defects/use-defect-options";
import {
  toDefectFieldErrors,
  validateDefectDescription,
  validateDefectSeverity,
  type DefectFieldErrors,
  type DefectFieldName,
  type DefectFormValues,
} from "@/features/defects/validation";
import type { Defect, DefectPatch, DefectSeverity } from "@/types/defect";
import { DEFECT_SEVERITY_LABELS } from "@/utils/defect-labels";

// Propiedades del formulario de corrección: el defecto vigente y el aviso de la corrección exitosa.
export type DefectEditFormProps = {
  defect: Defect;
  onUpdated?: (defect: Defect) => void;
};

// Severidades en el orden en que se ofrecen, de la más grave a la menos grave.
const SEVERITIES = Object.keys(DEFECT_SEVERITY_LABELS) as DefectSeverity[];

// Arma el cuerpo del PATCH con los campos cuyo valor difiere del defecto vigente.
function changedFields(defect: Defect, values: DefectFormValues): DefectPatch {
  const patch: DefectPatch = {};
  if (values.description !== defect.description) {
    patch.description = values.description;
  }
  if (values.severity !== defect.severity) {
    patch.severity = values.severity as DefectSeverity;
  }
  if (values.storyId !== defect.story.id) {
    patch.storyId = values.storyId;
  }
  if (values.detectedSprintId !== defect.detectedSprint.id) {
    patch.detectedSprintId = values.detectedSprintId;
  }
  return patch;
}

// Formulario de corrección de los datos de un defecto (007/US4). Parte de los valores vigentes,
// permite corregir descripción y severidad en Abierto, En progreso y Resuelto, y historia y sprint
// de detección solo en Abierto (el backend lo vuelve a exigir con el 409 links_frozen). Envía solo
// los campos cambiados y avisa la corrección con el defecto devuelto. Un rechazo se muestra junto al
// campo (400) o como mensaje general (404 y 409) y conserva lo escrito.
export function DefectEditForm({
  defect,
  onUpdated,
}: DefectEditFormProps): ReactElement {
  const [values, setValues] = useState<DefectFormValues>({
    description: defect.description,
    severity: defect.severity,
    storyId: defect.story.id,
    detectedSprintId: defect.detectedSprint.id,
  });
  const {
    stories,
    sprints,
    error: loadError,
  } = useDefectOptions(defect.projectId);
  const [fieldErrors, setFieldErrors] = useState<DefectFieldErrors>({});
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);

  const areLinksEditable = defect.status === "open";

  // Actualiza un campo del formulario a partir de lo que se escribió o eligió.
  function updateField(field: DefectFieldName, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
  }

  // Valida la forma y envía solo lo cambiado. La referencia descarta un segundo envío mientras el
  // primero sigue en curso.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) {
      return;
    }

    const errors: DefectFieldErrors = {};
    const description = validateDefectDescription(values.description);
    if (description) errors.description = description;
    const severity = validateDefectSeverity(values.severity);
    if (severity) errors.severity = severity;
    setFieldErrors(errors);
    setGeneralError(null);
    if (Object.keys(errors).length > 0) {
      return;
    }

    submittingRef.current = true;
    setIsSubmitting(true);
    const result = await updateDefect(defect.id, changedFields(defect, values));
    submittingRef.current = false;
    setIsSubmitting(false);

    if (result.ok) {
      onUpdated?.(result.data);
      return;
    }

    // Un rechazo muestra el mensaje general y, si el backend lo informa, el detalle junto a cada
    // campo; lo escrito no se toca.
    setFieldErrors(toDefectFieldErrors(result.error.fields));
    setGeneralError(result.error.message);
  }

  const shownError = generalError ?? loadError;

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-5">
      {shownError && <Alert>{shownError}</Alert>}
      <Textarea
        label="Descripción"
        name="description"
        value={values.description}
        onChange={(event) => updateField("description", event.target.value)}
        error={fieldErrors.description}
      />
      <DefectSelectField
        label="Severidad"
        name="severity"
        value={values.severity}
        onChange={(value) => updateField("severity", value)}
        options={SEVERITIES.map((item) => ({
          value: item,
          label: DEFECT_SEVERITY_LABELS[item],
        }))}
        error={fieldErrors.severity}
      />
      <DefectSelectField
        label="Historia"
        name="storyId"
        value={values.storyId}
        onChange={(value) => updateField("storyId", value)}
        options={stories.map((story) => ({
          value: story.id,
          label: story.title,
        }))}
        error={fieldErrors.storyId}
        disabled={!areLinksEditable}
      />
      <DefectSelectField
        label="Sprint de detección"
        name="detectedSprintId"
        value={values.detectedSprintId}
        onChange={(value) => updateField("detectedSprintId", value)}
        options={sprints.map((sprint) => ({
          value: sprint.id,
          label: sprint.name,
        }))}
        error={fieldErrors.detectedSprintId}
        disabled={!areLinksEditable}
      />
      <Button type="submit" disabled={isSubmitting}>
        Guardar cambios
      </Button>
    </form>
  );
}
