// Ayudante de pruebas compartido de los reportes: lo importan solo los archivos *.test.tsx. Sustituye
// fetch por un backend de prueba que responde las operaciones de
// specs/010-project-reports/contracts/openapi.yaml y monta la vista dentro de la sesión. El
// vi.mock de next/navigation y el de export-pdf los declara cada archivo de prueba.
import { createElement } from "react";
import { render } from "@testing-library/react";
import { vi } from "vitest";
import { ReportsView } from "@/features/reports/components/reports-view";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ProjectMetricsResponse, SprintMetric } from "@/types/metrics";
import type { Report, ReportSprint } from "@/types/report";
import type { SprintListItem, SprintListResponse } from "@/types/sprint";

// Respuesta de una operación: el estado HTTP y el cuerpo JSON.
export type BackendReply = { status: number; body: unknown };

// Reemplazos por operación; las que no se indican responden los datos por defecto.
export type ReportsBackendOverrides = {
  sprints?: BackendReply;
  sprintReport?: BackendReply;
  projectReport?: BackendReply;
};

// Petición recibida por el backend de prueba.
export type RecordedRequest = { method: string; path: string };

// Token de sesión e identificadores por defecto de los datos de prueba.
export const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
export const defaultProjectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
export const defaultSprintId = "3a7b9c1d-2e4f-4a6b-8c0d-1e2f3a4b5c6d";

// Integrantes del proyecto de prueba.
const anaId = "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d";
const brunoId = "5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9";

// Sprint Cerrado del reporte de sprint (US1-1).
export const defaultReportSprint: ReportSprint = {
  id: defaultSprintId,
  name: "Sprint 1",
  status: "closed",
  startDate: "2026-09-07",
  endDate: "2026-09-27",
  closedAt: "2026-09-27T18:00:00Z",
};

// Los 3 sprints Cerrados del reporte de proyecto (US3-1).
const closedSprints: ReportSprint[] = [
  defaultReportSprint,
  {
    id: "b2000000-0000-4000-8000-000000000002",
    name: "Sprint 2",
    status: "closed",
    startDate: "2026-09-28",
    endDate: "2026-10-18",
    closedAt: "2026-10-18T18:00:00Z",
  },
  {
    id: "b3000000-0000-4000-8000-000000000003",
    name: "Sprint 3",
    status: "closed",
    startDate: "2026-10-19",
    endDate: "2026-11-08",
    closedAt: "2026-11-08T18:00:00Z",
  },
];

// Valor disponible de una métrica de sprint con la finalidad indicada.
function metric(
  value: number,
  finality: SprintMetric["finality"] = "definitive",
): SprintMetric {
  return { status: "available", value, finality };
}

// Encabezado común a ambos alcances; el sprint y el período dependen del alcance.
function header(scope: "sprint" | "project"): Report["header"] {
  return {
    projectId: defaultProjectId,
    projectName: "Plataforma de seguimiento",
    projectStatus: "in_progress",
    sprint: scope === "sprint" ? defaultReportSprint : null,
    period:
      scope === "sprint"
        ? { startDate: "2026-09-07", endDate: "2026-09-27" }
        : { startDate: "2026-09-07", endDate: "2026-11-08" },
    generatedAt: "2026-11-10T14:30:00Z",
    generatedBy: { id: anaId, fullName: "Ana Pérez" },
  };
}

// Defectos del sprint: uno de cada severidad, salvo la baja (US1-1).
const defects = [
  {
    defectId: "d1000000-0000-4000-8000-000000000001",
    description: "El tablero no carga con el filtro vacío.",
    severity: "critical" as const,
    status: "resolved" as const,
    story: { id: "a1000000-0000-4000-8000-000000000001", title: "Tablero" },
    detectedSprint: { id: defaultSprintId, name: "Sprint 1" },
    resolvedSprint: { id: defaultSprintId, name: "Sprint 1" },
  },
  {
    defectId: "d2000000-0000-4000-8000-000000000002",
    description: "La exportación pierde los acentos.",
    severity: "medium" as const,
    status: "open" as const,
    story: { id: "a2000000-0000-4000-8000-000000000002", title: "Exportación" },
    detectedSprint: { id: defaultSprintId, name: "Sprint 1" },
    resolvedSprint: null,
  },
  {
    defectId: "d3000000-0000-4000-8000-000000000003",
    description: "El botón de ayuda queda desalineado.",
    severity: "low" as const,
    status: "in_progress" as const,
    story: { id: "a3000000-0000-4000-8000-000000000003", title: "Ayuda" },
    detectedSprint: { id: defaultSprintId, name: "Sprint 1" },
    resolvedSprint: null,
  },
];

