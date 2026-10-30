import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import {
  defaultMetrics,
  defaultProject,
  defaultProjectId,
  defaultSeries,
  defaultSprint,
  defaultSprints,
  installDashboardBackend,
  installResizeObserverStub,
  renderDashboard,
  type BackendReply,
  type RecordedRequest,
} from "@/features/dashboard/components/dashboard-test-backend";
import { clearSession } from "@/lib/session-storage";
import type {
  MetricsWarning,
  ProjectMetricsResponse,
  ProjectSeriesResponse,
  SeriesPoint,
} from "@/types/metrics";
import type { ProjectDetail } from "@/types/project";

// Pruebas de escenario de la Historia 6 (T033): montan DashboardView completo con el backend de
// prueba compartido y verifican el estado vacío, las advertencias, el aislamiento de fallas por
// sección y el reintento del resumen. Al final, pruebas transversales sin etiqueta de historia.

// Espía compartido del enrutador: la guardia de sesión llama a replace sin token.
const routerMock = vi.hoisted(() => ({
  replace: vi.fn(),
  push: vi.fn(),
  back: vi.fn(),
  forward: vi.fn(),
  refresh: vi.fn(),
  prefetch: vi.fn(),
}));

// Sustituye el enrutador de Next.js: la sesión lo usa fuera del App Router.
vi.mock("next/navigation", () => ({
  useRouter: () => routerMock,
  usePathname: () => "/",
  useSearchParams: () => new URLSearchParams(),
}));

// Textos esperados de las secciones, de los errores y de las figuras del dashboard.
const EMPTY_TITLE = "Sin datos todavía";
const EMPTY_TEXT =
  "Este proyecto todavía no tiene sprints, así que no hay métricas que mostrar. Planificá el primer sprint desde la sección Sprints del proyecto.";
const SUMMARY = "Resumen del proyecto";
const INDICATORS = "Indicadores clave";
const DISTRIBUTION = "Defectos abiertos por severidad";
const SUMMARY_ERROR = "No se pudo mostrar el dashboard.";
const ACTIVE_SPRINT_ERROR = "No se pudo cargar el sprint activo.";
const SPRINT_INDICATORS_ERROR =
  "No se pudieron cargar los indicadores del sprint activo.";
const PROJECT_INDICATORS_ERROR =
  "No se pudieron cargar los indicadores del proyecto.";
const VELOCITY_ERROR = "No se pudo cargar el gráfico de velocidad.";
const EFFORT_ERROR = "No se pudo cargar el gráfico de esfuerzo.";
const QUALITY_ERROR = "No se pudo cargar el gráfico de defectos.";
const DISTRIBUTION_ERROR =
  "No se pudo cargar la distribución de defectos abiertos.";
const WARNINGS_ERROR = "No se pudieron cargar las advertencias.";
const FIGURES = [
  "Velocidad por sprint",
  "Esfuerzo por sprint",
  "Defectos por sprint",
];

// Respuesta de error interno del servidor.
const serverError: BackendReply = {
  status: 500,
  body: { error: "Error interno." },
};
// Listado de sprints vacÃ­o porque el proyecto no tiene sprints.
const noSprints = {
  ...defaultSprints,
  emptyReason: "no_sprints" as const,
};
// Proyecto en curso sin sprint activo.
const projectWithoutSprint: ProjectDetail = {
  ...defaultProject,
  hasActiveSprint: false,
  activeSprint: null,
};
// Advertencia de planificaciÃ³n de ejemplo.
const planningWarning: MetricsWarning = {
  kind: "planning",
  code: "stories_without_estimate",
  message: "Hay 6 historias sin estimar en el backlog.",
};
// Advertencia de inconsistencia de datos de ejemplo.
const inconsistencyWarning: MetricsWarning = {
  kind: "data_inconsistency",
  code: "finished_without_hours",
  message: "Hay 2 tareas Terminadas sin horas registradas (casos: 3 y 5).",
};

// Métricas por defecto con las advertencias indicadas.
function metricsWithWarnings(
  warnings: MetricsWarning[],
): ProjectMetricsResponse {
  return { ...defaultMetrics, warnings };
}

