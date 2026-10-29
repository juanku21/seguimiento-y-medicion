"use client";

import { useCallback, useState } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import {
  getActiveSprintMetrics,
  getProjectMetrics,
  getProjectSeries,
} from "@/features/dashboard/api";
import { ActiveSprintPanel } from "@/features/dashboard/components/active-sprint-panel";
import { DashboardEmptyState } from "@/features/dashboard/components/dashboard-empty-state";
import { DashboardWarnings } from "@/features/dashboard/components/dashboard-warnings";
import { EffortChart } from "@/features/dashboard/components/effort-chart";
import { KeyIndicators } from "@/features/dashboard/components/key-indicators";
import { OpenDefectsDistribution } from "@/features/dashboard/components/open-defects-distribution";
import { ProjectSummary } from "@/features/dashboard/components/project-summary";
import { QualityChart } from "@/features/dashboard/components/quality-chart";
import {
  SectionError,
  WARNINGS_ERROR,
} from "@/features/dashboard/components/section-error";
import { VelocityChart } from "@/features/dashboard/components/velocity-chart";
import { getProjectDetail } from "@/features/projects/api";
import { listSprints } from "@/features/sprints/api";
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
  // Listado de sprints sin filtros: solo se usa para saber si el proyecto no tiene ninguno.
  const loadSprints = useCallback(() => listSprints(projectId), [projectId]);
  const sprints = useAuthorizedQuery(loadSprints);
  const hasNoSprints =
    sprints.status === "loaded" && sprints.data.emptyReason === "no_sprints";

  return (
    <div className="flex flex-col gap-8">
      <ProjectSummary project={project} />
      <ActiveSprintPanel activeSprintId={project.activeSprint?.id ?? null} />
      {projectMetrics.status === "loaded" && (
        <DashboardWarnings warnings={projectMetrics.data.warnings} />
      )}
      {projectMetrics.status === "error" && (
        <SectionError message={WARNINGS_ERROR} />
      )}
      {sprints.status === "loading" && (
        <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
          Cargando métricas…
        </p>
      )}
      {hasNoSprints && <DashboardEmptyState />}
      {sprints.status !== "loading" && !hasNoSprints && (
        // Con sprints, o si el listado falló, se muestran las secciones de métricas.
        <>
          <KeyIndicators
            projectMetrics={projectMetrics}
            activeSprintMetrics={activeSprintMetrics}
          />
          <VelocityChart series={series} />
          <EffortChart series={series} />
          <QualityChart series={series} />
          <OpenDefectsDistribution projectMetrics={projectMetrics} />
        </>
      )}
    </div>
  );
}

// Vista cliente del dashboard (009/US1 y US2). Consulta el resumen del proyecto con una carga
// estabilizada por el id; solo cuando el resumen llega monta las demás secciones, para que un error
// (por ejemplo un 404) no dispare otras consultas. Más adelante suma los gráficos.
export function DashboardView({ projectId }: DashboardViewProps) {
  // El contador fuerza una nueva carga del resumen al reintentar.
  const [attempt, setAttempt] = useState(0);
  const load = useCallback(() => {
    void attempt;
    return getProjectDetail(projectId);
  }, [projectId, attempt]);
  const state = useAuthorizedQuery(load);

  if (state.status === "loading") {
    return (
      <p role="status" className="text-sm text-zinc-600 dark:text-zinc-400">
        Cargando dashboard…
      </p>
    );
  }
  if (state.status === "error") {
    return (
      <div className="flex flex-col gap-4">
        <Alert>
          <p>No se pudo mostrar el dashboard.</p>
          <p>{state.message}</p>
        </Alert>
        {state.httpStatus !== 404 && (
          // Un 404 no se resuelve reintentando: solo se ofrece en los demás errores.
          <Button onClick={() => setAttempt((n) => n + 1)}>Reintentar</Button>
        )}
      </div>
    );
  }

  return <DashboardSections project={state.data} />;
}
