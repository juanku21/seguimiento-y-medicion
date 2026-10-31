import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, within } from "@testing-library/react";
import {
  defaultMetrics,
  defaultProjectId,
  defaultSeries,
  installDashboardBackend,
  installResizeObserverStub,
  renderDashboard,
} from "@/features/dashboard/components/dashboard-test-backend";
import { clearSession } from "@/lib/session-storage";
import type {
  ProjectMetricsResponse,
  ProjectSeriesResponse,
  SeriesPoint,
} from "@/types/metrics";
import { DEFECT_SEVERITY_LABELS } from "@/utils/defect-labels";

// Pruebas de escenario de la Historia 5 (T028): montan DashboardView completo con el backend de
// prueba compartido y verifican la figura "Defectos por sprint" y la región de defectos abiertos.

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

// Título, región y mensajes esperados del gráfico de defectos, y orden de severidades.
const TITLE = "Defectos por sprint";
const REGION = "Defectos abiertos por severidad";
const ERROR = "No se pudo cargar el gráfico de defectos.";
const NO_OPEN = "No hay defectos abiertos.";
const SEVERITY_ORDER = ["critical", "high", "medium", "low"] as const;

// Punto de serie de un sprint Cerrado.
function point(index: number, name: string, value: number): SeriesPoint {
  return {
    sprintId: `e${index}000000-0000-4000-8000-0000000000${String(index).padStart(2, "0")}`,
    sprintName: name,
    sprintStatus: "closed",
    status: "available",
    value,
    finality: "definitive",
  };
}

// Series de defectos con los pares indicados; el resto queda con los datos por defecto.
function seriesWith(
  detected: SeriesPoint[],
  resolved: SeriesPoint[],
): ProjectSeriesResponse {
  return {
    ...defaultSeries,
    defectsDetected: detected,
    defectsResolved: resolved,
  };
}

// Métricas por defecto con los defectos abiertos indicados.
function metricsWith(
  total: number,
  bySeverity: { critical: number; high: number; medium: number; low: number },
): ProjectMetricsResponse {
  return {
    ...defaultMetrics,
    defects: { ...defaultMetrics.defects, open: { total, bySeverity } },
  };
}

// Figura de calidad, una vez cargada.
async function findFigure() {
  return screen.findByRole("figure", { name: TITLE });
}

// Filas de datos (sin el encabezado) de la tabla de la figura.
function dataRows(figure: HTMLElement): HTMLElement[] {
  const table = within(figure).getByRole("table", {
    name: `Datos de ${TITLE}`,
  });
  return within(table).getAllByRole("row").slice(1);
}

// Pares término/valor de la región de distribución, en el orden en que se muestran.
async function findDistribution(): Promise<Array<[string, string]>> {
  const region = await screen.findByRole("region", { name: REGION });
  const terms = Array.from(region.querySelectorAll("dt"));
  const values = Array.from(region.querySelectorAll("dd"));
  expect(terms.length).toBe(values.length);
  return terms.map((term, index) => [
    term.textContent ?? "",
    values[index].textContent ?? "",
  ]);
}