// Punto de serie de un sprint Cerrado.
function closedPoint(index: number, name: string, value: number): SeriesPoint {
  return {
    sprintId: `c${index}000000-0000-4000-8000-0000000000${String(index).padStart(2, "0")}`,
    sprintName: name,
    sprintStatus: "closed",
    status: "available",
    value,
    finality: "definitive",
  };
}

// Series por defecto con un quinto sprint ya Cerrado en las seis listas.
function seriesWithClosedSprint5(): ProjectSeriesResponse {
  const add = (list: SeriesPoint[], value: number) => [
    ...list,
    closedPoint(5, "Sprint 5", value),
  ];
  return {
    plannedStoryPoints: add(defaultSeries.plannedStoryPoints, 18),
    completedStoryPoints: add(defaultSeries.completedStoryPoints, 17),
    estimatedHours: add(defaultSeries.estimatedHours, 108),
    actualHours: add(defaultSeries.actualHours, 100),
    defectsDetected: add(defaultSeries.defectsDetected, 2),
    defectsResolved: add(defaultSeries.defectsResolved, 2),
  };
}

// Espera a que el resumen se muestre.
async function findSummary() {
  return screen.findByRole("region", { name: SUMMARY });
}

// Espera a que la vista haya hecho todas sus consultas y termine de pintar la respuesta.
async function settle(requests: RecordedRequest[]) {
  await waitFor(() => {
    const paths = requests.map((request) => request.path);
    expect(paths.some((path) => path.endsWith("/metrics/series"))).toBe(true);
    expect(paths.some((path) => path.endsWith("/metrics/active-sprint"))).toBe(
      true,
    );
  });
  await waitFor(() => {
    expect(screen.queryByRole("status")).toBeNull();
  });
}

// Verifica que las tres figuras se muestran.
async function expectAllFigures() {
  for (const name of FIGURES) {
    expect(await screen.findByRole("figure", { name })).toBeDefined();
  }
}

// Verifica que ninguna sección de métricas (indicadores, figuras y distribución) se muestra.
function expectNoMetricsSections() {
  expect(screen.queryByRole("region", { name: INDICATORS })).toBeNull();
  expect(screen.queryAllByRole("figure")).toHaveLength(0);
  expect(screen.queryByRole("region", { name: DISTRIBUTION })).toBeNull();
}

