"use client";

import { LoadingState } from "@/components/loading-state";
import { useCallback, useId, type ReactNode } from "react";
import { Alert } from "@/components/alert";
import { getProjectEffortComparison } from "@/features/effort/api";
import { EffortComparisonTable } from "@/features/effort/components/effort-comparison-table";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";

// Propiedades de la comparación del proyecto: el proyecto tomado de la ruta.
export type ProjectEffortComparisonSectionProps = {
  projectId: string;
};

// Sección de la página de esfuerzo del proyecto con la comparación entre estimado y real de todas
// sus historias (006/US5). Pide la comparación con la sesión vigente y la muestra en la tabla; el
// error de la consulta se informa con su mensaje.
export function ProjectEffortComparisonSection({
  projectId,
}: ProjectEffortComparisonSectionProps): ReactNode {
  const headingId = useId();
  const load = useCallback(
    () => getProjectEffortComparison(projectId),
    [projectId],
  );
  const comparison = useAuthorizedQuery(load);

  return (
    <section
      aria-labelledby={headingId}
      className="flex min-w-0 flex-col gap-4 rounded-lg border border-border bg-surface p-5"
    >
      <h2 id={headingId} className="text-lg font-semibold tracking-tight">
        Estimado contra real
      </h2>
      {comparison.status === "loading" && (
        <LoadingState>Cargando comparación…</LoadingState>
      )}
      {comparison.status === "error" && <Alert>{comparison.message}</Alert>}
      {comparison.status === "loaded" && (
        <EffortComparisonTable stories={comparison.data.stories} />
      )}
    </section>
  );
}
