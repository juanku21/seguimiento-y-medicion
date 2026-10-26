"use client";

import { useRef, useState, type FormEvent } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { Input } from "@/components/input";
import { createEffortEntry } from "@/features/effort/api";
import {
  EFFORT_HOURS_HINT,
  toEffortFieldErrors,
  validateEffortFields,
  type EffortFieldErrors,
  type EffortFieldName,
  type EffortFormValues,
} from "@/features/effort/validation";
import type { EffortEntry } from "@/types/effort";

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

// Formulario de registro de esfuerzo sobre una historia (006/US1). Precarga la fecha de hoy, de modo
// que el registro lleva tres pasos: escribir la actividad, escribir las horas y confirmar (SC-001).
// Valida la forma antes de enviar, envía solo fecha, actividad y horas como número (el autor lo toma
// el backend de la sesión, FR-002) y avisa el alta con el registro devuelto. Los rechazos del backend
// se muestran junto al campo (400) o como mensaje general (404 y 409, con el mensaje recibido).
export function EffortEntryForm({ storyId, onCreated }: EffortEntryFormProps) {
  const [values, setValues] = useState<EffortFormValues>(() => ({
    entryDate: todayLocal(),
    activity: "",
    hours: "",
  }));
  const [fieldErrors, setFieldErrors] = useState<EffortFieldErrors>({});
  const [generalError, setGeneralError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);

  // Actualiza un campo del formulario a partir de lo que se escribió o eligió.
  function updateField(field: EffortFieldName, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
  }

  // Valida y envía el alta. La referencia descarta un segundo envío mientras el primero sigue en
  // curso, aun antes de que el botón deshabilitado llegue a renderizarse.
  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) {
      return;
    }

    const errors = validateEffortFields(values);
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
    setFieldErrors(toEffortFieldErrors(result.error.fields));
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
        hint={EFFORT_HOURS_HINT}
        error={fieldErrors.hours}
      />
      <Button type="submit" disabled={isSubmitting}>
        Registrar esfuerzo
      </Button>
    </form>
  );
}
