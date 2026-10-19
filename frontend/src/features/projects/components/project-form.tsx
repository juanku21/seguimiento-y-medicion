"use client";

import { useRef, useState, type FormEvent } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { Input } from "@/components/input";
import { Textarea } from "@/components/textarea";
import { createProject } from "@/features/projects/api";
import {
  MAX_DESCRIPTION_LENGTH,
  MAX_HOURS_PER_STORY_POINT,
  MAX_HOURS_PER_STORY_POINT_DECIMALS,
  MAX_PROJECT_NAME_LENGTH,
  MIN_HOURS_PER_STORY_POINT,
  validateDescription,
  validateHoursPerStoryPoint,
  validatePlannedEndDate,
  validateProjectName,
  validateStartDate,
} from "@/features/projects/validation";
import type { FieldError } from "@/types/auth";
import type { ProjectDetail } from "@/types/project";
import { PROJECT_STATUS_LABELS } from "@/utils/project-status";

// Valores del formulario tal como se escriben; el factor se guarda como texto para validarlo sin
// redondear y recién se convierte a número al enviarlo.
type FormValues = {
  name: string;
  description: string;
  startDate: string;
  plannedEndDate: string;
  hoursPerStoryPoint: string;
};

type FieldName = keyof FormValues;
type FieldErrors = Partial<Record<FieldName, string>>;

const EMPTY_VALUES: FormValues = {
  name: "",
  description: "",
  startDate: "",
  plannedEndDate: "",
  hoursPerStoryPoint: "",
};

// Indicaciones de los límites, visibles desde el inicio y armadas con las mismas constantes que
// usa la validación (FR-003, RN-07).
const NAME_HINT = `Hasta ${MAX_PROJECT_NAME_LENGTH} caracteres.`;
const DESCRIPTION_HINT = `Opcional, hasta ${MAX_DESCRIPTION_LENGTH} caracteres.`;
const HOURS_PER_STORY_POINT_HINT = `Entre ${MIN_HOURS_PER_STORY_POINT} y ${MAX_HOURS_PER_STORY_POINT}, con hasta ${MAX_HOURS_PER_STORY_POINT_DECIMALS} decimales. Usá punto decimal, por ejemplo 6.5.`;

// Indica si un nombre de campo informado por el backend corresponde a un campo del formulario.
function isFieldName(value: string): value is FieldName {
  return Object.hasOwn(EMPTY_VALUES, value);
}

// Aplica las reglas del formulario a los valores escritos y devuelve solo los campos con error.
function validate(values: FormValues): FieldErrors {
  const errors: FieldErrors = {};
  const checks: [FieldName, string | null][] = [
    ["name", validateProjectName(values.name)],
    ["description", validateDescription(values.description)],
    ["startDate", validateStartDate(values.startDate)],
    [
      "plannedEndDate",
      validatePlannedEndDate(values.startDate, values.plannedEndDate),
    ],
    [
      "hoursPerStoryPoint",
      validateHoursPerStoryPoint(values.hoursPerStoryPoint),
    ],
  ];
  for (const [field, message] of checks) {
    if (message) {
      errors[field] = message;
    }
  }
  return errors;
}

// Ubica junto a cada campo el detalle que informa el backend, descartando los que no son del
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

// Propiedades del formulario: con project, el formulario edita ese proyecto (US5); sin él, crea
// uno nuevo (US1).
type ProjectFormProps = {
  project?: ProjectDetail;
};

// Formulario de creación de proyecto (US1). Valida antes de enviar y, al crearse el proyecto,
// confirma su nombre y su estado sin redirigir. Ante un 401 el cliente HTTP descarta la sesión.
export function ProjectForm(_props: ProjectFormProps = {}) {
  const [values, setValues] = useState<FormValues>(EMPTY_VALUES);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [createdProject, setCreatedProject] = useState<ProjectDetail | null>(
    null,
  );
  const submittingRef = useRef(false);

  // Actualiza un campo del formulario a partir de lo que se escribió.
  function updateField(field: FieldName, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
  }

  // Envía la creación. La referencia descarta un segundo envío mientras el primero sigue en curso,
  // aun antes de que el botón deshabilitado llegue a renderizarse.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) {
      return;
    }

    const errors = validate(values);
    setFieldErrors(errors);
    setGeneralError(null);
    if (Object.keys(errors).length > 0) {
      return;
    }

    submittingRef.current = true;
    setIsSubmitting(true);
    const result = await createProject({
      ...values,
      hoursPerStoryPoint: Number(values.hoursPerStoryPoint),
    });
    submittingRef.current = false;
    setIsSubmitting(false);

    if (result.ok) {
      setCreatedProject(result.data);
      return;
    }

    // Un rechazo muestra el mensaje general y, si el backend lo informa, el detalle junto a cada
    // campo.
    setFieldErrors(toFieldErrors(result.error.fields));
    setGeneralError(result.error.message);
  }

  // Confirmación de la creación: la pantalla de estado del proyecto todavía no existe, así que se
  // muestran su nombre y su estado en el lugar del formulario.
  if (createdProject) {
    return (
      <Alert variant="success">
        <p className="font-medium">Proyecto creado</p>
        <p>
          {createdProject.name} · Estado:{" "}
          {PROJECT_STATUS_LABELS[createdProject.status]}
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
        label="Descripción"
        name="description"
        value={values.description}
        onChange={(event) => updateField("description", event.target.value)}
        hint={DESCRIPTION_HINT}
        error={fieldErrors.description}
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
        label="Fecha de finalización prevista"
        name="plannedEndDate"
        type="date"
        value={values.plannedEndDate}
        onChange={(event) => updateField("plannedEndDate", event.target.value)}
        error={fieldErrors.plannedEndDate}
      />
      <Input
        label="Horas por Story Point"
        name="hoursPerStoryPoint"
        inputMode="decimal"
        autoComplete="off"
        value={values.hoursPerStoryPoint}
        onChange={(event) =>
          updateField("hoursPerStoryPoint", event.target.value)
        }
        hint={HOURS_PER_STORY_POINT_HINT}
        error={fieldErrors.hoursPerStoryPoint}
      />
      <Button type="submit" disabled={isSubmitting}>
        Crear proyecto
      </Button>
    </form>
  );
}
