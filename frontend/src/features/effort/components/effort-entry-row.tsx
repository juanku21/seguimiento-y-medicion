"use client";

import { useId, useRef, useState, type FormEvent } from "react";
import { Alert } from "@/components/alert";
import { Input } from "@/components/input";
import { deleteEffortEntry, updateEffortEntry } from "@/features/effort/api";
import {
  EFFORT_HOURS_HINT,
  toEffortFieldErrors,
  validateEffortFields,
  type EffortFieldErrors,
  type EffortFieldName,
  type EffortFormValues,
} from "@/features/effort/validation";
import type { EffortEntry, EffortEntryPatch } from "@/types/effort";
import { formatHours } from "@/utils/effort-format";
import { formatDate } from "@/utils/format-date";

// Modo de la fila: solo lectura, edición en línea o confirmación de la eliminación.
type RowMode = "view" | "edit" | "confirm";

// Clases de los botones de la fila: el secundario para editar y cancelar, el principal para guardar
// y el de peligro para eliminar.
const secondaryButtonClass =
  "rounded-md border border-zinc-300 px-3 py-1.5 text-sm font-medium text-zinc-900 transition-colors hover:bg-zinc-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-100 dark:hover:bg-zinc-800";
const primaryButtonClass =
  "rounded-md bg-zinc-900 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-zinc-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 disabled:opacity-60 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-300";
const dangerButtonClass =
  "rounded-md bg-red-700 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-red-600 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-700 disabled:opacity-60";

// Propiedades de una fila de la lista de esfuerzo: el registro, si quien tiene la sesión puede
// corregirlo o eliminarlo, y el aviso de que una de esas acciones se completó.
export type EffortEntryRowProps = {
  entry: EffortEntry;
  canModify: boolean;
  onChanged?: () => void;
};

// Toma los valores del registro como punto de partida de la edición en línea.
function valuesOf(entry: EffortEntry): EffortFormValues {
  return {
    entryDate: entry.entryDate,
    activity: entry.activity,
    hours: String(entry.hours),
  };
}

// Arma la corrección con solo los campos que difieren del registro, con las horas como número.
function changedFields(
  entry: EffortEntry,
  values: EffortFormValues,
): EffortEntryPatch {
  const patch: EffortEntryPatch = {};
  if (values.entryDate !== entry.entryDate) {
    patch.entryDate = values.entryDate;
  }
  if (values.activity !== entry.activity) {
    patch.activity = values.activity;
  }
  const hours = Number(values.hours.trim());
  if (hours !== entry.hours) {
    patch.hours = hours;
  }
  return patch;
}

// Fila de un registro de esfuerzo (006/US2 y 006/US3). Muestra fecha, autor, actividad, horas y
// sprint; si el registro es propio y no es definitivo ofrece corregirlo en línea (fecha, actividad y
// horas, enviando solo lo cambiado) y eliminarlo previa confirmación. Con el 200 o el 204 avisa con
// onChanged para que se recargue el esfuerzo; con un rechazo muestra el mensaje del backend y, en la
// edición, conserva lo escrito.
export function EffortEntryRow({
  entry,
  canModify,
  onChanged,
}: EffortEntryRowProps) {
  const [mode, setMode] = useState<RowMode>("view");
  const [values, setValues] = useState<EffortFormValues>(() => valuesOf(entry));
  const [fieldErrors, setFieldErrors] = useState<EffortFieldErrors>({});
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);
  const questionId = useId();

  // Abre un modo de la fila descartando el mensaje anterior; la edición parte de los valores
  // vigentes del registro, sin arrastrar lo escrito en una edición cancelada.
  function openMode(next: RowMode) {
    setError(null);
    setFieldErrors({});
    if (next === "edit") {
      setValues(valuesOf(entry));
    }
    setMode(next);
  }

  // Actualiza un campo de la edición a partir de lo que se escribió o eligió.
  function updateField(field: EffortFieldName, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
  }

  // Valida y envía la corrección con solo los campos cambiados; sin cambios cierra la edición sin
  // enviar nada. La referencia descarta un segundo envío mientras el primero sigue en curso.
  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submittingRef.current) {
      return;
    }

    const errors = validateEffortFields(values);
    setFieldErrors(errors);
    setError(null);
    if (Object.keys(errors).length > 0) {
      return;
    }
    const patch = changedFields(entry, values);
    if (Object.keys(patch).length === 0) {
      setMode("view");
      return;
    }

    submittingRef.current = true;
    setIsSubmitting(true);
    const result = await updateEffortEntry(entry.id, patch);
    submittingRef.current = false;
    setIsSubmitting(false);

    if (result.ok) {
      setMode("view");
      onChanged?.();
      return;
    }
    setFieldErrors(toEffortFieldErrors(result.error.fields));
    setError(result.error.message);
  }

  // Envía la eliminación confirmada; con un rechazo vuelve a la vista con el mensaje del backend.
  async function confirmDelete() {
    if (submittingRef.current) {
      return;
    }
    submittingRef.current = true;
    setIsSubmitting(true);
    const result = await deleteEffortEntry(entry.id);
    submittingRef.current = false;
    setIsSubmitting(false);
    setMode("view");

    if (result.ok) {
      onChanged?.();
      return;
    }
    setError(result.error.message);
  }

  return (
    <li className="flex flex-col gap-2 rounded-md border border-zinc-300 px-4 py-3 text-sm dark:border-zinc-700">
      {error && <Alert>{error}</Alert>}

      {/* Edición en línea: los tres campos precargados, guardar y cancelar sin enviar nada. */}
      {mode === "edit" ? (
        <form noValidate onSubmit={handleSave} className="flex flex-col gap-4">
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
          <div className="flex flex-wrap gap-2">
            <button
              type="submit"
              disabled={isSubmitting}
              className={primaryButtonClass}
            >
              Guardar cambios
            </button>
            <button
              type="button"
              onClick={() => openMode("view")}
              disabled={isSubmitting}
              className={secondaryButtonClass}
            >
              Cancelar
            </button>
          </div>
        </form>
      ) : (
        <>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="tabular-nums">{formatDate(entry.entryDate)}</span>
            <span className="font-medium tabular-nums">
              {formatHours(entry.hours)}
            </span>
          </div>
          <span className="break-words">{entry.activity}</span>
          <div className="flex flex-wrap gap-x-4 gap-y-1 text-zinc-600 dark:text-zinc-400">
            <span>{entry.author.fullName}</span>
            <span>{entry.sprint ? entry.sprint.name : "Sin sprint"}</span>
          </div>
        </>
      )}

      {/* Acciones del registro propio y no definitivo; eliminar primero pide confirmación. */}
      {canModify && mode === "view" && (
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => openMode("edit")}
            className={secondaryButtonClass}
          >
            Editar
          </button>
          <button
            type="button"
            onClick={() => openMode("confirm")}
            className={secondaryButtonClass}
          >
            Eliminar
          </button>
        </div>
      )}
      {canModify && mode === "confirm" && (
        <div
          aria-labelledby={questionId}
          role="group"
          className="flex flex-col gap-2"
        >
          <p id={questionId} className="font-medium">
            ¿Eliminar este registro?
          </p>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={confirmDelete}
              disabled={isSubmitting}
              className={dangerButtonClass}
            >
              Confirmar eliminación
            </button>
            <button
              type="button"
              onClick={() => openMode("view")}
              disabled={isSubmitting}
              className={secondaryButtonClass}
            >
              Cancelar
            </button>
          </div>
        </div>
      )}
    </li>
  );
}
