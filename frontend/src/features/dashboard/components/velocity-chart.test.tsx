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

// Pruebas de escenario de la Historia 3 (T021): montan DashboardView completo con el backend de
// prueba compartido y verifican la figura "Velocidad por sprint" a través de su tabla de datos.

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

const TITLE = "Velocidad por sprint";
const EMPTY = "Todavía no hay sprints cerrados para mostrar la velocidad.";
const ERROR = "No se pudo cargar el gráfico de velocidad.";
const SINGLE = "Todavía no hay evolución que comparar.";

// Punto de serie de un sprint con el estado indicado.
function point(
  index: number,
  name: string,
  value: number,
  sprintStatus: "active" | "closed" = "closed",
): SeriesPoint {
  return {
    sprintId: `c${index}000000-0000-4000-8000-0000000000${String(index).padStart(2, "0")}`,
    sprintName: name,
    sprintStatus,
    status: "available",
    value,
    finality: sprintStatus === "active" ? "partial" : "definitive",
  };
}

// Series de velocidad con los pares indicados; el resto de las series queda con los datos por defecto.
function seriesWith(
  planned: SeriesPoint[],
  completed: SeriesPoint[],
): ProjectSeriesResponse {
  return {
    ...defaultSeries,
    plannedStoryPoints: planned,
    completedStoryPoints: completed,
  };
}

// Figura de velocidad, una vez cargada.
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

