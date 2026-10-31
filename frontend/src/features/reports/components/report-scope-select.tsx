"use client";

import { useId } from "react";
import type { SprintListItem } from "@/types/sprint";

// Valor de la opción que representa al proyecto completo.
export const PROJECT_SCOPE = "project";

// Propiedades del selector: los sprints ofrecidos, la opción elegida y el aviso de cambio.
interface ReportScopeSelectProps {
  sprints: SprintListItem[];
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}

// Selector del alcance del reporte: el proyecto completo y los sprints Activo y Cerrados.
export function ReportScopeSelect({
  sprints,
  value,
  onChange,
  disabled = false,
}: ReportScopeSelectProps) {
  const selectId = useId();
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={selectId} className="text-sm font-medium">
        Alcance del reporte
      </label>
      <select
        id={selectId}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
        className="h-11 rounded-md border border-zinc-300 bg-transparent px-3 text-sm dark:border-zinc-700"
      >
        <option value={PROJECT_SCOPE}>Proyecto completo</option>
        {sprints.map((sprint) => (
          <option key={sprint.id} value={sprint.id}>
            {sprint.name} ({sprint.status === "active" ? "Activo" : "Cerrado"})
          </option>
        ))}
      </select>
    </div>
  );
}
