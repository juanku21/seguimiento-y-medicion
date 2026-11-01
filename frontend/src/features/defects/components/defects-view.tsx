"use client";

import { useState, type ReactElement } from "react";
import { DefectFilters } from "@/features/defects/components/defect-filters";
import { DefectList } from "@/features/defects/components/defect-list";
import type { DefectFilterValues } from "@/types/defect";

// Propiedades de la vista de defectos: el proyecto tomado de la ruta.
export type DefectsViewProps = {
  projectId: string;
};

// Filtros iniciales: ninguno aplicado.
const NO_FILTERS: DefectFilterValues = {
  status: [],
  severity: [],
  storyId: "",
  sprintId: "",
};

// Vista de los defectos del proyecto (007/US3): conserva los filtros elegidos y los comparte entre
// los controles de filtro y el listado.
export function DefectsView({ projectId }: DefectsViewProps): ReactElement {
  const [filters, setFilters] = useState<DefectFilterValues>(NO_FILTERS);

  return (
    <div className="flex flex-col gap-6">
      <DefectFilters
        projectId={projectId}
        value={filters}
        onChange={setFilters}
      />
      <DefectList projectId={projectId} filters={filters} />
    </div>
  );
}
