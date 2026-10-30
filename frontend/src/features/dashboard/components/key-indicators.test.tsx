import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import {
  defaultActiveSprintMetrics,
  defaultMetrics,
  defaultProjectId,
  installDashboardBackend,
  renderDashboard,
} from "@/features/dashboard/components/dashboard-test-backend";
import { clearSession } from "@/lib/session-storage";
import type {
  ActiveSprintMetrics,
  Cumulative,
  ProjectMetricsResponse,
} from "@/types/metrics";

// Pruebas de escenario de la Historia 2 (T013): montan DashboardView completo con el backend de
// prueba compartido y verifican los indicadores clave contra los valores que entrega la feature de
// métricas, sin recalcular ninguno.

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

// Nombres de los grupos de indicadores y signo menos que usa el formato.
const SPRINT_GROUP = "Indicadores del sprint activo";
const PROJECT_GROUP = "Indicadores del proyecto";
const MINUS = "−";

// Métrica de sprint activo disponible y parcial.
function sprintMetric(value: number) {
  return { status: "available" as const, value, finality: "partial" as const };
}

// Métrica de sprint activo no calculable.
function sprintNotCalculable() {
  return {
    status: "not_calculable" as const,
    value: null,
    finality: "partial" as const,
  };
}

// Métricas del sprint activo con los cambios indicados.
function sprintMetricsWith(changes: Partial<ActiveSprintMetrics>) {
  return {
    activeSprint: true,
    metrics: { ...defaultActiveSprintMetrics.metrics!, ...changes },
  };
}

// Métricas del proyecto con los cambios indicados.
function projectMetricsWith(
  changes: Partial<ProjectMetricsResponse>,
): ProjectMetricsResponse {
  return { ...defaultMetrics, ...changes };
}

// Acumulado del proyecto con los cambios indicados.
function cumulativeWith(changes: Partial<Cumulative>): Cumulative {
  return { ...defaultMetrics.cumulative, ...changes };
}

// Región de indicadores, una vez cargada.
async function findIndicators() {
  return screen.findByRole("region", { name: "Indicadores clave" });
}

// Encabezado h3 de un grupo.
function groupHeading(region: HTMLElement, group: string): HTMLElement {
  return within(region).getByRole("heading", {
    level: 3,
    name: new RegExp(group),
  });
}

// Valor (dd) de un término dentro de un grupo: el dt posterior al encabezado del grupo y anterior
// al siguiente encabezado.
function valueOf(
  region: HTMLElement,
  group: string,
  term: string,
): HTMLElement {
  const headings: HTMLElement[] = Array.from(region.querySelectorAll("h3"));
  const start = groupHeading(region, group);
  const next = headings[headings.indexOf(start) + 1];
  const found = Array.from(region.querySelectorAll("dt")).find(
    (dt) =>
      dt.textContent?.trim() === term &&
      Boolean(
        start.compareDocumentPosition(dt) & Node.DOCUMENT_POSITION_FOLLOWING,
      ) &&
      (!next ||
        Boolean(
          next.compareDocumentPosition(dt) & Node.DOCUMENT_POSITION_PRECEDING,
        )),
  );
  expect(found, `${group} / ${term}`).toBeTruthy();
  const dd = found!.nextElementSibling as HTMLElement;
  expect(dd.tagName).toBe("DD");
  return dd;
}

// Texto del valor sin las marcas de finalidad.
function plain(element: HTMLElement): string {
  return (element.textContent ?? "").replace(/Parcial|Provisoria/g, "").trim();
}

// Cada prueba arranca sin sesiÃ³n y con el espÃ­a del enrutador limpio.
beforeEach(() => {
  clearSession();
  routerMock.replace.mockClear();
});

// Desmonta el componente y restablece la sesiÃ³n y los globales sustituidos.
afterEach(() => {
  cleanup();
  clearSession();
  vi.unstubAllGlobals();
});

