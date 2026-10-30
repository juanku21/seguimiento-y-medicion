import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { DashboardWarnings } from "@/features/dashboard/components/dashboard-warnings";
import type { MetricsWarning } from "@/types/metrics";

// Pruebas unitarias de la Historia 6 (T032, RN-21): las advertencias se separan en dos listas por
// kind, con roles distintos, y cada message se muestra tal como llega.

const PLANNING_MESSAGE = "El sprint activo no tiene Story Points estimados.";
const INCONSISTENCY_MESSAGE =
  "Hay 2 tareas Terminadas sin horas registradas (casos: 3 y 5).";

const planning: MetricsWarning = {
  kind: "planning",
  code: "sprint_without_estimates",
  message: PLANNING_MESSAGE,
};
const inconsistency: MetricsWarning = {
  kind: "data_inconsistency",
  code: "finished_without_hours",
  message: INCONSISTENCY_MESSAGE,
};

afterEach(() => {
  cleanup();
});

describe("Usar el dashboard cuando falta información o falla una sección", () => {
  it("Advertencias de planificación y de inconsistencia de datos", () => {
    render(<DashboardWarnings warnings={[planning, inconsistency]} />);

    const region = screen.getByRole("region", { name: "Advertencias" });
    const status = within(region).getByRole("status", {
      name: "Avisos de planificación",
    });
    const alert = within(region).getByRole("alert", {
      name: "Datos que hay que revisar",
    });

    // Cada lista muestra su mensaje sin cambios y no el de la otra.
    expect(within(status).getByText(PLANNING_MESSAGE)).toBeTruthy();
    expect(within(status).queryByText(INCONSISTENCY_MESSAGE)).toBeNull();
    expect(within(alert).getByText(INCONSISTENCY_MESSAGE)).toBeTruthy();
    expect(within(alert).queryByText(PLANNING_MESSAGE)).toBeNull();

    // El texto introductorio explica que hay información rota que alguien debe revisar.
    expect(within(region).getByText(/información rota/i)).toBeTruthy();
    expect(within(region).getByText(/revis/i, { selector: "p" })).toBeTruthy();
  });

  it("Solo advertencias de planificación no muestra alerta", () => {
    render(<DashboardWarnings warnings={[planning]} />);

    const region = screen.getByRole("region", { name: "Advertencias" });
    expect(
      within(region).getByRole("status", { name: "Avisos de planificación" }),
    ).toBeTruthy();
    expect(within(region).getByText(PLANNING_MESSAGE)).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText(/información rota/i)).toBeNull();
  });

  it("Sin advertencias no se muestra la región", () => {
    const { container } = render(<DashboardWarnings warnings={[]} />);

    expect(screen.queryByRole("region", { name: "Advertencias" })).toBeNull();
    expect(container.textContent).toBe("");
  });
});
