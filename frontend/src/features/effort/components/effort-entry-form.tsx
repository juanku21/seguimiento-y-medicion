"use client";

import { useRef, useState, type FormEvent } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { Input } from "@/components/input";
import { createEffortEntry } from "@/features/effort/api";
import {
  validateEffortActivity,
  validateEffortEntryDate,
  validateEffortHours,
} from "@/features/effort/validation";
import type { FieldError } from "@/types/auth";
import type { EffortEntry, EffortEntryInput } from "@/types/effort";

// Valores del formulario tal como se escriben: las horas quedan como el texto del campo numérico
// hasta validarlas.
type FormValues = {
  entryDate: string;
  activity: string;
  hours: string;
};

type FieldName = keyof EffortEntryInput;
type FieldErrors = Partial<Record<FieldName, string>>;

const FIELD_NAMES: readonly FieldName[] = ["entryDate", "activity", "hours"];

// Indicación de la escala de horas, visible desde el inicio (RN-02).
const HOURS_HINT = "Entre 0,25 y 24, en múltiplos de 0,25.";

// Propiedades del formulario: la historia sobre la que se registra y el aviso del alta exitosa.
export type EffortEntryFormProps = {
  storyId: string;
  onCreated?: (entry: EffortEntry) => void;
};

// Devuelve el día de hoy en la zona del navegador con el formato AAAA-MM-DD, el valor de un campo
// de tipo fecha. Se arma con la fecha local y no con toISOString, que daría el día en UTC.
function todayLocal(): string {
  const now = new Date();
  const month = String(now.getMonth() + 1).padStart(2, "0");
  const day = String(now.getDate()).padStart(2, "0");
  return `${now.getFullYear()}-${month}-${day}`;
}

// Indica si un nombre de campo informado por el backend corresponde a un campo del formulario.
function isFieldName(value: string): value is FieldName {
  return FIELD_NAMES.some((name) => name === value);
}

// Aplica las reglas de forma del formulario y devuelve solo los campos que tienen error. La fecha
// futura o anterior al inicio del proyecto y el tope diario los decide el backend.
function validateFields(values: FormValues): FieldErrors {
  const errors: FieldErrors = {};
  const checks: [FieldName, string | null][] = [
    ["entryDate", validateEffortEntryDate(values.entryDate)],
    ["activity", validateEffortActivity(values.activity)],
    ["hours", validateEffortHours(values.hours)],
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

// Formulario de registro de esfuerzo sobre una historia (006/US1). Precarga la fecha de hoy, de modo
// que el registro lleva tres pasos: escribir la actividad, escribir las horas y confirmar (SC-001).
// Valida la forma antes de enviar, envía solo fecha, actividad y horas como número (el autor lo toma
// el backend de la sesión, FR-002) y avisa el alta con el registro devuelto. Los rechazos del backend
// se muestran junto al campo (400) o como mensaje general (404 y 409, con el mensaje recibido).
export function EffortEntryForm({ storyId, onCreated }: EffortEntryFormProps) {
  const [values, setValues] = useState<FormValues>(() => ({
    entryDate: todayLocal(),
    activity: "",
    hours: "",
  }));
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);

  // Actualiza un campo del formulario a partir de lo que se escribió o eligió.
  function updateField(field: FieldName, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
  }

  // Valida y envía el alta. La referencia descarta un segundo envío mientras el primero sigue en
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
    const result = await createEffortEntry(storyId, {
      entryDate: values.entryDate,
      activity: values.activity,
      hours: Number(values.hours.trim()),
    });
    submittingRef.current = false;
    setIsSubmitting(false);

    // El alta exitosa deja la fecha elegida y vacía actividad y horas para el registro siguiente.
    if (result.ok) {
      setValues((current) => ({ ...current, activity: "", hours: "" }));
      onCreated?.(result.data);
      return;
    }

    // Un rechazo muestra el mensaje general y, si el backend lo informa, el detalle junto a cada
    // campo.
    setFieldErrors(toFieldErrors(result.error.fields));
    setGeneralError(result.error.message);
  }

  return (
    <form noValidate onSubmit={handleSubmit} className="flex flex-col gap-5">
      {generalError && <Alert>{generalError}</Alert>}
      <Input
        label="Fecha"
        name="entryDate"
        type="date"
        value={values.entryDate}
        onChange={(event) => updateField("entryDate", event.target.value)}
        error={fieldErrors.entryDate}
      />
      <Input
        label="Actividad"
        name="activity"
        autoComplete="off"
        value={values.activity}
        onChange={(event) => updateField("activity", event.target.value)}
        error={fieldErrors.activity}
      />
      <Input
        label="Horas"
        name="hours"
        type="number"
        inputMode="decimal"
        step={0.25}
        value={values.hours}
        onChange={(event) => updateField("hours", event.target.value)}
        hint={HOURS_HINT}
        error={fieldErrors.hours}
      />
      <Button type="submit" disabled={isSubmitting}>
        Registrar esfuerzo
      </Button>
    </form>
  );
}
