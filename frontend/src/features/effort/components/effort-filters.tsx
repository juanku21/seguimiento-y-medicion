"use client";

import type { ReactNode } from "react";
import { FIELD_CONTROL_CLASS_NAME, FormField } from "@/components/form-field";
import { Input } from "@/components/input";
import type { ProjectEffortFilter, StoryRef, UserRef } from "@/types/effort";

// Sprint que se ofrece como opción del filtro por sprint.
export type SprintOption = {
  id: string;
  name: string;
};

// Propiedades de los filtros del esfuerzo del proyecto: los filtros elegidos, la acción que recibe
// la nueva selección y las opciones de integrante, historia y sprint.
export type EffortFiltersProps = {
  value: ProjectEffortFilter;
  onChange: (next: ProjectEffortFilter) => void;
  members: UserRef[];
  stories: StoryRef[];
  sprints: SprintOption[];
};

// Opción de un selector de filtro: el valor que se entrega y el texto que se muestra.
type FilterOption = {
  value: string;
  label: string;
};

// Propiedades de un selector de filtro: su etiqueta, el valor elegido, las opciones (la primera es
// la de no filtrar) y la acción que recibe el valor nuevo.
type FilterSelectProps = {
  label: string;
  value: string;
  options: FilterOption[];
  onChange: (next: string) => void;
};

// Selector de un filtro con su etiqueta asociada, para que se lo encuentre por su nombre accesible.
function FilterSelect({ label, value, options, onChange }: FilterSelectProps) {
  return (
    <FormField label={label}>
      {(control) => (
        <select
          {...control}
          className={`h-11 ${FIELD_CONTROL_CLASS_NAME}`}
          value={value}
          onChange={(event) => onChange(event.target.value)}
        >
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

// Filtros del esfuerzo del proyecto (006/US4). Componente controlado: muestra los filtros recibidos
// y entrega los cinco filtros completos en cada cambio, conservando los que no cambiaron, porque se
// combinan (FR-037). Cada selector empieza con la opción de no filtrar (valor "") y el de sprint
// ofrece además "Sin sprint" (valor "none", FR-039).
export function EffortFilters({
  value,
  onChange,
  members,
  stories,
  sprints,
}: EffortFiltersProps): ReactNode {
  // Entrega los filtros vigentes con un único filtro reemplazado.
  function update(key: keyof ProjectEffortFilter, next: string) {
    onChange({ ...value, [key]: next });
  }

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <FilterSelect
        label="Integrante"
        value={value.userId}
        options={[
          { value: "", label: "Todos" },
          ...members.map((member) => ({
            value: member.id,
            label: member.fullName,
          })),
        ]}
        onChange={(next) => update("userId", next)}
      />
      <FilterSelect
        label="Historia"
        value={value.storyId}
        options={[
          { value: "", label: "Todas" },
          ...stories.map((story) => ({ value: story.id, label: story.title })),
        ]}
        onChange={(next) => update("storyId", next)}
      />
      <FilterSelect
        label="Sprint"
        value={value.sprintId}
        options={[
          { value: "", label: "Todos" },
          { value: "none", label: "Sin sprint" },
          ...sprints.map((sprint) => ({
            value: sprint.id,
            label: sprint.name,
          })),
        ]}
        onChange={(next) => update("sprintId", next)}
      />
      <Input
        label="Desde"
        name="from"
        type="date"
        value={value.from}
        onChange={(event) => update("from", event.target.value)}
      />
      <Input
        label="Hasta"
        name="to"
        type="date"
        value={value.to}
        onChange={(event) => update("to", event.target.value)}
      />
    </div>
  );
}
