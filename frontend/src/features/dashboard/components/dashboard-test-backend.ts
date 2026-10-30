// Ayudante de pruebas compartido del dashboard: lo importan solo los archivos *.test.tsx. Sustituye
// fetch por un backend de prueba que responde las seis operaciones de
// specs/009-project-dashboard/contracts/consumed-api.md y monta la vista dentro de la sesión.
import { createElement } from "react";
import { render } from "@testing-library/react";
import { vi } from "vitest";
import { DashboardView } from "@/features/dashboard/components/dashboard-view";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type {
  ActiveSprintMetricsResponse,
  ProjectMetricsResponse,
  ProjectSeriesResponse,
  SeriesPoint,
} from "@/types/metrics";
import type { ProjectDetail } from "@/types/project";
import type { SprintDetail, SprintListResponse } from "@/types/sprint";

// Respuesta de una operación: el estado HTTP y el cuerpo JSON.
export type BackendReply = { status: number; body: unknown };

// Reemplazos por operación; las que no se indican responden los datos por defecto.
export type DashboardBackendOverrides = {
  project?: BackendReply;
  sprint?: BackendReply;
  sprints?: BackendReply;
  metrics?: BackendReply;
  activeSprintMetrics?: BackendReply;
  series?: BackendReply;
};

// Petición recibida por el backend de prueba.
export type RecordedRequest = { method: string; path: string };

// Token de sesiÃ³n e identificadores por defecto de los datos de prueba.
export const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
export const defaultProjectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
export const defaultSprintId = "3a7b9c1d-2e4f-4a6b-8c0d-1e2f3a4b5c6d";

