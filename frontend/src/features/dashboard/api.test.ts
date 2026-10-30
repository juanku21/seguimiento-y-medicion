import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import {
  getActiveSprintMetrics,
  getProjectMetrics,
  getProjectSeries,
  readSeries,
  isActiveSprintMetricsResponse,
  isMetric,
  isProjectMetricsResponse,
  isSprintMetric,
} from "@/features/dashboard/api";
import type { ProjectSeriesResponse } from "@/types/metrics";
import { writeToken } from "@/lib/session-storage";

type FetchMock = Mock<typeof fetch>;

// Valor calculado disponible y métrica de sprint con la finalidad indicada.
const available = (value: number) => ({ status: "available", value });
const sprintMetric = (finality: string | null = "definitive") => ({
  ...available(1),
  finality,
});

const metricNames = [
  "plannedStoryPoints",
  "completedStoryPoints",
  "estimatedHours",
  "actualHours",
  "deviationHours",
  "deviationPercent",
  "storyCompletionPercent",
];

// Cuerpo completo válido de las métricas del sprint activo.
function activeMetrics(): Record<string, unknown> {
  return Object.fromEntries(metricNames.map((n) => [n, sprintMetric()]));
}

// Cuerpo completo válido de las métricas del proyecto.
function projectMetrics() {
  return {
    velocity: { value: available(14) },
    cumulative: {
      finality: "definitive",
      ...Object.fromEntries(metricNames.map((n) => [n, available(1)])),
    },
    defects: {
      open: {
        total: 7,
        bySeverity: { critical: 1, high: 2, medium: 3, low: 1 },
      },
    },
    warnings: [{ kind: "planning", code: "x", message: "Aviso" }] as object[],
  };
}

describe("Reconocedores del dashboard - validación de entradas", () => {
  it("isMetric acepta available con número y los otros estados con null", () => {
    expect(isMetric({ status: "available", value: 3 })).toBe(true);
    expect(isMetric({ status: "not_calculable", value: null })).toBe(true);
    expect(isMetric({ status: "not_applicable", value: null })).toBe(true);
  });

  it("isMetric rechaza combinaciones incoherentes y estados desconocidos", () => {
    expect(isMetric({ status: "available", value: null })).toBe(false);
    expect(isMetric({ status: "not_calculable", value: 3 })).toBe(false);
    expect(isMetric({ status: "unknown", value: null })).toBe(false);
  });

  it("isSprintMetric exige finality válido o null", () => {
    expect(isSprintMetric(sprintMetric("partial"))).toBe(true);
    expect(isSprintMetric(sprintMetric(null))).toBe(true);
    expect(isSprintMetric(sprintMetric("otra"))).toBe(false);
    expect(isSprintMetric(available(1))).toBe(false);
  });

  it("isActiveSprintMetricsResponse acepta sin sprint activo y la respuesta completa", () => {
    expect(
      isActiveSprintMetricsResponse({ activeSprint: false, metrics: null }),
    ).toBe(true);
    expect(
      isActiveSprintMetricsResponse({
        activeSprint: true,
        metrics: activeMetrics(),
      }),
    ).toBe(true);
  });

  it("isActiveSprintMetricsResponse rechaza sprint activo sin métricas o con una métrica faltante", () => {
    expect(
      isActiveSprintMetricsResponse({ activeSprint: true, metrics: null }),
    ).toBe(false);
    const incomplete = activeMetrics();
    delete incomplete.actualHours;
    expect(
      isActiveSprintMetricsResponse({
        activeSprint: true,
        metrics: incomplete,
      }),
    ).toBe(false);
  });

  it("isProjectMetricsResponse acepta la respuesta completa", () => {
    expect(isProjectMetricsResponse(projectMetrics())).toBe(true);
  });

  it("isProjectMetricsResponse rechaza una severidad faltante", () => {
    const body = projectMetrics();
    delete (body.defects.open.bySeverity as Record<string, number>).low;
    expect(isProjectMetricsResponse(body)).toBe(false);
  });

  it("isProjectMetricsResponse rechaza una advertencia sin message", () => {
    const body = projectMetrics();
    body.warnings = [{ kind: "planning", code: "x" }];
    expect(isProjectMetricsResponse(body)).toBe(false);
  });

  it("isProjectMetricsResponse rechaza cumulative.finality provisional", () => {
    const body = projectMetrics();
    body.cumulative.finality = "provisional";
    expect(isProjectMetricsResponse(body)).toBe(false);
  });
});

