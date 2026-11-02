"use client";

import { useState } from "react";
import { BacklogFilters } from "@/features/backlog/components/backlog-filters";
import { StoryList } from "@/features/backlog/components/story-list";
import type { AppliedFilters } from "@/types/story";

// Propiedades de la vista: el proyecto cuyo backlog se muestra.
export type BacklogViewProps = {
  projectId: string;
};

// Vista del backlog (003/US2). Guarda los filtros elegidos en un estado, para que el listado vuelva
// a consultar solo cuando cambia la selección, y se los pasa a los filtros y al listado. Arranca sin
// filtros.
export function BacklogView({ projectId }: BacklogViewProps) {
  const [filters, setFilters] = useState<AppliedFilters>({
    status: [],
    priority: [],
  });

  return (
    <div className="flex flex-col gap-6">
      <BacklogFilters value={filters} onChange={setFilters} />
      <StoryList projectId={projectId} filters={filters} />
    </div>
  );
}