// Escenarios del gráfico de defectos por sprint y de la distribución de abiertos.
describe("009/US5 - Ver la calidad del producto", () => {
  // Sustituye ResizeObserver para que el gráfico se dibuje en jsdom.
  beforeEach(() => {
    installResizeObserverStub();
  });

  // Desmonta la vista y restablece la sesión y los globales sustituidos.
  afterEach(() => {
    cleanup();
    clearSession();
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  // Escenario: US5-1 (caso normal: tres sprints con detectados y resueltos)
  it("grafica los defectos detectados y resueltos por sprint", async () => {
    installDashboardBackend({
      series: {
        status: 200,
        body: seriesWith(
          [
            point(1, "Sprint 1", 7),
            point(2, "Sprint 2", 4),
            point(3, "Sprint 3", 9),
          ],
          [
            point(1, "Sprint 1", 4),
            point(2, "Sprint 2", 6),
            point(3, "Sprint 3", 2),
          ],
        ),
      },
    });
    renderDashboard(defaultProjectId);

    const figure = await findFigure();
    const rows = dataRows(figure);
    expect(rows).toHaveLength(3);
    [
      ["7", "4"],
      ["4", "6"],
      ["9", "2"],
    ].forEach(([detected, resolved], index) => {
      expect(
        within(rows[index]).getByText(`Sprint ${index + 1}`),
      ).toBeDefined();
      expect(within(rows[index]).getByText(detected)).toBeDefined();
      expect(within(rows[index]).getByText(resolved)).toBeDefined();
    });

    expect(within(figure).getByText(TITLE).tagName).toBe("FIGCAPTION");
    expect(
      within(figure).getAllByText("Defectos detectados").length,
    ).toBeGreaterThan(0);
    expect(
      within(figure).getAllByText("Defectos resueltos").length,
    ).toBeGreaterThan(0);
    const svg = figure.querySelector("svg.recharts-surface");
    expect(svg).not.toBeNull();
    expect(svg!.textContent).toContain("Sprint");
    expect(svg!.textContent).toContain("Defectos");
  });

  // Escenario: US5-2 (caso normal: distribución de defectos abiertos por severidad)
  it("muestra la distribución por severidad con su total", async () => {
    installDashboardBackend();
    renderDashboard(defaultProjectId);

    expect(await findDistribution()).toEqual([
      [DEFECT_SEVERITY_LABELS.critical, "2"],
      [DEFECT_SEVERITY_LABELS.high, "3"],
      [DEFECT_SEVERITY_LABELS.medium, "1"],
      [DEFECT_SEVERITY_LABELS.low, "1"],
      ["Total", "7"],
    ]);
  });

  // Escenario: US5-3 (caso límite: más resueltos que detectados en un sprint)
  it("muestra más resueltos que detectados sin tratarlo como error", async () => {
    installDashboardBackend({
      series: {
        status: 200,
        body: seriesWith([point(1, "Sprint 1", 2)], [point(1, "Sprint 1", 6)]),
      },
    });
    renderDashboard(defaultProjectId);

    const rows = dataRows(await findFigure());
    expect(rows).toHaveLength(1);
    expect(within(rows[0]).getByText("2")).toBeDefined();
    expect(within(rows[0]).getByText("6")).toBeDefined();
    expect(screen.queryAllByRole("alert")).toHaveLength(0);
  });

  // Escenario: US5-4 (caso límite: sin defectos abiertos)
  it("muestra 0 en las cuatro severidades sin defectos", async () => {
    installDashboardBackend({
      metrics: {
        status: 200,
        body: metricsWith(0, { critical: 0, high: 0, medium: 0, low: 0 }),
      },
    });
    renderDashboard(defaultProjectId);

    expect(await findDistribution()).toEqual([
      [DEFECT_SEVERITY_LABELS.critical, "0"],
      [DEFECT_SEVERITY_LABELS.high, "0"],
      [DEFECT_SEVERITY_LABELS.medium, "0"],
      [DEFECT_SEVERITY_LABELS.low, "0"],
      ["Total", "0"],
    ]);
    const region = screen.getByRole("region", { name: REGION });
    expect(within(region).getByText(NO_OPEN)).toBeDefined();
    expect(screen.queryAllByRole("alert")).toHaveLength(0);
  });

  // Escenario: US5-5 (caso límite: una sola severidad con defectos)
  it("muestra una sola severidad y las otras en 0", async () => {
    installDashboardBackend({
      metrics: {
        status: 200,
        body: metricsWith(4, { critical: 4, high: 0, medium: 0, low: 0 }),
      },
    });
    renderDashboard(defaultProjectId);

    const pairs = await findDistribution();
    expect(pairs.slice(0, 4)).toEqual(
      SEVERITY_ORDER.map((key, index) => [
        DEFECT_SEVERITY_LABELS[key],
        index === 0 ? "4" : "0",
      ]),
    );
    expect(pairs[4]).toEqual(["Total", "4"]);
  });

  // Escenario: US5-6 (caso límite: los descartados no se cuentan y el total no se suma)
  it("no muestra los defectos descartados", async () => {
    // El total (5) difiere de la suma de las severidades (7): la vista muestra open.total tal cual.
    installDashboardBackend({
      metrics: {
        status: 200,
        body: metricsWith(5, { critical: 2, high: 3, medium: 1, low: 1 }),
      },
    });
    renderDashboard(defaultProjectId);

    const pairs = await findDistribution();
    expect(pairs).toEqual([
      [DEFECT_SEVERITY_LABELS.critical, "2"],
      [DEFECT_SEVERITY_LABELS.high, "3"],
      [DEFECT_SEVERITY_LABELS.medium, "1"],
      [DEFECT_SEVERITY_LABELS.low, "1"],
      ["Total", "5"],
    ]);
    const region = screen.getByRole("region", { name: REGION });
    expect(within(region).queryByText(/Descartad/i)).toBeNull();
  });

  // Escenario: US5-7 (caso de error: defectsDetected malformada, falla solo esta sección)
  it("informa la falla de la sección de calidad y muestra el resto", async () => {
    installDashboardBackend({
      series: {
        status: 200,
        body: { ...defaultSeries, defectsDetected: "no es una lista" },
      },
    });
    renderDashboard(defaultProjectId);

    expect(await screen.findByText(ERROR)).toBeDefined();
    expect(screen.queryByRole("figure", { name: TITLE })).toBeNull();
    expect(
      await screen.findByRole("figure", { name: "Velocidad por sprint" }),
    ).toBeDefined();
    expect(
      await screen.findByRole("figure", { name: "Esfuerzo por sprint" }),
    ).toBeDefined();
    expect(await screen.findByText("Plataforma de seguimiento")).toBeDefined();
    expect(await screen.findByText("Velocidad del equipo")).toBeDefined();
  });
});