describe("Consultas del dashboard - rutas y cuerpos", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con sesión en la pestaña y un fetch sustituido.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJicnVubyJ9.firma");
    fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);
  });

  // Restaura fetch y limpia la sesión.
  afterEach(() => {
    vi.unstubAllGlobals();
    sessionStorage.clear();
  });

  // Hace que fetch responda 200 con el cuerpo dado.
  function respond(body: unknown) {
    fetchMock.mockImplementation(
      async () =>
        new Response(JSON.stringify(body), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
  }

  // Devuelve método y URL de la única llamada a fetch.
  function requested(): { method: string; url: string } {
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [input, init] = fetchMock.mock.calls[0];
    const url =
      input instanceof Request
        ? input.url
        : input instanceof URL
          ? input.href
          : input;
    const method =
      input instanceof Request ? input.method : (init?.method ?? "GET");
    return { method, url };
  }

  it("getProjectMetrics pide con GET la ruta con el identificador codificado", async () => {
    respond(projectMetrics());
    const result = await getProjectMetrics("a/b");
    const { method, url } = requested();
    expect(method).toBe("GET");
    expect(url.endsWith("/api/v1/projects/a%2Fb/metrics")).toBe(true);
    expect(result.ok).toBe(true);
  });

  it("getActiveSprintMetrics pide con GET la ruta con el identificador codificado", async () => {
    respond({ activeSprint: false, metrics: null });
    const result = await getActiveSprintMetrics("a/b");
    const { method, url } = requested();
    expect(method).toBe("GET");
    expect(url.endsWith("/api/v1/projects/a%2Fb/metrics/active-sprint")).toBe(
      true,
    );
    expect(result.ok).toBe(true);
  });

  it("getProjectMetrics devuelve error si el cuerpo no cumple el reconocedor", async () => {
    respond({ inesperado: true });
    const result = await getProjectMetrics("p1");
    expect(result.ok).toBe(false);
  });

  it("getActiveSprintMetrics devuelve error si el cuerpo no cumple el reconocedor", async () => {
    respond({ activeSprint: true, metrics: null });
    const result = await getActiveSprintMetrics("p1");
    expect(result.ok).toBe(false);
  });
});

// Punto de serie válido; las pruebas lo alteran para cada caso.
const seriesPoint = (overrides: Record<string, unknown> = {}) => ({
  sprintId: "s1",
  sprintName: "Sprint 1",
  sprintStatus: "closed",
  status: "available",
  value: 5,
  finality: "definitive",
  ...overrides,
});

// Sobre de series con las seis listas válidas.
function seriesEnvelope(overrides: Record<string, unknown> = {}) {
  const keys = [
    "plannedStoryPoints",
    "completedStoryPoints",
    "estimatedHours",
    "actualHours",
    "defectsDetected",
    "defectsResolved",
  ];
  return {
    ...Object.fromEntries(keys.map((k) => [k, [seriesPoint()]])),
    ...overrides,
  } as unknown as ProjectSeriesResponse;
}