// Las 4 historias planificadas del sprint (US1-1).
const storyLines = [
  {
    storyId: "a1000000-0000-4000-8000-000000000001",
    title: "Tablero",
    priority: "high" as const,
    storyPoints: 8,
    status: "completed" as const,
  },
  {
    storyId: "a2000000-0000-4000-8000-000000000002",
    title: "Exportación",
    priority: "high" as const,
    storyPoints: 5,
    status: "completed" as const,
  },
  {
    storyId: "a3000000-0000-4000-8000-000000000003",
    title: "Ayuda",
    priority: "medium" as const,
    storyPoints: 3,
    status: "not_completed" as const,
  },
  {
    storyId: "a4000000-0000-4000-8000-000000000004",
    title: "Perfil",
    priority: "low" as const,
    storyPoints: null,
    status: "not_completed" as const,
  },
];

// Reporte de sprint completo y válido (US1-1): 4 historias, 3 defectos con distinta severidad,
// esfuerzo de 2 integrantes y métricas del sprint Cerrado con finalidad definitiva y provisoria.
export function sprintReport(overrides: Partial<Report> = {}): Report {
  return {
    scope: "sprint",
    partial: false,
    header: header("sprint"),
    stories: {
      state: "complete",
      groups: [
        { sprint: defaultReportSprint, state: "complete", stories: storyLines },
      ],
    },
    estimations: {
      state: "complete",
      groups: [
        {
          sprint: defaultReportSprint,
          state: "complete",
          items: storyLines.map((story) => ({
            storyId: story.storyId,
            title: story.title,
            storyPoints: story.storyPoints,
            poker: "none" as const,
            session: null,
          })),
        },
      ],
    },
    effort: {
      state: "complete",
      groups: [
        {
          sprint: defaultReportSprint,
          state: "complete",
          stories: [
            { storyId: storyLines[0].storyId, title: "Tablero", hours: 30 },
            { storyId: storyLines[1].storyId, title: "Exportación", hours: 20 },
          ],
          totalHours: 50,
        },
      ],
      byMember: [
        { userId: anaId, fullName: "Ana Pérez", hours: 32 },
        { userId: brunoId, fullName: "Bruno Díaz", hours: 18 },
      ],
      totalHours: 50,
    },
    metrics: {
      state: "complete",
      sprint: {
        sprint: {
          id: defaultSprintId,
          name: "Sprint 1",
          status: "closed",
          startDate: "2026-09-07",
          endDate: "2026-09-27",
          closedAt: "2026-09-27T18:00:00Z",
        },
        plannedStoryPoints: metric(16),
        completedStoryPoints: metric(13),
        estimatedHours: metric(96),
        actualHours: metric(50),
        deviationHours: metric(-46),
        deviationPercent: metric(-47.92),
        storyCompletionPercent: metric(50),
        defectsDetected: metric(3),
        defectsResolved: metric(1, "provisional"),
        warnings: [],
      },
      project: null,
    },
    defects: {
      state: "complete",
      defects,
      totals: { detected: 3, resolved: 1 },
    },
    ...overrides,
  };
}

// Métricas del proyecto para el reporte consolidado de 3 sprints Cerrados.
const projectMetrics: ProjectMetricsResponse = {
  velocity: { value: { status: "available", value: 14 }, sprintsConsidered: 3 },
  cumulative: {
    finality: "definitive",
    plannedStoryPoints: { status: "available", value: 48 },
    completedStoryPoints: { status: "available", value: 39 },
    estimatedHours: { status: "available", value: 288 },
    actualHours: { status: "available", value: 150 },
    deviationHours: { status: "available", value: -138 },
    deviationPercent: { status: "available", value: -47.92 },
    storyCompletionPercent: { status: "available", value: 81.25 },
  },
  effortOutsideSprintsHours: 0,
  effortInPlannedSprintsHours: 0,
  unestimatedBacklogStories: 0,
  defects: {
    detected: {
      total: 3,
      bySeverity: { critical: 1, high: 0, medium: 1, low: 1 },
    },
    resolved: {
      total: 1,
      bySeverity: { critical: 1, high: 0, medium: 0, low: 0 },
    },
    open: {
      total: 2,
      bySeverity: { critical: 0, high: 0, medium: 1, low: 1 },
    },
  },
  warnings: [],
};

