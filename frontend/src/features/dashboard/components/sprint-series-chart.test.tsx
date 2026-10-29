import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { installResizeObserverStub } from "@/features/dashboard/components/dashboard-test-backend";
import { SprintSeriesChart } from "@/features/dashboard/components/sprint-series-chart";
import type { Finality, SeriesPoint } from "@/types/metrics";

// Pruebas unitarias de SprintSeriesChart (T020): RN-07 a RN-10 y FR-025.

const TITLE = "Velocidad por sprint";
const EMPTY = "No hay sprints cerrados.";
const ERROR = "No se pudo cargar el gráfico de velocidad.";
const SINGLE = "Todavía no hay evolución que comparar.";

// Punto de serie con los valores indicados.
function point(
  id: string,
  name: string,
  value: number,
  finality: Finality = "definitive",
  sprintStatus: "active" | "closed" = "closed",
): SeriesPoint {
  return {
    sprintId: id,
    sprintName: name,
    sprintStatus,
    status: "available",
    value,
    finality,
  };
}

// Renderiza el gráfico con las dos series indicadas.
function renderChart(planned: SeriesPoint[], completed: SeriesPoint[]) {
  return render(
    <SprintSeriesChart
      title={TITLE}
      yAxisLabel="Story Points"
      kind="storyPoints"
      format={(value) => String(value)}
      emptyMessage={EMPTY}
      errorMessage={ERROR}
      series={[
        { name: "Story Points planificados", points: planned },
        { name: "Story Points completados", points: completed },
      ]}
    />,
  );
}

// Puntos en orden no alfabético ni por valor.
const PLANNED = [
  point("s2", "Sprint B", 30),
  point("s1", "Sprint C", 10),
  point("s3", "Sprint A", 20),
];
const COMPLETED = [
  point("s2", "Sprint B", 25),
  point("s1", "Sprint C", 8),
  point("s3", "Sprint A", 19),
];

describe("009/US3 - SprintSeriesChart", () => {
  beforeEach(() => {
    installResizeObserverStub();
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("RN-08: figure con título como nombre accesible, ejes y leyenda", () => {
    const { container } = renderChart(PLANNED, COMPLETED);
    const figure = screen.getByRole("figure", { name: TITLE });
    expect(within(figure).getByText(TITLE).tagName).toBe("FIGCAPTION");
    const svg = container.querySelector("svg");
    expect(svg).not.toBeNull();
    expect(svg!.textContent).toContain("Sprint");
    expect(svg!.textContent).toContain("Story Points");
    expect(
      within(figure).getAllByText("Story Points planificados").length,
    ).toBeGreaterThan(0);
    expect(
      within(figure).getAllByText("Story Points completados").length,
    ).toBeGreaterThan(0);
  });

  it("RN-07: la tabla tiene una fila por sprint en el orden recibido", () => {
    renderChart(PLANNED, COMPLETED);
    const table = screen.getByRole("table", { name: `Datos de ${TITLE}` });
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(3);
    expect(rows[0].textContent).toContain("Sprint B");
    expect(rows[0].textContent).toContain("30");
    expect(rows[0].textContent).toContain("25");
    expect(rows[1].textContent).toContain("Sprint C");
    expect(rows[2].textContent).toContain("Sprint A");
  });

  it("FR-025: un valor 0 aparece como 0 en su fila", () => {
    renderChart(
      [point("s1", "Sprint 1", 10), point("s2", "Sprint 2", 12)],
      [point("s1", "Sprint 1", 0), point("s2", "Sprint 2", 9)],
    );
    const table = screen.getByRole("table", { name: `Datos de ${TITLE}` });
    const row = within(table).getByRole("row", { name: /Sprint 1/ });
    expect(within(row).getByText("0")).toBeDefined();
  });

  it("finalidad: parcial y provisoria se marcan en la tabla y el eje X", () => {
    const { container } = renderChart(
      [
        point("s1", "Sprint 1", 10),
        point("s2", "Sprint 2", 12, "partial", "active"),
        point("s3", "Sprint 3", 14, "provisional"),
      ],
      [
        point("s1", "Sprint 1", 8),
        point("s2", "Sprint 2", 5, "partial", "active"),
        point("s3", "Sprint 3", 7, "provisional"),
      ],
    );
    const table = screen.getByRole("table", { name: `Datos de ${TITLE}` });
    const rows = within(table).getAllByRole("row").slice(1);
    expect(within(rows[1]).getAllByText("Parcial").length).toBeGreaterThan(0);
    expect(within(rows[2]).getAllByText("Provisoria").length).toBeGreaterThan(
      0,
    );
    expect(container.querySelector("svg")!.textContent).toContain(
      "Sprint 2 (parcial)",
    );
  });

  it("RN-09: con un punto dibuja el gráfico y avisa que no hay evolución", () => {
    const { container } = renderChart(
      [point("s1", "Sprint 1", 10)],
      [point("s1", "Sprint 1", 8)],
    );
    expect(container.querySelector("svg")).not.toBeNull();
    expect(screen.getByText(SINGLE)).toBeDefined();
  });

  it("RN-10: con cero puntos no hay SVG ni tabla y se muestra el mensaje vacío", () => {
    const { container } = renderChart([], []);
    expect(screen.getByText(EMPTY)).toBeDefined();
    expect(container.querySelector("svg")).toBeNull();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("series con sprints distintos muestran el mensaje de falla", () => {
    const { container } = renderChart(PLANNED, [
      point("s2", "Sprint B", 25),
      point("s1", "Sprint C", 8),
      point("s9", "Sprint Z", 19),
    ]);
    expect(screen.getByText(ERROR)).toBeDefined();
    expect(container.querySelector("svg")).toBeNull();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("series con distinto orden muestran el mensaje de falla sin reordenar", () => {
    const { container } = renderChart(PLANNED, [...COMPLETED].reverse());
    expect(screen.getByText(ERROR)).toBeDefined();
    expect(container.querySelector("svg")).toBeNull();
    expect(screen.queryByRole("table")).toBeNull();
  });
});