describe("readSeries - validación por porción (RN-20)", () => {
  it("devuelve los puntos de una serie válida", () => {
    const envelope = seriesEnvelope();
    expect(readSeries(envelope, "plannedStoryPoints")).toEqual([seriesPoint()]);
  });

  it("devuelve null si la serie no es una lista", () => {
    const envelope = seriesEnvelope({ plannedStoryPoints: "no es lista" });
    expect(readSeries(envelope, "plannedStoryPoints")).toBeNull();
  });

  it("devuelve null si un punto no tiene sprintId", () => {
    const envelope = seriesEnvelope({
      plannedStoryPoints: [seriesPoint({ sprintId: undefined })],
    });
    expect(readSeries(envelope, "plannedStoryPoints")).toBeNull();
  });

  it("devuelve null si un punto tiene un sprintStatus desconocido", () => {
    const envelope = seriesEnvelope({
      plannedStoryPoints: [seriesPoint({ sprintStatus: "planned" })],
    });
    expect(readSeries(envelope, "plannedStoryPoints")).toBeNull();
  });

  it("devuelve null si un punto tiene una finality desconocida", () => {
    const envelope = seriesEnvelope({
      plannedStoryPoints: [seriesPoint({ finality: "otra" })],
    });
    expect(readSeries(envelope, "plannedStoryPoints")).toBeNull();
  });

  it("con defectsDetected malformada, plannedStoryPoints sigue devolviendo sus puntos", () => {
    const envelope = seriesEnvelope({ defectsDetected: 42 });
    expect(readSeries(envelope, "defectsDetected")).toBeNull();
    expect(readSeries(envelope, "plannedStoryPoints")).toEqual([seriesPoint()]);
  });
});

describe("getProjectSeries - ruta", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    sessionStorage.clear();
  });

  it("pide con GET la ruta de series con el identificador codificado", async () => {
    sessionStorage.clear();
    writeToken("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJicnVubyJ9.firma");
    const fetchMock = vi.fn<typeof fetch>(
      async () =>
        new Response(JSON.stringify(seriesEnvelope()), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const result = await getProjectSeries("a/b");
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [input, init] = fetchMock.mock.calls[0];
    const url =
      input instanceof Request
        ? input.url
        : input instanceof URL
          ? input.href
          : input;
    const method =
      input instanceof Request ? input.method : (init?.method ?? "GET");
    expect(method).toBe("GET");
    expect(url.endsWith("/api/v1/projects/a%2Fb/metrics/series")).toBe(true);
    expect(result.ok).toBe(true);
  });
});

// Ramas de rechazo de los reconocedores ante cuerpos que no son objetos o con partes mal formadas (T045).
describe("Reconocedores del dashboard - ramas de rechazo (T045)", () => {
  it("isSprintMetric rechaza valores que no son objetos", () => {
    expect(isSprintMetric(null)).toBe(false);
    expect(isSprintMetric("available")).toBe(false);
  });

  it("isActiveSprintMetricsResponse rechaza cuerpos que no son objetos o sin activeSprint booleano", () => {
    expect(isActiveSprintMetricsResponse(null)).toBe(false);
    expect(
      isActiveSprintMetricsResponse({ activeSprint: "true", metrics: null }),
    ).toBe(false);
  });

  it("isProjectMetricsResponse rechaza defects.open sin total numérico o sin bySeverity", () => {
    const withoutTotal = projectMetrics();
    (withoutTotal.defects.open as Record<string, unknown>).total = "3";
    expect(isProjectMetricsResponse(withoutTotal)).toBe(false);

    const withoutSeverity = projectMetrics();
    (withoutSeverity.defects.open as Record<string, unknown>).bySeverity = null;
    expect(isProjectMetricsResponse(withoutSeverity)).toBe(false);

    const notObject = projectMetrics();
    (notObject.defects as Record<string, unknown>).open = null;
    expect(isProjectMetricsResponse(notObject)).toBe(false);
  });

  it("readSeries devuelve null si un punto de la serie no es un objeto", () => {
    const envelope = {
      plannedStoryPoints: [null],
    } as unknown as ProjectSeriesResponse;
    expect(readSeries(envelope, "plannedStoryPoints")).toBeNull();
  });
});
