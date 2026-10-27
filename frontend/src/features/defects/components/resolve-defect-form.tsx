"use client";

import { useEffect, useState, type FormEvent } from "react";
import { Alert } from "@/components/alert";
import { FIELD_CONTROL_CLASS_NAME, FormField } from "@/components/form-field";
import { listDetectionSprints } from "@/features/defects/api";
import type { SprintListItem } from "@/types/sprint";

// Propiedades del formulario de resolución: el proyecto del defecto, el error del sprint de
// resolución que informó el backend, si hay un envío en curso, y los avisos de confirmar (con el
// sprint elegido o "" si no se eligió ninguno) y de cancelar.
export type ResolveDefectFormProps = {
  projectId: string;
  error?: string;
  isSubmitting: boolean;
  onConfirm: (resolvedSprintId: string) => void;
  onCancel: () => void;
};

// Clases del botón principal, compartida con las acciones de estado, y del secundario.
export const primaryButtonClass =
  "rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-zinc-700 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 disabled:opacity-60 dark:bg-zinc-100 dark:text-zinc-900 dark:hover:bg-zinc-300";
const secondaryButtonClass =
  "rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-900 transition-colors hover:bg-zinc-100 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-zinc-900 disabled:opacity-60 dark:border-zinc-700 dark:text-zinc-100 dark:hover:bg-zinc-800";

// Formulario para elegir el sprint de resolución de un defecto (007/US2). Ofrece los sprints
// Activos y Cerrados del proyecto, los mismos que admite la detección, y nunca uno Planificado
// (FR-024). Confirmar no se bloquea por falta de sprint: la regla la resuelve el backend, cuyo
// rechazo llega en error y se muestra junto al selector.
export function ResolveDefectForm({
  projectId,
  error,
  isSubmitting,
  onConfirm,
  onCancel,
}: ResolveDefectFormProps) {
  const [sprints, setSprints] = useState<SprintListItem[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [resolvedSprintId, setResolvedSprintId] = useState("");

  // Carga los sprints elegibles al montar; la bandera descarta la respuesta si el formulario se
  // cerró antes de que llegara.
  useEffect(() => {
    let isCurrent = true;
    listDetectionSprints(projectId).then((result) => {
      if (!isCurrent) {
        return;
      }
      if (result.ok) {
        setSprints(result.data.sprints);
      } else {
        setLoadError(result.error.message);
      }
    });
    return () => {
      isCurrent = false;
    };
  }, [projectId]);

  // Confirma la resolución con el sprint elegido.
  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onConfirm(resolvedSprintId);
  }

  return (
    <form
      noValidate
      onSubmit={handleSubmit}
      className="flex flex-col gap-3 rounded-md border border-zinc-300 px-4 py-3 dark:border-zinc-700"
    >
      {loadError && <Alert>{loadError}</Alert>}
      <FormField label="Sprint de resolución" error={error}>
        {(control) => (
          <select
            {...control}
            name="resolvedSprintId"
            value={resolvedSprintId}
            onChange={(event) => setResolvedSprintId(event.target.value)}
            className={`h-11 ${FIELD_CONTROL_CLASS_NAME}`}
          >
            <option value=""></option>
            {sprints.map((sprint) => (
              <option key={sprint.id} value={sprint.id}>
                {sprint.name}
              </option>
            ))}
          </select>
        )}
      </FormField>
      <div className="flex flex-wrap gap-2">
        <button
          type="submit"
          disabled={isSubmitting}
          className={primaryButtonClass}
        >
          Confirmar resolución
        </button>
        <button
          type="button"
          onClick={onCancel}
          disabled={isSubmitting}
          className={secondaryButtonClass}
        >
          Cancelar
        </button>
      </div>
    </form>
  );
}