// Reporte de proyecto completo y válido (US3-1): 3 sprints Cerrados, con las historias del
// sprint de prueba agrupadas en el primero.
export function projectReport(overrides: Partial<Report> = {}): Report {
  const base = sprintReport();
  return {
    ...base,
    scope: "project",
    header: header("project"),
    stories: {
      state: "complete",
      groups: closedSprints.map((sprint, index) => ({
        sprint,
        state: index === 0 ? ("complete" as const) : ("empty" as const),
        stories: index === 0 ? storyLines : [],
      })),
    },
    estimations: {
      state: "complete",
      groups: closedSprints.map((sprint, index) => ({
        sprint,
        state: index === 0 ? ("complete" as const) : ("empty" as const),
        items: index === 0 ? (base.estimations.groups?.[0].items ?? []) : [],
      })),
    },
    effort: {
      ...base.effort,
      groups: closedSprints.map((sprint, index) => ({
        sprint,
        state: index === 0 ? ("complete" as const) : ("empty" as const),
        stories: index === 0 ? (base.effort.groups?.[0].stories ?? []) : [],
        totalHours: index === 0 ? 50 : 0,
      })),
    },
    metrics: { state: "complete", sprint: null, project: projectMetrics },
    ...overrides,
  };
}

// Listado de sprints del proyecto: los 3 Cerrados, sin motivo de vacío.
const defaultSprints: SprintListResponse = {
  sprints: closedSprints.map((sprint): SprintListItem => ({
    id: sprint.id,
    name: sprint.name,
    goal: "Meta del sprint.",
    status: sprint.status,
    startDate: sprint.startDate,
    endDate: sprint.endDate,
    startedAt: `${sprint.startDate}T09:00:00Z`,
    closedAt: sprint.closedAt,
    storyCount: 4,
    results: null,
  })),
  appliedFilters: { status: [] },
  emptyReason: null,
};

// Instala el backend de prueba en fetch y devuelve la lista de peticiones recibidas, que se va
// llenando a medida que la vista consulta.
export function installReportsBackend(
  overrides: ReportsBackendOverrides = {},
): RecordedRequest[] {
  const requests: RecordedRequest[] = [];
  const base = `/api/v1/projects/${defaultProjectId}`;

  // Respuestas por método y ruta, con el reemplazo de la prueba cuando existe.
  const replies: Record<string, BackendReply> = {
    [`GET ${base}/sprints`]: overrides.sprints ?? {
      status: 200,
      body: defaultSprints,
    },
    [`POST ${base}/reports/sprints/${defaultSprintId}`]:
      overrides.sprintReport ?? { status: 200, body: sprintReport() },
    [`POST ${base}/reports/project`]: overrides.projectReport ?? {
      status: 200,
      body: projectReport(),
    },
  };

  const fetchMock = vi.fn<typeof fetch>(async (input, init) => {
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    const method = (init?.method ?? "GET").toUpperCase();
    const path = new URL(raw, "http://localhost").pathname;
    requests.push({ method, path });

    // Cualquier otro proyecto o ruta responde como un proyecto inexistente.
    const reply = replies[`${method} ${path}`] ?? {
      status: 404,
      body: { error: "El proyecto no existe." },
    };
    return new Response(JSON.stringify(reply.body), {
      status: reply.status,
      headers: { "Content-Type": "application/json" },
    });
  });
  vi.stubGlobal("fetch", fetchMock);
  return requests;
}

// Guarda la sesión de la pestaña y monta la vista de reportes dentro del proveedor de sesión,
// como en el layout raíz.
export function renderReports(projectId: string) {
  writeToken(sessionToken);
  return render(
    createElement(
      SessionProvider,
      null,
      createElement(ReportsView, { projectId }),
    ),
  );
}