// Escenarios de uso del dashboard con informaciÃ³n faltante o con una secciÃ³n que falla.
describe("009/US6 - Usar el dashboard cuando falta información o falla una sección", () => {
  // Sustituye ResizeObserver para que los grÃ¡ficos se dibujen en jsdom.
  beforeEach(() => {
    installResizeObserverStub();
  });

  // Desmonta la vista y restablece la sesiÃ³n, los temporizadores y los globales sustituidos.
  afterEach(() => {
    cleanup();
    clearSession();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  // Escenario: US6-1 (caso límite: proyecto recién creado, sin sprints ni historias)
  it("muestra el estado vacío en un proyecto recién creado", async () => {
    const requests = installDashboardBackend({
      project: { status: 200, body: projectWithoutSprint },
      sprints: { status: 200, body: noSprints },
    });
    renderDashboard(defaultProjectId);

    const region = await screen.findByRole("region", { name: EMPTY_TITLE });
    expect(within(region).getByText(EMPTY_TEXT)).toBeDefined();
    expect(within(region).queryAllByRole("link")).toHaveLength(0);
    await settle(requests);
    expectNoMetricsSections();
  });

  // Escenario: US6-2 (caso alternativo: sin sprints pero con historias en el backlog)
  it("muestra el resumen y avisa que no hay sprints", async () => {
    const requests = installDashboardBackend({
      project: {
        status: 200,
        body: {
          ...projectWithoutSprint,
          backlog: {
            countsByState: { todo: 5, ready: 3 },
            estimatedStoryPoints: 34,
            estimatedHours: 204,
          },
        },
      },
      sprints: { status: 200, body: noSprints },
    });
    renderDashboard(defaultProjectId);

    const summary = await findSummary();
    expect(
      within(summary).getByText("Plataforma de seguimiento"),
    ).toBeDefined();
    for (const name of [
      "Ana Pérez",
      "Bruno Díaz",
      "Carla Gómez",
      "Diego Ruiz",
    ]) {
      expect(within(summary).getByText(name)).toBeDefined();
    }
    expect(
      await screen.findByRole("region", { name: EMPTY_TITLE }),
    ).toBeDefined();
    await settle(requests);
    expectNoMetricsSections();
  });

  // Escenario: US6-3 (caso normal: advertencia de planificación con 6 historias sin estimar)
  it("muestra la advertencia de planificación", async () => {
    installDashboardBackend({
      metrics: { status: 200, body: metricsWithWarnings([planningWarning]) },
    });
    renderDashboard(defaultProjectId);

    const region = await screen.findByRole("region", { name: "Advertencias" });
    expect(within(region).getByText(planningWarning.message)).toBeDefined();
  });

  // Escenario: US6-4 (caso alternativo: advertencia de inconsistencia de datos)
  it("distingue la advertencia de inconsistencia de la de planificación", async () => {
    installDashboardBackend({
      metrics: {
        status: 200,
        body: metricsWithWarnings([planningWarning, inconsistencyWarning]),
      },
    });
    renderDashboard(defaultProjectId);

    const region = await screen.findByRole("region", { name: "Advertencias" });
    const alert = within(region).getByRole("alert", {
      name: "Datos que hay que revisar",
    });
    const status = within(region).getByRole("status", {
      name: "Avisos de planificación",
    });
    expect(within(alert).getByText(inconsistencyWarning.message)).toBeDefined();
    expect(within(alert).queryByText(planningWarning.message)).toBeNull();
    expect(within(status).getByText(planningWarning.message)).toBeDefined();
    expect(within(status).queryByText(inconsistencyWarning.message)).toBeNull();
    // Los indicadores se muestran igual.
    const indicators = await screen.findByRole("region", { name: INDICATORS });
    expect(within(indicators).getByText("Velocidad del equipo")).toBeDefined();
  });

  // Escenario: US6-5 (caso de error: defectsDetected malformada, falla solo la figura de calidad)
  it("aísla la falla del gráfico de calidad", async () => {
    installDashboardBackend({
      series: {
        status: 200,
        body: { ...defaultSeries, defectsDetected: "no es una lista" },
      },
    });
    renderDashboard(defaultProjectId);

    expect(await screen.findByText(QUALITY_ERROR)).toBeDefined();
    expect(screen.queryByRole("figure", { name: FIGURES[2] })).toBeNull();
    expect(await findSummary()).toBeDefined();
    const indicators = await screen.findByRole("region", { name: INDICATORS });
    expect(within(indicators).getByText("Velocidad del equipo")).toBeDefined();
    expect(
      await screen.findByRole("figure", { name: FIGURES[0] }),
    ).toBeDefined();
    expect(
      await screen.findByRole("figure", { name: FIGURES[1] }),
    ).toBeDefined();
    expect(screen.queryByText(VELOCITY_ERROR)).toBeNull();
    expect(screen.queryByText(EFFORT_ERROR)).toBeNull();
  });

  // Escenario: US6-6 (caso de error: 500 en las métricas del proyecto y en las series)
  it("informa por separado la falla de dos secciones", async () => {
    installDashboardBackend({ metrics: serverError, series: serverError });
    renderDashboard(defaultProjectId);

    for (const message of [
      PROJECT_INDICATORS_ERROR,
      VELOCITY_ERROR,
      EFFORT_ERROR,
      QUALITY_ERROR,
      DISTRIBUTION_ERROR,
      WARNINGS_ERROR,
    ]) {
      expect(await screen.findByText(message)).toBeDefined();
    }
    expect(await findSummary()).toBeDefined();
    expect(
      await screen.findByRole("heading", {
        name: "Indicadores del sprint activo",
      }),
    ).toBeDefined();
    expect(screen.queryByText(SUMMARY_ERROR)).toBeNull();
  });

  // Escenario: US6-7 (caso alternativo: la sección vuelve a responder al recargar)
  it("muestra la sección que vuelve a responder al recargar", async () => {
    installDashboardBackend({ series: serverError });
    const view = renderDashboard(defaultProjectId);
    expect(await screen.findByText(VELOCITY_ERROR)).toBeDefined();
    view.unmount();

    // Recarga: el backend ya responde las series.
    installDashboardBackend();
    renderDashboard(defaultProjectId);

    await expectAllFigures();
    expect(screen.queryByText(VELOCITY_ERROR)).toBeNull();
  });

  // Escenario: US6-8 (caso alternativo: datos vigentes al recargar después de cerrar el sprint, SC-013)
  it("muestra los datos vigentes al recargar después de cerrar el sprint", async () => {
    installDashboardBackend();
    const view = renderDashboard(defaultProjectId);
    expect(
      await screen.findByText("Entregar el tablero de métricas del proyecto."),
    ).toBeDefined();
    view.unmount();

    // Recarga: el sprint ya está Cerrado y no hay sprint en curso.
    installDashboardBackend({
      project: { status: 200, body: projectWithoutSprint },
      activeSprintMetrics: {
        status: 200,
        body: { activeSprint: false, metrics: null },
      },
      series: { status: 200, body: seriesWithClosedSprint5() },
    });
    renderDashboard(defaultProjectId);

    const figure = await screen.findByRole("figure", { name: FIGURES[0] });
    const table = within(figure).getByRole("table", {
      name: "Datos de Velocidad por sprint",
    });
    expect(within(table).getByText("Sprint 5")).toBeDefined();
    expect(screen.getAllByText("No hay ningún sprint en curso.").length).toBe(
      2,
    );
    expect(
      screen.queryByText("Entregar el tablero de métricas del proyecto."),
    ).toBeNull();
  });

  // Escenario: US6-9 (caso de error: falla el resumen; se invita a reintentar)
  it("no muestra el dashboard si falla el resumen e invita a reintentar", async () => {
    const requests = installDashboardBackend({ project: serverError });
    const view = renderDashboard(defaultProjectId);

    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText(SUMMARY_ERROR)).toBeDefined();
    const retry = screen.getByRole("button", { name: "Reintentar" });
    expect(screen.queryByRole("region", { name: SUMMARY })).toBeNull();
    expect(screen.queryByRole("region", { name: INDICATORS })).toBeNull();
    expect(screen.queryAllByRole("figure")).toHaveLength(0);
    expect(requests.length).toBe(1);

    // Con el backend corregido, reintentar muestra el dashboard.
    installDashboardBackend();
    fireEvent.click(retry);
    expect(await findSummary()).toBeDefined();
    await expectAllFigures();
    view.unmount();

    // Con un 404 no hay nada que reintentar.
    installDashboardBackend({
      project: { status: 404, body: { error: "El proyecto no existe." } },
    });
    renderDashboard(defaultProjectId);
    expect(await screen.findByRole("alert")).toBeDefined();
    expect(screen.queryByRole("button", { name: "Reintentar" })).toBeNull();
  });

  // Pruebas transversales: sin etiqueta de historia (RN-15, SC-010).
  describe("degradación por consulta", () => {
    it("falla solo el sprint activo si su consulta devuelve 500", async () => {
      const requests = installDashboardBackend({ sprint: serverError });
      renderDashboard(defaultProjectId);

      expect(await screen.findByText(ACTIVE_SPRINT_ERROR)).toBeDefined();
      await findSummary();
      await expectAllFigures();
      await settle(requests);
      expect(screen.getAllByRole("alert")).toHaveLength(1);
      expect(screen.getByRole("region", { name: INDICATORS })).toBeDefined();
    });

    it("muestra las secciones normalmente si el listado de sprints devuelve 500", async () => {
      const requests = installDashboardBackend({ sprints: serverError });
      renderDashboard(defaultProjectId);

      await findSummary();
      expect(
        await screen.findByRole("region", { name: INDICATORS }),
      ).toBeDefined();
      await expectAllFigures();
      expect(
        await screen.findByRole("region", { name: DISTRIBUTION }),
      ).toBeDefined();
      await settle(requests);
      expect(requests.some((r) => r.path.endsWith("/sprints"))).toBe(true);
      expect(screen.queryAllByRole("alert")).toHaveLength(0);
      expect(screen.queryByRole("region", { name: EMPTY_TITLE })).toBeNull();
    });

    it("falla solo lo que usa las métricas del proyecto si devuelven 500", async () => {
      const requests = installDashboardBackend({ metrics: serverError });
      renderDashboard(defaultProjectId);

      await findSummary();
      for (const message of [
        PROJECT_INDICATORS_ERROR,
        DISTRIBUTION_ERROR,
        WARNINGS_ERROR,
      ]) {
        expect(await screen.findByText(message)).toBeDefined();
      }
      await expectAllFigures();
      await settle(requests);
      expect(screen.getAllByRole("alert")).toHaveLength(3);
      expect(
        screen.getByRole("heading", { name: "Indicadores del sprint activo" }),
      ).toBeDefined();
    });

    it("falla solo los indicadores del sprint activo si sus métricas devuelven 500", async () => {
      const requests = installDashboardBackend({
        activeSprintMetrics: serverError,
      });
      renderDashboard(defaultProjectId);

      await findSummary();
      expect(await screen.findByText(SPRINT_INDICATORS_ERROR)).toBeDefined();
      await expectAllFigures();
      await settle(requests);
      expect(screen.getAllByRole("alert")).toHaveLength(1);
      expect(screen.getByRole("region", { name: DISTRIBUTION })).toBeDefined();
    });

    it("falla solo los tres gráficos si las series devuelven 500", async () => {
      const requests = installDashboardBackend({ series: serverError });
      renderDashboard(defaultProjectId);

      await findSummary();
      for (const message of [VELOCITY_ERROR, EFFORT_ERROR, QUALITY_ERROR]) {
        expect(await screen.findByText(message)).toBeDefined();
      }
      await settle(requests);
      expect(screen.getAllByRole("alert")).toHaveLength(3);
      expect(screen.queryAllByRole("figure")).toHaveLength(0);
      expect(screen.getByRole("region", { name: INDICATORS })).toBeDefined();
      expect(screen.getByRole("region", { name: DISTRIBUTION })).toBeDefined();
    });
  });

  it("muestra la carga de métricas mientras el listado de sprints no responde", async () => {
    installDashboardBackend();
    // El listado de sprints queda sin responder; las demás consultas pasan al backend de prueba.
    const backend = globalThis.fetch;
    vi.stubGlobal(
      "fetch",
      (input: RequestInfo | URL, init?: RequestInit): Promise<Response> =>
        String(input).includes(`/projects/${defaultProjectId}/sprints`)
          ? new Promise<Response>(() => {})
          : backend(input, init),
    );
    renderDashboard(defaultProjectId);

    await findSummary();
    const loading = await screen.findByText("Cargando métricas…");
    expect(loading.getAttribute("role")).toBe("status");
    expect(screen.queryByRole("region", { name: INDICATORS })).toBeNull();
    expect(screen.queryAllByRole("figure")).toHaveLength(0);
  });

  // RN-17: el dashboard no se actualiza solo.
  it("no vuelve a consultar sin recargar", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const requests = installDashboardBackend();
    renderDashboard(defaultProjectId);

    await findSummary();
    await expectAllFigures();
    await settle(requests);
    const before = requests.length;

    await vi.advanceTimersByTimeAsync(10 * 60 * 1000);

    expect(requests.length).toBe(before);
  });

  // RN-02, FR-003: el dashboard es de solo lectura (prueba de caracterización, sin RED).
  it("solo hace consultas GET", async () => {
    const requests = installDashboardBackend();
    renderDashboard(defaultProjectId);

    await findSummary();
    await expectAllFigures();
    await settle(requests);

    // Todas las peticiones son GET y son exactamente las seis operaciones, una vez cada una.
    const base = `/api/v1/projects/${defaultProjectId}`;
    expect(requests.every((request) => request.method === "GET")).toBe(true);
    expect(requests.map((request) => request.path).sort()).toEqual(
      [
        base,
        `/api/v1/sprints/${defaultSprint.id}`,
        `${base}/sprints`,
        `${base}/metrics`,
        `${base}/metrics/active-sprint`,
        `${base}/metrics/series`,
      ].sort(),
    );

    // No hay controles de edición ni campos de entrada en la pantalla.
    expect(
      screen.queryAllByRole("button", {
        name: /editar|eliminar|borrar|guardar|crear|agregar|nuevo/i,
      }),
    ).toHaveLength(0);
    expect(screen.queryAllByRole("textbox")).toHaveLength(0);
  });
});
