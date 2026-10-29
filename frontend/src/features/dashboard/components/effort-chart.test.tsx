import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, within } from "@testing-library/react";
import {
  defaultProjectId,
  defaultSeries,
  installDashboardBackend,
  installResizeObserverStub,
  renderDashboard,
} from "@/features/dashboard/components/dashboard-test-backend";
import { clearSession } from "@/lib/session-storage";
import type { ProjectSeriesResponse, SeriesPoint } from "@/types/metrics";

// Pruebas de escenario de la Historia 4 (T026): montan DashboardView completo con el backend de
// prueba compartido y verifican la figura "Esfuerzo por sprint" a través de su tabla de datos.

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

const TITLE = "Esfuerzo por sprint";
const EMPTY = "Todavía no hay sprints iniciados para mostrar esfuerzo.";
const ERROR = "No se pudo cargar el gráfico de esfuerzo.";

// Punto de serie de un sprint con el estado indicado.
function point(
  index: number,
  name: string,
  value: number,
  sprintStatus: "active" | "closed" = "closed",
): SeriesPoint {
  return {
    sprintId: `d${index}000000-0000-4000-8000-0000000000${String(index).padStart(2, "0")}`,
    sprintName: name,
    sprintStatus,
    status: "available",
    value,
    finality: sprintStatus === "active" ? "partial" : "definitive",
  };
}

// Series de esfuerzo con los pares indicados; el resto de las series queda con los datos por defecto.
function seriesWith(
  estimated: SeriesPoint[],
  actual: SeriesPoint[],
): ProjectSeriesResponse {
  return { ...defaultSeries, estimatedHours: estimated, actualHours: actual };
}

// Tres sprints Cerrados de la spec: estimadas 126/144/108 y reales 142,5/130/95.
function specPairs(): [SeriesPoint[], SeriesPoint[]] {
  return [
    [
      point(1, "Sprint 1", 126),
      point(2, "Sprint 2", 144),
      point(3, "Sprint 3", 108),
    ],
    [
      point(1, "Sprint 1", 142.5),
      point(2, "Sprint 2", 130),
      point(3, "Sprint 3", 95),
    ],
  ];
}

// Figura de esfuerzo, una vez cargada.
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

describe("009/US4 - Ver la evolución del esfuerzo", () => {
  beforeEach(() => {
    installResizeObserverStub();
  });

  afterEach(() => {
    cleanup();
    clearSession();
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  // Escenario: US4-1 (caso normal: tres sprints con estimadas y reales)
  it("grafica las horas estimadas y reales por sprint", async () => {
    const [estimated, actual] = specPairs();
    installDashboardBackend({
      series: { status: 200, body: seriesWith(estimated, actual) },
    });
    renderDashboard(defaultProjectId);

    const figure = await findFigure();
    const rows = dataRows(figure);
    expect(rows).toHaveLength(3);
    [
      ["126 h", "142,5 h"],
      ["144 h", "130 h"],
      ["108 h", "95 h"],
    ].forEach(([est, act], index) => {
      expect(
        within(rows[index]).getByText(`Sprint ${index + 1}`),
      ).toBeDefined();
      expect(within(rows[index]).getByText(est)).toBeDefined();
      expect(within(rows[index]).getByText(act)).toBeDefined();
    });

    expect(within(figure).getByText(TITLE).tagName).toBe("FIGCAPTION");
    expect(
      within(figure).getAllByText("Horas estimadas").length,
    ).toBeGreaterThan(0);
    expect(within(figure).getAllByText("Horas reales").length).toBeGreaterThan(
      0,
    );
    const svg = figure.querySelector("svg.recharts-surface");
    expect(svg).not.toBeNull();
    expect(svg!.textContent).toContain("Sprint");
    expect(svg!.textContent).toContain("Horas");
  });

  // Escenario: US4-2 (caso alternativo: un sprint Activo se identifica como parcial)
  it("marca como parcial al sprint activo", async () => {
    const [estimated, actual] = specPairs();
    estimated.push(point(4, "Sprint 4", 90, "active"));
    actual.push(point(4, "Sprint 4", 30, "active"));
    installDashboardBackend({
      series: { status: 200, body: seriesWith(estimated, actual) },
    });
    renderDashboard(defaultProjectId);

    const figure = await findFigure();
    const rows = dataRows(figure);
    expect(rows).toHaveLength(4);
    expect(within(rows[3]).getAllByText("Parcial")).toHaveLength(2);
    expect(figure.querySelector("svg.recharts-surface")!.textContent).toContain(
      "Sprint 4 (parcial)",
    );
  });

  // Escenario: US4-3 (caso límite: estimadas 0 con reales registradas)
  it("muestra 0 horas estimadas sin perder el punto", async () => {
    installDashboardBackend({
      series: {
        status: 200,
        body: seriesWith(
          [point(1, "Sprint 1", 0), point(2, "Sprint 2", 100)],
          [point(1, "Sprint 1", 12), point(2, "Sprint 2", 90)],
        ),
      },
    });
    renderDashboard(defaultProjectId);

    const rows = dataRows(await findFigure());
    expect(rows).toHaveLength(2);
    expect(within(rows[0]).getByText("0 h")).toBeDefined();
    expect(within(rows[0]).getByText("12 h")).toBeDefined();
  });

  // Escenario: US4-4 (caso límite: sprint sin horas reales)
  it("muestra 0 horas reales en un sprint sin registros", async () => {
    installDashboardBackend({
      series: {
        status: 200,
        body: seriesWith(
          [point(1, "Sprint 1", 80), point(2, "Sprint 2", 100)],
          [point(1, "Sprint 1", 0), point(2, "Sprint 2", 90)],
        ),
      },
    });
    renderDashboard(defaultProjectId);

    const rows = dataRows(await findFigure());
    expect(within(rows[0]).getByText("80 h")).toBeDefined();
    expect(within(rows[0]).getByText("0 h")).toBeDefined();
  });

  // Escenario: US4-5 (caso límite: series vacías)
  it("muestra un estado vacío sin sprints con esfuerzo", async () => {
    installDashboardBackend({
      series: { status: 200, body: seriesWith([], []) },
    });
    const { container } = renderDashboard(defaultProjectId);

    expect(await screen.findByText(EMPTY)).toBeDefined();
    expect(screen.queryByRole("figure", { name: TITLE })).toBeNull();
    // Todo SVG de la vista pertenece al gráfico de velocidad o al de calidad: el de esfuerzo no
    // dibuja ninguno.
    const velocity = await screen.findByRole("figure", {
      name: "Velocidad por sprint",
    });
    const quality = await screen.findByRole("figure", {
      name: "Defectos por sprint",
    });
    const surfaces = container.querySelectorAll("svg.recharts-surface");
    expect(surfaces.length).toBeGreaterThan(0);
    surfaces.forEach((surface) => {
      expect(velocity.contains(surface) || quality.contains(surface)).toBe(
        true,
      );
    });
  });

  // Escenario: US4-6 (caso de error: actualHours malformada, falla solo este gráfico)
  it("informa la falla del gráfico de esfuerzo y muestra los demás", async () => {
    installDashboardBackend({
      series: {
        status: 200,
        body: { ...defaultSeries, actualHours: "no es una lista" },
      },
    });
    renderDashboard(defaultProjectId);

    expect(await screen.findByText(ERROR)).toBeDefined();
    expect(screen.queryByRole("figure", { name: TITLE })).toBeNull();
    expect(
      await screen.findByRole("figure", { name: "Velocidad por sprint" }),
    ).toBeDefined();
  });
});
