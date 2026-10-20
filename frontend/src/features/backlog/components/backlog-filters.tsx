"use client";

import type { AppliedFilters } from "@/types/story";
import {
  STORY_PRIORITY_LABELS,
  STORY_STATUS_LABELS,
} from "@/utils/story-labels";

// Propiedades de los filtros: los valores elegidos y la acción que recibe la nueva selección.
export type BacklogFiltersProps = {
  value: AppliedFilters;
  onChange: (next: AppliedFilters) => void;
};

// Propiedades de un grupo de casillas: su título, las opciones con su etiqueta en español, los
// valores marcados y la acción que recibe la nueva selección.
type CheckboxGroupProps<T extends string> = {
  legend: string;
  labels: Record<T, string>;
  selected: T[];
  onChange: (next: T[]) => void;
};

// Grupo de casillas de un filtro. Ofrece solo los valores admitidos y entrega la selección en el
// orden de las opciones, agregando o quitando el valor de la casilla tocada (FR-034, FR-037).
function CheckboxGroup<T extends string>({
  legend,
  labels,
  selected,
  onChange,
}: CheckboxGroupProps<T>) {
  const options = Object.keys(labels) as T[];

  // Alterna el valor tocado y conserva el resto de la selección.
  function toggle(option: T) {
    const isSelected = selected.includes(option);
    onChange(
      options.filter((item) =>
        item === option ? !isSelected : selected.includes(item),
      ),
    );
  }

  return (
    <fieldset className="flex flex-wrap items-center gap-x-4 gap-y-2">
      <legend className="mb-1 text-sm font-medium">{legend}</legend>
      {options.map((option) => (
        <label key={option} className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={selected.includes(option)}
            onChange={() => toggle(option)}
            className="h-4 w-4"
          />
          {labels[option]}
        </label>
      ))}
    </fieldset>
  );
}

// Filtros del backlog por estado y por prioridad (003/US2). Componente controlado: muestra los
// filtros recibidos y entrega los dos filtros completos en cada cambio.
export function BacklogFilters({ value, onChange }: BacklogFiltersProps) {
  return (
    <div className="flex flex-col gap-4 sm:flex-row sm:gap-8">
      <CheckboxGroup
        legend="Estado"
        labels={STORY_STATUS_LABELS}
        selected={value.status}
        onChange={(status) => onChange({ ...value, status })}
      />
      <CheckboxGroup
        legend="Prioridad"
        labels={STORY_PRIORITY_LABELS}
        selected={value.priority}
        onChange={(priority) => onChange({ ...value, priority })}
      />
    </div>
  );
}