describe("009/US3 - Ver la evolución de la velocidad", () => {
  beforeEach(() => {
    installResizeObserverStub();
  });

  afterEach(() => {
    cleanup();
    clearSession();
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  // Escenario: US3-1 (cuatro sprints Cerrados en el orden recibido)
  it("muestra los 4 pares de Story Points en el orden recibido", async () => {
    installDashboardBackend();
    const { container } = renderDashboard(defaultProjectId);

    const figure = await findFigure();
    const rows = dataRows(figure);
    expect(rows).toHaveLength(4);
    const expected = [
      ["Sprint 1", "21", "13"],
      ["Sprint 2", "24", "21"],
      ["Sprint 3", "13", "8"],
      ["Sprint 4", "25", "20"],
    ];
    expected.forEach(([name, planned, completed], index) => {
      expect(within(rows[index]).getByText(name)).toBeDefined();
      expect(within(rows[index]).getByText(planned)).toBeDefined();
      expect(within(rows[index]).getByText(completed)).toBeDefined();
    });

    expect(within(figure).getByText(TITLE).tagName).toBe("FIGCAPTION");
    expect(
      within(figure).getAllByText("Story Points planificados").length,
    ).toBeGreaterThan(0);
    expect(
      within(figure).getAllByText("Story Points completados").length,
    ).toBeGreaterThan(0);
    const svg = container.querySelector("svg.recharts-surface");
    expect(svg).not.toBeNull();
    expect(svg!.textContent).toContain("Sprint");
    expect(svg!.textContent).toContain("Story Points");
  });

  // Escenario: US3-2 (3 Cerrados y un Activo: el Activo no entra)
  it("omite el sprint Activo de las series", async () => {
    installDashboardBackend({
      series: {
        status: 200,
        body: seriesWith(
          [
            point(1, "Sprint 1", 21),
            point(2, "Sprint 2", 24),
            point(3, "Sprint 3", 13),
            point(4, "Sprint 4", 9, "active"),
          ],
          [
            point(1, "Sprint 1", 13),
            point(2, "Sprint 2", 21),
            point(3, "Sprint 3", 8),
            point(4, "Sprint 4", 4, "active"),
          ],
        ),
      },
    });
    renderDashboard(defaultProjectId);

    const figure = await findFigure();
    expect(dataRows(figure)).toHaveLength(3);
    expect(within(figure).queryByText(/\(parcial\)/)).toBeNull();
    expect(within(figure).queryByText("Sprint 4")).toBeNull();
  });

  // Escenario: US3-3 (un único Cerrado: no hay evolución que comparar)
  it("con un único sprint Cerrado muestra una fila y el aviso", async () => {
    installDashboardBackend({
      series: {
        status: 200,
        body: seriesWith(
          [point(1, "Sprint 1", 21)],
          [point(1, "Sprint 1", 13)],
        ),
      },
    });
    renderDashboard(defaultProjectId);

    const figure = await findFigure();
    expect(dataRows(figure)).toHaveLength(1);
    expect(within(figure).getByText(SINGLE)).toBeDefined();
  });

  // Escenario: US3-4 (series vacías: todavía no hay sprints cerrados)
  it("con series vacías muestra el mensaje de vacío y ningún gráfico", async () => {
    installDashboardBackend({
      series: { status: 200, body: seriesWith([], []) },
    });
    const { container } = renderDashboard(defaultProjectId);

    expect(await screen.findByText(EMPTY)).toBeDefined();
    expect(screen.queryByRole("figure", { name: TITLE })).toBeNull();
    expect(container.querySelector("svg.recharts-surface")).toBeNull();
  });

  // Escenario: US3-5 (20 sprints Cerrados legibles, SC-009)
  it("con 20 sprints Cerrados muestra cada nombre una vez con su par de valores", async () => {
    const names = Array.from({ length: 20 }, (_, i) => `Sprint ${i + 1}`);
    installDashboardBackend({
      series: {
        status: 200,
        body: seriesWith(
          names.map((name, i) => point(i + 1, name, 100 + i)),
          names.map((name, i) => point(i + 1, name, 200 + i)),
        ),
      },
    });
    const { container } = renderDashboard(defaultProjectId);

    const figure = await findFigure();
    const rows = dataRows(figure);
    expect(rows).toHaveLength(20);
    names.forEach((name, i) => {
      expect(within(rows[i]).getByText(name)).toBeDefined();
      expect(within(rows[i]).getByText(String(100 + i))).toBeDefined();
      expect(within(rows[i]).getByText(String(200 + i))).toBeDefined();
    });

    // Cada nombre aparece exactamente una vez como rótulo del eje X.
    const ticks = Array.from(
      container.querySelectorAll(
        ".recharts-xAxis-tick-labels .recharts-cartesian-axis-tick-value",
      ),
    ).map((tick) => tick.textContent);
    names.forEach((name) => {
      expect(ticks.filter((tick) => tick === name)).toHaveLength(1);
    });
  });

  // Escenario: US3-6 (un Cerrado con 0 completados muestra 0)
  it("un sprint Cerrado con 0 completados muestra 0 en su fila", async () => {
    installDashboardBackend({
      series: {
        status: 200,
        body: seriesWith(
          [point(1, "Sprint 1", 21), point(2, "Sprint 2", 24)],
          [point(1, "Sprint 1", 0), point(2, "Sprint 2", 21)],
        ),
      },
    });
    renderDashboard(defaultProjectId);

    const figure = await findFigure();
    const rows = dataRows(figure);
    expect(within(rows[0]).getByText("0")).toBeDefined();
  });

  // Escenario: US3-7 (plannedStoryPoints malformada: falla solo el gráfico)
  it("con plannedStoryPoints malformada muestra el error del gráfico y el resto de la vista", async () => {
    installDashboardBackend({
      series: {
        status: 200,
        body: { ...defaultSeries, plannedStoryPoints: "no es una lista" },
      },
    });
    renderDashboard(defaultProjectId);

    expect(await screen.findByText(ERROR)).toBeDefined();
    expect(screen.queryByRole("figure", { name: TITLE })).toBeNull();
    expect(
      await screen.findByRole("region", { name: "Resumen del proyecto" }),
    ).toBeDefined();
    expect(
      await screen.findByRole("region", { name: "Indicadores clave" }),
    ).toBeDefined();
  });
});
