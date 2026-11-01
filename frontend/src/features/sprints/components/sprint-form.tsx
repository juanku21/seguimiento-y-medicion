"use client";

import { useRef, useState, type FormEvent } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { Input } from "@/components/input";
import { Textarea } from "@/components/textarea";
import { createSprint, updateSprint } from "@/features/sprints/api";
import {
  MAX_SPRINT_GOAL_LENGTH,
  MAX_SPRINT_NAME_LENGTH,
  validateSprintEndDate,
  validateSprintGoal,
  validateSprintName,
  validateSprintStartDate,
} from "@/features/sprints/validation";
import type { FieldError } from "@/types/auth";
import type { SprintDetail, SprintInput } from "@/types/sprint";
import { formatDate } from "@/utils/format-date";
import { SPRINT_STATUS_LABELS } from "@/utils/sprint-labels";

type FieldName = keyof SprintInput;
type FieldErrors = Partial<Record<FieldName, string>>;

const EMPTY_VALUES: SprintInput = {
  name: "",
  goal: "",
  startDate: "",
  endDate: "",
};

// Indicaciones de los límites, visibles desde el inicio y armadas con las mismas constantes que
// usa la validación (FR-002, FR-003).
const NAME_HINT = `Hasta ${MAX_SPRINT_NAME_LENGTH} caracteres.`;
const GOAL_HINT = `Hasta ${MAX_SPRINT_GOAL_LENGTH} caracteres.`;
const END_DATE_HINT = "Debe ser posterior a la fecha de inicio.";

// Indica si un nombre de campo informado por el backend corresponde a un campo del formulario.
function isFieldName(value: string): value is FieldName {
  return Object.hasOwn(EMPTY_VALUES, value);
}

// Aplica las reglas del formulario (FR-002 a FR-004) y devuelve solo los campos que tienen error.
function validateFields(values: SprintInput): FieldErrors {
  const errors: FieldErrors = {};
  const checks: [FieldName, string | null][] = [
    ["name", validateSprintName(values.name)],
    ["goal", validateSprintGoal(values.goal)],
    ["startDate", validateSprintStartDate(values.startDate)],
    ["endDate", validateSprintEndDate(values.startDate, values.endDate)],
  ];
  for (const [field, message] of checks) {
    if (message) {
      errors[field] = message;
    }
  }
  return errors;
}

// Ubica junto a cada campo el detalle de un 400 del backend, descartando los que no son del
// formulario.
function toFieldErrors(fields: FieldError[]): FieldErrors {
  const errors: FieldErrors = {};
  for (const { field, message } of fields) {
    if (isFieldName(field)) {
      errors[field] = message;
    }
  }
  return errors;
}

// Propiedades del formulario de sprint: el proyecto en el que se planifica el sprint nuevo o, al
// modificar, el sprint Planificado cuyos valores precargan el formulario.
export type SprintFormProps = { projectId: string } | { sprint: SprintDetail };

// Toma del sprint a modificar los valores editables con los que arranca el formulario.
function valuesOf(sprint: SprintDetail): SprintInput {
  return {
    name: sprint.name,
    goal: sprint.goal,
    startDate: sprint.startDate,
    endDate: sprint.endDate,
  };
}

// Formulario de creación (004/US1) y de modificación (004/US7) de un sprint. Valida antes de
// enviar, envía solo nombre, Sprint Goal y fechas (el autor lo toma el backend de la sesión, FR-060;
// el estado y las historias no se modifican, FR-015) y confirma el sprint devuelto por el backend.
// Los rechazos del backend se muestran junto al campo (400) o como mensaje general (404 y 409; el de
// superposición ya nombra el sprint en conflicto). Ante un 401 el cliente HTTP descarta la sesión.
export function SprintForm(props: SprintFormProps) {
  const editedSprint = "sprint" in props ? props.sprint : null;
  const [values, setValues] = useState<SprintInput>(
    editedSprint ? valuesOf(editedSprint) : EMPTY_VALUES,
  );
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [savedSprint, setSavedSprint] = useState<SprintDetail | null>(null);
  const submittingRef = useRef(false);

  // Actualiza un campo del formulario a partir de lo que se escribió o eligió.
  function updateField(field: FieldName, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
  }

  // Valida y envía la creación. La referencia descarta un segundo envío mientras el primero sigue en
  // curso, aun antes de que el botón deshabilitado llegue a renderizarse.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) {
      return;
    }

    const errors = validateFields(values);
    setFieldErrors(errors);
    setGeneralError(null);
    if (Object.keys(errors).length > 0) {
      return;
    }

    submittingRef.current = true;
    setIsSubmitting(true);
    const result =
      "sprint" in props
        ? await updateSprint(props.sprint.id, values)
        : await createSprint(props.projectId, values);
    submittingRef.current = false;
    setIsSubmitting(false);

    if (result.ok) {
      setSavedSprint(result.data);
      return;
    }

    // Un rechazo muestra el mensaje general y, si el backend lo informa, el detalle junto a cada
    // campo.
    setFieldErrors(toFieldErrors(result.error.fields));
    setGeneralError(result.error.message);
  }

  // Confirmación de la modificación con los datos devueltos por el backend, en el lugar del
  // formulario: nombre, estado y fechas del sprint actualizado.
  if (savedSprint && editedSprint) {
    return (
      <Alert variant="success">
        <p className="font-medium">Sprint actualizado</p>
        <p>{savedSprint.name}</p>
        <p>
          Estado: {SPRINT_STATUS_LABELS[savedSprint.status]} ·{" "}
          {formatDate(savedSprint.startDate)} al{" "}
          {formatDate(savedSprint.endDate)}
        </p>
      </Alert>
    );
  }

  // Confirmación de la creación con los datos devueltos por el backend, en el lugar del formulario.
  if (savedSprint) {
    return (
      <Alert variant="success">
        <p className="font-medium">Sprint creado</p>
        <p>{savedSprint.name}</p>
        <p>
          Estado: {SPRINT_STATUS_LABELS[savedSprint.status]} · Creado por{" "}
          {savedSprint.createdBy.fullName}
        </p>
      </Alert>
    );
  }

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-5">
      {generalError && <Alert>{generalError}</Alert>}
      <Input
        label="Nombre"
        name="name"
        autoComplete="off"
        value={values.name}
        onChange={(event) => updateField("name", event.target.value)}
        hint={NAME_HINT}
        error={fieldErrors.name}
      />
      <Textarea
        label="Sprint Goal"
        name="goal"
        value={values.goal}
        onChange={(event) => updateField("goal", event.target.value)}
        hint={GOAL_HINT}
        error={fieldErrors.goal}
      />
      <Input
        label="Fecha de inicio"
        name="startDate"
        type="date"
        value={values.startDate}
        onChange={(event) => updateField("startDate", event.target.value)}
        error={fieldErrors.startDate}
      />
      <Input
        label="Fecha de fin"
        name="endDate"
        type="date"
        value={values.endDate}
        onChange={(event) => updateField("endDate", event.target.value)}
        hint={END_DATE_HINT}
        error={fieldErrors.endDate}
      />
      <Button type="submit" disabled={isSubmitting}>
        {editedSprint ? "Guardar cambios" : "Crear sprint"}
      </Button>
    </form>
  );
}
