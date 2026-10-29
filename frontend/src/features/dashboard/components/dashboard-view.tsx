"use client";

import { useCallback } from "react";
import { Alert } from "@/components/alert";
import { ActiveSprintPanel } from "@/features/dashboard/components/active-sprint-panel";
import { ProjectSummary } from "@/features/dashboard/components/project-summary";
import { getProjectDetail } from "@/features/projects/api";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";

interface DashboardViewProps {
  projectId: string;
}

// Vista cliente del dashboard (009/US1). Consulta el resumen del proyecto con una carga estabilizada
// por el id; solo cuando el resumen llega monta las demás secciones, para que un error (por ejemplo
// un 404) no dispare otras consultas. Más adelante suma las métricas y los gráficos.
export function DashboardView({ projectId }: DashboardViewProps) {
  const load = useCallback(() => getProjectDetail(projectId), [projectId]);
  const state = useAuthorizedQuery(load);

  if (state.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando dashboard…
      </p>
    );
  }
  if (state.status === "error") {
    return <Alert>{state.message}</Alert>;
  }

  // Secciones en el orden del contrato: resumen del proyecto y sprint activo.
  const project = state.data;
  return (
    <div className="flex flex-col gap-8">
      <ProjectSummary project={project} />
      <ActiveSprintPanel activeSprintId={project.activeSprint?.id ?? null} />
    </div>
  );
}
