"use client";

import { useSearchParams } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { FIELD_CONTROL_CLASS_NAME, FormField } from "@/components/form-field";
import { Textarea } from "@/components/textarea";
import { listStories } from "@/features/backlog/api";
import { createDefect, listDetectionSprints } from "@/features/defects/api";
import {
  toDefectFieldErrors,
  validateDefectFields,
  type DefectFieldErrors,
  type DefectFieldName,
  type DefectFormValues,
} from "@/features/defects/validation";
import type { Defect, DefectSeverity } from "@/types/defect";
import type { SprintListItem } from "@/types/sprint";
import type { StoryListItem } from "@/types/story";
import { DEFECT_SEVERITY_LABELS } from "@/utils/defect-labels";

// Propiedades del formulario: el proyecto donde se registra y el aviso del alta exitosa.
export type DefectFormProps = {
  projectId: string;
  onCreated?: (defect: Defect) => void;
};

const NO_SPRINTS_NOTICE = "Todavía no hay ningún sprint Activo ni Cerrado.";

// Severidades en el orden en que se ofrecen, de la más grave a la menos grave.
const SEVERITIES = Object.keys(DEFECT_SEVERITY_LABELS) as DefectSeverity[];

type SelectFieldProps = {
  label: string;
  name: string;
  value: string;
  onChange: (value: string) => void;
  options: { value: string; label: string }[];
  error?: string;
};

// Select nativo con su etiqueta y los estilos comunes de los campos; su primera opción es vacía.
function SelectField({
  label,
  name,
  value,
  onChange,
  options,
  error,
}: SelectFieldProps) {
  return (
    <FormField label={label} error={error}>
      {(control) => (
        <select
          {...control}
          name={name}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          className={`h-11 ${FIELD_CONTROL_CLASS_NAME}`}
        >
          <option value=""></option>
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      )}
    </FormField>
  );
}

// Formulario de registro de un defecto (007/US1). Carga las historias del backlog y los sprints
// Activos y Cerrados, propone el sprint Activo y la historia de ?storyId= si llega, de modo que el
// registro lleva cinco pasos (cuatro desde la historia, SC-001). Valida la forma antes de enviar,
// envía solo los cuatro campos del defecto (el autor lo toma el backend de la sesión, FR-056) y
// avisa el alta con el defecto devuelto. Los rechazos del backend se muestran junto al campo (400)
// o como mensaje general (404 y 409, con el mensaje recibido).
export function DefectForm({ projectId, onCreated }: DefectFormProps) {
  const presetStoryId = useSearchParams().get("storyId") ?? "";
  const [values, setValues] = useState<DefectFormValues>({
    description: "",
    severity: "",
    storyId: presetStoryId,
    detectedSprintId: "",
  });
  const [stories, setStories] = useState<StoryListItem[]>([]);
  const [sprints, setSprints] = useState<SprintListItem[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [fieldErrors, setFieldErrors] = useState<DefectFieldErrors>({});
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);

  // Carga historias y sprints al montar y propone el sprint Activo, si existe.
  useEffect(() => {
    let cancelled = false;
    async function load() {
      const [storiesResult, sprintsResult] = await Promise.all([
        listStories(projectId, { status: [], priority: [] }),
        listDetectionSprints(projectId),
      ]);
      if (cancelled) {
        return;
      }
      if (storiesResult.ok) {
        setStories(storiesResult.data.stories);
      } else {
        setGeneralError(storiesResult.error.message);
      }
      if (sprintsResult.ok) {
        const loaded = sprintsResult.data.sprints;
        setSprints(loaded);
        const active = loaded.find((sprint) => sprint.status === "active");
        if (active) {
          setValues((current) => ({ ...current, detectedSprintId: active.id }));
        }
      } else {
        setGeneralError(sprintsResult.error.message);
      }
      setIsLoading(false);
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  // Actualiza un campo del formulario a partir de lo que se escribió o eligió.
  function updateField(field: DefectFieldName, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
  }

  // Valida y envía el alta. La referencia descarta un segundo envío mientras el primero sigue en
  // curso, aun antes de que el botón deshabilitado llegue a renderizarse.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) {
      return;
    }

    const errors = validateDefectFields(values);
    setFieldErrors(errors);
    setGeneralError(null);
    if (Object.keys(errors).length > 0) {
      return;
    }

    submittingRef.current = true;
    setIsSubmitting(true);
    const result = await createDefect(projectId, {
      description: values.description,
      severity: values.severity as DefectSeverity,
      storyId: values.storyId,
      detectedSprintId: values.detectedSprintId,
    });
    submittingRef.current = false;
    setIsSubmitting(false);

    if (result.ok) {
      onCreated?.(result.data);
      return;
    }

    // Un rechazo muestra el mensaje general y, si el backend lo informa, el detalle junto a cada
    // campo.
    setFieldErrors(toDefectFieldErrors(result.error.fields));
    setGeneralError(result.error.message);
  }

  const hasNoSprints = !isLoading && sprints.length === 0;

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-5">
      {generalError && <Alert>{generalError}</Alert>}
      {hasNoSprints && <Alert>{NO_SPRINTS_NOTICE}</Alert>}
      <Textarea
        label="Descripción"
        name="description"
        value={values.description}
        onChange={(event) => updateField("description", event.target.value)}
        error={fieldErrors.description}
      />
      <SelectField
        label="Severidad"
        name="severity"
        value={values.severity}
        onChange={(value) => updateField("severity", value)}
        options={SEVERITIES.map((severity) => ({
          value: severity,
          label: DEFECT_SEVERITY_LABELS[severity],
        }))}
        error={fieldErrors.severity}
      />
      <SelectField
        label="Historia"
        name="storyId"
        value={values.storyId}
        onChange={(value) => updateField("storyId", value)}
        options={stories.map((story) => ({
          value: story.id,
          label: story.title,
        }))}
        error={fieldErrors.storyId}
      />
      <SelectField
        label="Sprint de detección"
        name="detectedSprintId"
        value={values.detectedSprintId}
        onChange={(value) => updateField("detectedSprintId", value)}
        options={sprints.map((sprint) => ({
          value: sprint.id,
          label: sprint.name,
        }))}
        error={fieldErrors.detectedSprintId}
      />
      <Button
        type="submit"
        disabled={isSubmitting || isLoading || hasNoSprints}
      >
        Registrar defecto
      </Button>
    </form>
  );
}
