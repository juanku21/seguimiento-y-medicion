"use client";

import type { ReactNode } from "react";
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

// Filtros del esfuerzo del proyecto (006/US4). Firma mínima de T057: la implementación corresponde a
// T062.
export function EffortFilters(props: EffortFiltersProps): ReactNode {
  void props;
  throw new Error("not implemented");
}
