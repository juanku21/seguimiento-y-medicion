"use client";

import { useCallback, type ReactElement } from "react";
import { FIELD_CONTROL_CLASS_NAME, FormField } from "@/components/form-field";
import { listStories } from "@/features/backlog/api";
import { listSprints } from "@/features/sprints/api";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";
import type { ApiResult } from "@/lib/api-client";
import type { DefectFilterValues } from "@/types/defect";
import {
  DEFECT_SEVERITY_LABELS,
  DEFECT_STATUS_LABELS,
} from "@/utils/defect-labels";

// Propiedades de los filtros: el proyecto, los filtros elegidos y la acción que recibe la nueva
// selección completa.
export type DefectFiltersProps = {
  projectId: string;
  value: DefectFilterValues;
  onChange: (next: DefectFilterValues) => void;
};

// Opción de un selector: el identificador que se entrega y el texto que se muestra.
type FilterOption = {
  id: string;
  label: string;
};

// Opciones de los selectores de historia y de sprint del proyecto.
type FilterChoices = {
  stories: FilterOption[];
  sprints: FilterOption[];
};

// Pide las historias y los sprints del proyecto para armar los selectores; si alguna consulta
// falla, devuelve su error.
async function loadFilterChoices(
  projectId: string,
): Promise<ApiResult<FilterChoices>> {
  const [stories, sprints] = await Promise.all([
    listStories(projectId, { status: [], priority: [] }),
    listSprints(projectId),
  ]);
  if (!stories.ok) {
    return stories;
  }
  if (!sprints.ok) {
    return sprints;
  }
  return {
    ok: true,
    data: {
      stories: stories.data.stories.map((story) => ({
        id: story.id,
        label: story.title,
      })),
      sprints: sprints.data.sprints.map((sprint) => ({
        id: sprint.id,
        label: sprint.name,
      })),
    },
  };
}

// Propiedades de un grupo de casillas: su título, las opciones con su etiqueta en español, los
// valores marcados y la acción que recibe la nueva selección.
type CheckboxGroupProps<T extends string> = {
  legend: string;
  labels: Record<T, string>;
  selected: T[];
  onChange: (next: T[]) => void;
};

// Grupo de casillas de un filtro. Ofrece solo los valores admitidos y entrega la selección en el
// orden de las opciones, agregando o quitando el valor de la casilla tocada (FR-040, FR-046).
function CheckboxGroup<T extends string>({
  legend,
  labels,
  selected,
  onChange,
}: CheckboxGroupProps<T>): ReactElement {
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

// Propiedades de un selector de filtro: su etiqueta, el valor elegido, las opciones y la acción
// que recibe el valor nuevo; el valor vacío equivale a no filtrar.
type FilterSelectProps = {
  label: string;
  value: string;
  options: FilterOption[];
  onChange: (next: string) => void;
};

// Selector de un filtro con su etiqueta asociada y una primera opción para no filtrar.
function FilterSelect({
  label,
  value,
  options,
  onChange,
}: FilterSelectProps): ReactElement {
  return (
    <FormField label={label}>
      {(control) => (
        <select
          {...control}
          className={`h-11 ${FIELD_CONTROL_CLASS_NAME}`}
          value={value}
          onChange={(event) => onChange(event.target.value)}
        >
          <option value="">Todos</option>
          {options.map((option) => (
            <option key={option.id} value={option.id}>
              {option.label}
            </option>
          ))}
        </select>
      )}
    </FormField>
  );
}

// Filtros del listado de defectos (007/US3). Componente controlado: muestra los filtros recibidos y
// entrega los cuatro filtros completos en cada cambio, porque se combinan (FR-040 a FR-042). Estado
// y severidad admiten varios valores; historia y sprint, uno, elegido entre los del proyecto.
export function DefectFilters({
  projectId,
  value,
  onChange,
}: DefectFiltersProps): ReactElement {
  const loadChoices = useCallback(
    () => loadFilterChoices(projectId),
    [projectId],
  );
  const choices = useAuthorizedQuery(loadChoices);
  const options: FilterChoices =
    choices.status === "loaded" ? choices.data : { stories: [], sprints: [] };

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-4 sm:flex-row sm:gap-8">
        <CheckboxGroup
          legend="Estado"
          labels={DEFECT_STATUS_LABELS}
          selected={value.status}
          onChange={(status) => onChange({ ...value, status })}
        />
        <CheckboxGroup
          legend="Severidad"
          labels={DEFECT_SEVERITY_LABELS}
          selected={value.severity}
          onChange={(severity) => onChange({ ...value, severity })}
        />
      </div>
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FilterSelect
          label="Historia"
          value={value.storyId}
          options={options.stories}
          onChange={(storyId) => onChange({ ...value, storyId })}
        />
        <FilterSelect
          label="Sprint"
          value={value.sprintId}
          options={options.sprints}
          onChange={(sprintId) => onChange({ ...value, sprintId })}
        />
      </div>
    </div>
  );
}
