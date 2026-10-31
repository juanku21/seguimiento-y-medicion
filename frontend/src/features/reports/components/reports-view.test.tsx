import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { createElement } from "react";
import {
  defaultProjectId,
  defaultReportSprint,
  defaultSprintId,
  installReportsBackend,
  renderReports,
  sprintReport,
} from "@/features/reports/components/reports-test-backend";
import { ReportsView } from "@/features/reports/components/reports-view";
import { clearSession } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { SprintMetric } from "@/types/metrics";
import type { Report } from "@/types/report";
import type { SprintListItem, SprintListResponse } from "@/types/sprint";

// Pruebas de escenario de la Historia 1 (T014): montan ReportsView completo con el backend de
// prueba compartido, eligen el sprint, generan el reporte y verifican el article "Reporte". El
// módulo export-pdf todavía no existe (llega con la Historia 2), así que no se sustituye aquí.

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

// Orden fijo de las seis regiones del reporte (contracts/ui-contract.md).
const REGIONS = [
  "Encabezado",
  "Historias",
  "Estimaciones",
  "Esfuerzo",
  "Métricas",
  "Defectos",
];
const PARTIAL_NOTE = "Reporte parcial: incluye un sprint en curso.";

// Cada prueba arranca sin sesión y con el espía del enrutador limpio.
beforeEach(() => {
  clearSession();
  routerMock.replace.mockClear();
});

// Desmonta el componente y restablece la sesión y los globales sustituidos.
afterEach(() => {
  cleanup();
  clearSession();
  vi.unstubAllGlobals();
});

// Arma el listado de sprints con un sprint de cada estado indicado.
function sprintsWith(statuses: ("active" | "closed")[]): SprintListResponse {
  return {
    sprints: statuses.map((status, index): SprintListItem => ({
      id:
        index === 0
          ? defaultSprintId
          : `b${index}000000-0000-4000-8000-00000000000${index}`,
      name: `Sprint ${index + 1}`,
      goal: "Meta del sprint.",
      status,
      startDate: "2026-09-07",
      endDate: "2026-09-27",
      startedAt: "2026-09-07T09:00:00Z",
      closedAt: status === "closed" ? "2026-09-27T18:00:00Z" : null,
      storyCount: 4,
      results: null,
    })),
    appliedFilters: { status: [] },
    emptyReason: null,
  };
}

// Métrica no calculable del sprint.
const notCalculable: SprintMetric = {
  status: "not_calculable",
  value: null,
  finality: "definitive",
};

// Monta la vista, elige el sprint por su opción, pulsa "Generar reporte" y espera el article.
async function generate(optionName: string) {
  renderReports(defaultProjectId);
  const scope = await screen.findByRole("combobox", {
    name: "Alcance del reporte",
  });
  await screen.findByRole("option", { name: optionName });
  const option = screen.getByRole("option", { name: optionName });
  fireEvent.change(scope, { target: { value: option.getAttribute("value") } });
  fireEvent.click(screen.getByRole("button", { name: "Generar reporte" }));
}

// Espera el article "Reporte" y lo devuelve.
async function findReport() {
  return screen.findByRole("article", { name: "Reporte" });
}

// Devuelve una de las regiones del reporte por su nombre accesible.
function region(report: HTMLElement, name: string) {
  return within(report).getByRole("region", { name });
}

