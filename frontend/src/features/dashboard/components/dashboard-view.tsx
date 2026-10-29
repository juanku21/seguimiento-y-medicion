"use client";

import { useCallback } from "react";
import { Alert } from "@/components/alert";
import {
  getActiveSprintMetrics,
  getProjectMetrics,
  getProjectSeries,
} from "@/features/dashboard/api";
import { ActiveSprintPanel } from "@/features/dashboard/components/active-sprint-panel";
import { EffortChart } from "@/features/dashboard/components/effort-chart";
import { KeyIndicators } from "@/features/dashboard/components/key-indicators";
import { ProjectSummary } from "@/features/dashboard/components/project-summary";
import { VelocityChart } from "@/features/dashboard/components/velocity-chart";
import { getProjectDetail } from "@/features/projects/api";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";
import type { ProjectDetail } from "@/types/project";

interface DashboardViewProps {
  projectId: string;
}

// Secciones del dashboard que dependen del resumen ya cargado: consulta las métricas del proyecto y
// del sprint activo a la par, cada una con su estado, y monta las secciones en el orden del contrato.
function DashboardSections({ project }: { project: ProjectDetail }) {
  const projectId = project.id;
  const loadMetrics = useCallback(
    () => getProjectMetrics(projectId),
    [projectId],
  );
  const projectMetrics = useAuthorizedQuery(loadMetrics);
  const loadSprintMetrics = useCallback(
    () => getActiveSprintMetrics(projectId),
    [projectId],
  );
  const activeSprintMetrics = useAuthorizedQuery(loadSprintMetrics);
  // Única consulta de series: la comparten todos los gráficos de la vista.
  const loadSeries = useCallback(
    () => getProjectSeries(projectId),
    [projectId],
  );
  const series = useAuthorizedQuery(loadSeries);

  return (
    <div className="flex flex-col gap-8">
      <ProjectSummary project={project} />
      <ActiveSprintPanel activeSprintId={project.activeSprint?.id ?? null} />
      <KeyIndicators
        projectMetrics={projectMetrics}
        activeSprintMetrics={activeSprintMetrics}
      />
      <VelocityChart series={series} />
      <EffortChart series={series} />
    </div>
  );
}

// Vista cliente del dashboard (009/US1 y US2). Consulta el resumen del proyecto con una carga
// estabilizada por el id; solo cuando el resumen llega monta las demás secciones, para que un error
// (por ejemplo un 404) no dispare otras consultas. Más adelante suma los gráficos.
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

  return <DashboardSections project={state.data} />;
}