// Escenarios de los indicadores clave del sprint activo y del proyecto.
describe("009/US2 - Ver los indicadores clave del momento", () => {
  // Escenario: US2-1 (Caso normal)
  it("muestra los indicadores con los valores de la feature de métricas", async () => {
    installDashboardBackend();
    renderDashboard(defaultProjectId);
    const region = await findIndicators();

    await waitFor(() => {
      expect(
        plain(valueOf(region, SPRINT_GROUP, "Story Points planificados")),
      ).toBe("18");
    });
    expect(
      plain(valueOf(region, SPRINT_GROUP, "Story Points completados")),
    ).toBe("5");
    expect(plain(valueOf(region, SPRINT_GROUP, "Historias completadas"))).toBe(
      "27,78 %",
    );
    expect(plain(valueOf(region, SPRINT_GROUP, "Horas estimadas"))).toBe(
      "108 h",
    );
    expect(plain(valueOf(region, SPRINT_GROUP, "Horas reales"))).toBe("40 h");
    expect(plain(valueOf(region, SPRINT_GROUP, "Desviación"))).toBe(
      `${MINUS}68 h`,
    );
    expect(plain(valueOf(region, SPRINT_GROUP, "Desviación porcentual"))).toBe(
      `${MINUS}62,96 %`,
    );
    // La velocidad es la informada (14,00), no el promedio de la serie de la misma respuesta.
    expect(plain(valueOf(region, PROJECT_GROUP, "Velocidad del equipo"))).toBe(
      "14,00",
    );
    expect(plain(valueOf(region, PROJECT_GROUP, "Defectos abiertos"))).toBe(
      "7",
    );
  });

  // Escenario: US2-2 (Caso alternativo)
  it("marca como parciales los indicadores del sprint activo y no la velocidad", async () => {
    installDashboardBackend();
    renderDashboard(defaultProjectId);
    const region = await findIndicators();

    const terms = [
      "Story Points planificados",
      "Story Points completados",
      "Historias completadas",
      "Horas estimadas",
      "Horas reales",
      "Desviación",
      "Desviación porcentual",
    ];
    await waitFor(() => {
      expect(valueOf(region, SPRINT_GROUP, terms[0]).textContent).toContain(
        "Parcial",
      );
    });
    for (const term of terms) {
      expect(valueOf(region, SPRINT_GROUP, term).textContent).toContain(
        "Parcial",
      );
    }
    expect(
      valueOf(region, PROJECT_GROUP, "Velocidad del equipo").textContent,
    ).not.toContain("Parcial");
  });

  // Escenario: US2-3 (Caso alternativo)
  it("muestra la marca de parcial del acumulado", async () => {
    installDashboardBackend();
    const view = renderDashboard(defaultProjectId);
    let region = await findIndicators();
    await waitFor(() => {
      expect(groupHeading(region, PROJECT_GROUP).textContent).toContain(
        "Parcial",
      );
    });
    view.unmount();
    cleanup();

    installDashboardBackend({
      metrics: {
        status: 200,
        body: projectMetricsWith({
          cumulative: cumulativeWith({ finality: "definitive" }),
        }),
      },
    });
    renderDashboard(defaultProjectId);
    region = await findIndicators();
    await waitFor(() => {
      expect(
        valueOf(region, PROJECT_GROUP, "Velocidad del equipo"),
      ).toBeTruthy();
    });
    expect(groupHeading(region, PROJECT_GROUP).textContent).not.toContain(
      "Parcial",
    );
  });

  // Escenario: US2-4 (Caso límite)
  it("muestra la velocidad no calculable con su explicación", async () => {
    installDashboardBackend({
      metrics: {
        status: 200,
        body: projectMetricsWith({
          velocity: { value: { status: "not_calculable", value: null } },
        }),
      },
    });
    renderDashboard(defaultProjectId);
    const region = await findIndicators();

    await waitFor(() => {
      expect(
        valueOf(region, PROJECT_GROUP, "Velocidad del equipo").textContent,
      ).toContain("No calculable");
    });
    const text =
      valueOf(region, PROJECT_GROUP, "Velocidad del equipo").textContent ?? "";
    expect(text).toContain("Todavía no hay sprints cerrados.");
    expect(text).not.toMatch(/\d/);
  });

  // Escenario: US2-5 (Caso límite)
  it("muestra la desviación porcentual no calculable y la absoluta como número", async () => {
    installDashboardBackend({
      activeSprintMetrics: {
        status: 200,
        body: sprintMetricsWith({
          estimatedHours: sprintMetric(0),
          deviationPercent: sprintNotCalculable(),
          deviationHours: sprintMetric(40),
        }),
      },
    });
    renderDashboard(defaultProjectId);
    const region = await findIndicators();

    await waitFor(() => {
      expect(
        valueOf(region, SPRINT_GROUP, "Desviación porcentual").textContent,
      ).toContain("No calculable");
    });
    expect(
      valueOf(region, SPRINT_GROUP, "Desviación porcentual").textContent,
    ).toContain("Las horas estimadas son 0.");
    expect(plain(valueOf(region, SPRINT_GROUP, "Desviación"))).toBe("+40 h");
  });

  // Escenario: US2-6 (Caso límite)
  it("muestra 0 defectos abiertos como cifra real", async () => {
    installDashboardBackend({
      metrics: {
        status: 200,
        body: projectMetricsWith({
          defects: {
            open: {
              total: 0,
              bySeverity: { critical: 0, high: 0, medium: 0, low: 0 },
            },
          },
        }),
      },
    });
    renderDashboard(defaultProjectId);
    const region = await findIndicators();

    await waitFor(() => {
      expect(plain(valueOf(region, PROJECT_GROUP, "Defectos abiertos"))).toBe(
        "0",
      );
    });
    expect(
      valueOf(region, PROJECT_GROUP, "Defectos abiertos").textContent,
    ).not.toContain("No calculable");
  });

  // Escenario: US2-7 (Caso de error)
  it("informa la falla de los indicadores y muestra el resto", async () => {
    installDashboardBackend({
      metrics: { status: 500, body: { error: "Error interno." } },
      activeSprintMetrics: { status: 500, body: { error: "Error interno." } },
    });
    renderDashboard(defaultProjectId);
    const region = await findIndicators();

    const sprintAlert = await within(region).findByText(
      "No se pudieron cargar los indicadores del sprint activo.",
    );
    expect(sprintAlert.closest('[role="alert"]')).toBeTruthy();
    const projectAlert = await within(region).findByText(
      "No se pudieron cargar los indicadores del proyecto.",
    );
    expect(projectAlert.closest('[role="alert"]')).toBeTruthy();

    // El resumen y el sprint activo se muestran igual.
    expect(
      screen.getByRole("region", { name: "Resumen del proyecto" }),
    ).toBeTruthy();
    expect(
      await screen.findByRole("region", { name: "Sprint activo" }),
    ).toBeTruthy();
  });

  // Sin etiqueta de historia para no alterar el conteo de escenarios (SC-005).
  it("muestra el porcentaje de historias no calculable con su explicación", async () => {
    installDashboardBackend({
      activeSprintMetrics: {
        status: 200,
        body: sprintMetricsWith({
          storyCompletionPercent: sprintNotCalculable(),
        }),
      },
    });
    renderDashboard(defaultProjectId);
    const region = await findIndicators();

    await waitFor(() => {
      expect(
        valueOf(region, SPRINT_GROUP, "Historias completadas").textContent,
      ).toContain("No calculable");
    });
    const text =
      valueOf(region, SPRINT_GROUP, "Historias completadas").textContent ?? "";
    expect(text).toContain("No hay historias planificadas.");
    expect(text).not.toMatch(/\d/);
  });
});