describe("010/US1 - Generar y ver el reporte de un sprint", () => {
  // Escenario: US1-1 (Caso normal)
  it("muestra las seis secciones del reporte del sprint cerrado", async () => {
    installReportsBackend();
    await generate("Sprint 1 (Cerrado)");
    const report = await findReport();

    expect(
      within(report).getByRole("heading", {
        level: 2,
        name: "Reporte del sprint Sprint 1",
      }),
    ).toBeTruthy();
    // Las seis regiones, en el orden fijo, cada una con su h3.
    expect(
      within(report)
        .getAllByRole("region")
        .map((r) => within(r).getByRole("heading", { level: 3 }).textContent),
    ).toEqual(REGIONS);

    // Encabezado con proyecto, alcance, período, generación y autor.
    const header = region(report, "Encabezado");
    expect(within(header).getByText("Plataforma de seguimiento")).toBeTruthy();
    expect(within(header).getByText("Sprint Sprint 1")).toBeTruthy();
    expect(within(header).getByText("07/09/2026 al 27/09/2026")).toBeTruthy();
    expect(within(header).getByText("Generado el")).toBeTruthy();
    expect(within(header).getByText("Ana Pérez")).toBeTruthy();

    // Historias: las 4 con sus Story Points y estados.
    const stories = region(report, "Historias");
    ["Tablero", "Exportación", "Ayuda", "Perfil"].forEach((title) =>
      expect(within(stories).getByText(title)).toBeTruthy(),
    );
    expect(within(stories).getByText("Sin estimar")).toBeTruthy();
    expect(within(stories).getAllByText("Completada")).toHaveLength(2);

    // Estimaciones, esfuerzo, métricas y defectos.
    expect(
      within(region(report, "Estimaciones")).getByText("Tablero"),
    ).toBeTruthy();
    const effort = region(report, "Esfuerzo");
    expect(within(effort).getByText("Bruno Díaz")).toBeTruthy();
    expect(within(effort).getByText("Total registrado: 50,00")).toBeTruthy();
    expect(
      within(region(report, "Métricas")).getByText("Desviación porcentual"),
    ).toBeTruthy();
    const defects = region(report, "Defectos");
    expect(within(defects).getByText("Detectados: 3")).toBeTruthy();
    expect(within(defects).getByText("Resueltos: 1")).toBeTruthy();
  });

  // Escenario: US1-2 (Caso alternativo)
  it("marca como parcial el reporte del sprint activo", async () => {
    installReportsBackend({
      sprints: { status: 200, body: sprintsWith(["active"]) },
      sprintReport: {
        status: 200,
        body: sprintReport({
          partial: true,
          header: {
            ...sprintReport().header,
            sprint: {
              ...defaultReportSprint,
              status: "active",
              closedAt: null,
            },
          },
        }),
      },
    });
    await generate("Sprint 1 (Activo)");
    const report = await findReport();

    expect(within(report).getByRole("status").textContent).toBe(PARTIAL_NOTE);
  });

  // Escenario: US1-3 (Caso alternativo)
  it("muestra el resumen de Planning Poker con 3 rondas y valor acordado 8", async () => {
    const base = sprintReport();
    const groups = base.estimations.groups ?? [];
    installReportsBackend({
      sprintReport: {
        status: 200,
        body: sprintReport({
          estimations: {
            state: "complete",
            groups: [
              {
                ...groups[0],
                items: groups[0].items.map((item, index) =>
                  index === 0
                    ? {
                        ...item,
                        poker: "agreed" as const,
                        session: {
                          rounds: 3,
                          agreedStoryPoints: 8,
                          finishedAt: "2026-09-08T10:00:00Z",
                        },
                      }
                    : item,
                ),
              },
            ],
          },
        }),
      },
    });
    await generate("Sprint 1 (Cerrado)");
    const report = await findReport();

    expect(
      within(region(report, "Estimaciones")).getByText(
        "3 rondas, valor acordado 8",
      ),
    ).toBeTruthy();
  });

  // Escenario: US1-4 (Caso alternativo)
  it("indica que la historia no tuvo sesión de estimación", async () => {
    installReportsBackend();
    await generate("Sprint 1 (Cerrado)");
    const report = await findReport();

    expect(
      within(region(report, "Estimaciones")).getAllByText(
        "No hubo sesión de estimación",
      ),
    ).toHaveLength(4);
  });

  // Escenario: US1-5 (Caso límite)
  it("lista las historias planificadas sin ninguna completada", async () => {
    const base = sprintReport();
    const groups = base.stories.groups ?? [];
    installReportsBackend({
      sprintReport: {
        status: 200,
        body: sprintReport({
          stories: {
            state: "complete",
            groups: [
              {
                ...groups[0],
                stories: groups[0].stories.map((story) => ({
                  ...story,
                  status: "not_completed" as const,
                })),
              },
            ],
          },
        }),
      },
    });
    await generate("Sprint 1 (Cerrado)");
    const report = await findReport();

    const stories = region(report, "Historias");
    expect(within(stories).getByText("Tablero")).toBeTruthy();
    expect(within(stories).getByText("Perfil")).toBeTruthy();
    const cells = within(stories).getAllByRole("cell");
    expect(cells.filter((c) => c.textContent === "Completada")).toHaveLength(0);
    expect(within(stories).getAllByText("No completada")).toHaveLength(4);
  });

  // Escenario: US1-6 (Caso límite)
  it("muestra sin datos en las secciones que dependen de historias", async () => {
    const base = sprintReport();
    const sprint = base.header.sprint ?? defaultReportSprint;
    installReportsBackend({
      sprintReport: {
        status: 200,
        body: sprintReport({
          stories: {
            state: "complete",
            groups: [{ sprint, state: "empty", stories: [] }],
          },
          estimations: {
            state: "complete",
            groups: [{ sprint, state: "empty", items: [] }],
          },
        }),
      },
    });
    await generate("Sprint 1 (Cerrado)");
    const report = await findReport();

    expect(
      within(region(report, "Historias")).getByText("Sin datos."),
    ).toBeTruthy();
    expect(
      within(region(report, "Estimaciones")).getByText("Sin datos."),
    ).toBeTruthy();
    expect(
      within(region(report, "Encabezado")).getByText("Generado por"),
    ).toBeTruthy();
    expect(
      within(region(report, "Métricas")).getByText("Story Points planificados"),
    ).toBeTruthy();
  });

  // Escenario: US1-7 (Caso límite)
  it("muestra el valor no calculable con su explicación y nunca 0", async () => {
    const base = sprintReport();
    const metrics: Report["metrics"] = {
      ...base.metrics,
      sprint: base.metrics.sprint && {
        ...base.metrics.sprint,
        estimatedHours: {
          status: "available",
          value: 0,
          finality: "definitive",
        },
        deviationPercent: notCalculable,
      },
    };
    installReportsBackend({
      sprintReport: { status: 200, body: sprintReport({ metrics }) },
    });
    await generate("Sprint 1 (Cerrado)");
    const report = await findReport();

    const row = within(region(report, "Métricas"))
      .getByText("Desviación porcentual")
      .closest("tr");
    expect(row).not.toBeNull();
    const cells = within(row as HTMLElement).getAllByRole("cell");
    expect(
      cells.some((c) => c.textContent?.startsWith("No calculable: ")),
    ).toBe(true);
    expect(cells.some((c) => /^0(?:[,.]0+)?\b/.test(c.textContent ?? ""))).toBe(
      false,
    );
  });

  // Escenario: US1-8 (Caso de error)
  it("informa que un sprint planificado no tiene nada que reportar", async () => {
    const message = "El sprint todavía no arrancó: no tiene nada que reportar.";
    installReportsBackend({
      sprintReport: { status: 409, body: { error: message } },
    });
    await generate("Sprint 1 (Cerrado)");

    expect((await screen.findByRole("alert")).textContent).toBe(message);
    expect(screen.queryByRole("article", { name: "Reporte" })).toBeNull();
  });

  // Escenario: US1-9 (Caso de error)
  it("responde a un sprint de otro proyecto como inexistente", async () => {
    installReportsBackend({
      sprintReport: { status: 404, body: { error: "El sprint no existe." } },
    });
    await generate("Sprint 1 (Cerrado)");

    expect((await screen.findByRole("alert")).textContent).toBe(
      "El sprint no existe.",
    );
    expect(screen.queryByRole("article", { name: "Reporte" })).toBeNull();
  });

  // Escenario: US1-10 (Caso de error)
  it("responde a quien no es integrante como proyecto inexistente", async () => {
    installReportsBackend();
    renderReports("1f2e3d4c-0000-4000-8000-00000000ffff");

    expect((await screen.findByRole("alert")).textContent).toBe(
      "El proyecto no existe.",
    );
    expect(screen.queryByRole("combobox")).toBeNull();
  });

  // Sin etiqueta de escenario: el selector solo ofrece Activo y Cerrados.
  it("no ofrece sprints planificados y pide solo activos y cerrados", async () => {
    installReportsBackend({
      sprints: { status: 200, body: sprintsWith(["active", "closed"]) },
    });
    renderReports(defaultProjectId);
    await screen.findByRole("option", { name: "Sprint 1 (Activo)" });

    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Proyecto completo",
      "Sprint 1 (Activo)",
      "Sprint 2 (Cerrado)",
    ]);
    const calls = vi.mocked(fetch).mock.calls.map(([input]) => String(input));
    expect(
      calls.some((url) => url.includes("status=active&status=closed")),
    ).toBe(true);
  });

  // Sin etiqueta de escenario (SC-014).
  it("sin sesión lleva al inicio de sesión", async () => {
    const requests = installReportsBackend();
    // renderReports guarda el token; aquí se monta la vista sin sesión.
    render(
      createElement(
        SessionProvider,
        null,
        createElement(ReportsView, { projectId: defaultProjectId }),
      ),
    );

    await waitFor(() => {
      expect(routerMock.replace).toHaveBeenCalledWith("/login");
    });
    expect(requests).toHaveLength(0);
  });
});