// Proyecto en curso con 4 integrantes y un sprint activo (US1-1).
export const defaultProject: ProjectDetail = {
  id: defaultProjectId,
  name: "Plataforma de seguimiento",
  description: "Proyecto de ejemplo para el dashboard.",
  status: "in_progress",
  startDate: "2026-09-01",
  plannedEndDate: "2026-12-15",
  actualEndDate: null,
  hoursPerStoryPoint: 6,
  readOnly: false,
  owner: { id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d", fullName: "Ana Pérez" },
  members: [
    {
      userId: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
      fullName: "Ana Pérez",
      isOwner: true,
      joinedAt: "2026-09-01T10:00:00Z",
    },
    {
      userId: "5e6f7a8b-9c0d-4e1f-a2b3-c4d5e6f7a8b9",
      fullName: "Bruno Díaz",
      isOwner: false,
      joinedAt: "2026-09-01T10:05:00Z",
    },
    {
      userId: "7a8b9c0d-1e2f-4a3b-b4c5-d6e7f8a9b0c1",
      fullName: "Carla Gómez",
      isOwner: false,
      joinedAt: "2026-09-02T09:00:00Z",
    },
    {
      userId: "9c0d1e2f-3a4b-4c5d-a6e7-f8a9b0c1d2e3",
      fullName: "Diego Ruiz",
      isOwner: false,
      joinedAt: "2026-09-03T09:00:00Z",
    },
  ],
  hasActiveSprint: true,
  activeSprint: { id: defaultSprintId, name: "Sprint 5" },
  backlog: { countsByState: {}, estimatedStoryPoints: 0, estimatedHours: 0 },
};

// Sprint activo con 10 días restantes (US1-1).
export const defaultSprint: SprintDetail = {
  id: defaultSprintId,
  projectId: defaultProjectId,
  name: "Sprint 5",
  goal: "Entregar el tablero de métricas del proyecto.",
  status: "active",
  startDate: "2026-10-19",
  endDate: "2026-11-08",
  startedAt: "2026-10-19T09:00:00Z",
  closedAt: null,
  createdBy: {
    id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
    fullName: "Ana Pérez",
  },
  closedBy: null,
  createdAt: "2026-10-10T09:00:00Z",
  updatedAt: "2026-10-19T09:00:00Z",
  stories: [],
  results: null,
  hoursPerStoryPoint: 6,
  schedule: { remainingDays: 10, overdueDays: null },
};

// Listado de sprints sin motivo de vacío.
export const defaultSprints: SprintListResponse = {
  sprints: [],
  appliedFilters: { status: [] },
  emptyReason: null,
};

// Métricas del proyecto: velocidad 14,00 (no es el promedio de la serie), 7 defectos abiertos
// repartidos en 2/3/1/1 y un acumulado parcial.
export const defaultMetrics: ProjectMetricsResponse = {
  velocity: {
    value: { status: "available", value: 14 },
  },
  cumulative: {
    finality: "partial",
    plannedStoryPoints: { status: "available", value: 83 },
    completedStoryPoints: { status: "available", value: 62 },
    estimatedHours: { status: "available", value: 498 },
    actualHours: { status: "available", value: 410 },
    deviationHours: { status: "available", value: -88 },
    deviationPercent: { status: "available", value: -17.67 },
    storyCompletionPercent: { status: "available", value: 74.7 },
  },
  defects: {
    open: {
      total: 7,
      bySeverity: { critical: 2, high: 3, medium: 1, low: 1 },
    },
  },
  warnings: [],
};

// Métricas del sprint activo, todas parciales (US2-1).
export const defaultActiveSprintMetrics: ActiveSprintMetricsResponse = {
  activeSprint: true,
  metrics: {
    plannedStoryPoints: metric(18),
    completedStoryPoints: metric(5),
    estimatedHours: metric(108),
    actualHours: metric(40),
    deviationHours: metric(-68),
    deviationPercent: metric(-62.96),
    storyCompletionPercent: metric(27.78),
  },
};

// Valor disponible y parcial de un sprint en curso.
function metric(value: number) {
  return { status: "available" as const, value, finality: "partial" as const };
}

// Punto de una serie para un sprint Cerrado.
function point(index: number, sprintName: string, value: number): SeriesPoint {
  return {
    sprintId: `b${index}000000-0000-4000-8000-00000000000${index}`,
    sprintName,
    sprintStatus: "closed",
    status: "available",
    value,
    finality: "definitive",
  };
}

// Nombres de los sprints Cerrados que usan las series por defecto.
const closedSprintNames = ["Sprint 1", "Sprint 2", "Sprint 3", "Sprint 4"];

// Arma una serie con un valor por cada sprint Cerrado.
function series(values: number[]): SeriesPoint[] {
  return values.map((value, index) =>
    point(index + 1, closedSprintNames[index], value),
  );
}

// Series de 4 sprints Cerrados: Story Points planificados 21/24/13/25 y completados 13/21/8/20 (US3-1).
export const defaultSeries: ProjectSeriesResponse = {
  plannedStoryPoints: series([21, 24, 13, 25]),
  completedStoryPoints: series([13, 21, 8, 20]),
  estimatedHours: series([126, 144, 78, 150]),
  actualHours: series([110, 150, 70, 140]),
  defectsDetected: series([3, 2, 4, 1]),
  defectsResolved: series([2, 2, 3, 1]),
};

// Instala el backend de prueba en fetch y devuelve la lista de peticiones recibidas, que se va
// llenando a medida que la vista consulta.
export function installDashboardBackend(
  overrides: DashboardBackendOverrides = {},
): RecordedRequest[] {
  const requests: RecordedRequest[] = [];
  const base = `/api/v1/projects/${defaultProjectId}`;

  // Respuestas por método y ruta, con el reemplazo de la prueba cuando existe.
  const replies: Record<string, BackendReply> = {
    [`GET ${base}`]: overrides.project ?? { status: 200, body: defaultProject },
    [`GET /api/v1/sprints/${defaultSprintId}`]: overrides.sprint ?? {
      status: 200,
      body: defaultSprint,
    },
    [`GET ${base}/sprints`]: overrides.sprints ?? {
      status: 200,
      body: defaultSprints,
    },
    [`GET ${base}/metrics`]: overrides.metrics ?? {
      status: 200,
      body: defaultMetrics,
    },
    [`GET ${base}/metrics/active-sprint`]: overrides.activeSprintMetrics ?? {
      status: 200,
      body: defaultActiveSprintMetrics,
    },
    [`GET ${base}/metrics/series`]: overrides.series ?? {
      status: 200,
      body: defaultSeries,
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

// Instala un ResizeObserver mínimo que informa 800 x 320, para que ResponsiveContainer de Recharts
// dibuje el SVG en jsdom (research.md, sección 2).
export function installResizeObserverStub() {
  class ResizeObserverStub {
    private readonly callback: ResizeObserverCallback;

    constructor(callback: ResizeObserverCallback) {
      this.callback = callback;
    }

    observe(target: Element) {
      const entry = {
        target,
        contentRect: { width: 800, height: 320 },
      } as ResizeObserverEntry;
      this.callback([entry], this as unknown as ResizeObserver);
    }

    unobserve() {}

    disconnect() {}
  }
  vi.stubGlobal("ResizeObserver", ResizeObserverStub);
}

// Guarda la sesión de la pestaña y monta el dashboard del proyecto dentro del proveedor de sesión,
// como en el layout raíz.
export function renderDashboard(projectId: string) {
  writeToken(sessionToken);
  return render(
    createElement(
      SessionProvider,
      null,
      createElement(DashboardView, { projectId }),
    ),
  );
}
