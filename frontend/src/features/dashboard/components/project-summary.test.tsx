import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import {
  defaultProject,
  defaultProjectId,
  defaultSprint,
  installDashboardBackend,
  renderDashboard,
} from "@/features/dashboard/components/dashboard-test-backend";
import { clearSession } from "@/lib/session-storage";
import type { ProjectDetail } from "@/types/project";
import type { SprintDetail } from "@/types/sprint";

// Pruebas de escenario de la Historia 1 (T006): montan DashboardView completo con el backend de
// prueba compartido y verifican el resumen del proyecto y el sprint activo.

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

// Arma un detalle de sprint con el cronograma indicado.
function sprintWith(
  schedule: SprintDetail["schedule"],
  extra: Partial<SprintDetail> = {},
): SprintDetail {
  return { ...defaultSprint, ...extra, schedule };
}

// Arma un proyecto a partir del de prueba con los cambios indicados.
function projectWith(changes: Partial<ProjectDetail>): ProjectDetail {
  return { ...defaultProject, ...changes };
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

// Escenarios del resumen del proyecto y del sprint en curso.
describe("009/US1 - Ver el estado del proyecto y del sprint en curso", () => {
  // Escenario: US1-1 (resumen y sprint activo con 10 días restantes)
  it("muestra el resumen y el sprint activo con 10 días restantes", async () => {
    installDashboardBackend();
    renderDashboard(defaultProjectId);

    const summary = await screen.findByRole("region", {
      name: "Resumen del proyecto",
    });
    expect(within(summary).getByText("Plataforma de seguimiento")).toBeTruthy();
    expect(within(summary).getByText(/En curso/)).toBeTruthy();
    expect(summary.textContent).toContain("Inicio");
    expect(summary.textContent).toContain("01/09/2026");
    expect(summary.textContent).toContain("Finalización prevista");
    expect(summary.textContent).toContain("15/12/2026");
    for (const name of [
      "Ana Pérez",
      "Bruno Díaz",
      "Carla Gómez",
      "Diego Ruiz",
    ]) {
      expect(within(summary).getByText(name)).toBeTruthy();
    }

    const sprint = await screen.findByRole("region", { name: "Sprint activo" });
    await within(sprint).findByText("10 días restantes");
    expect(within(sprint).getByText("Sprint 5")).toBeTruthy();
    expect(
      within(sprint).getByText("Entregar el tablero de métricas del proyecto."),
    ).toBeTruthy();
    expect(sprint.textContent).toContain("19/10/2026");
    expect(sprint.textContent).toContain("08/11/2026");
  });

  // Escenario: US1-2 (proyecto finalizado con su fecha real y sin sprint en curso)
  it("muestra el proyecto finalizado con su fecha real y sin sprint en curso", async () => {
    installDashboardBackend({
      project: {
        status: 200,
        body: projectWith({
          status: "finished",
          readOnly: true,
          actualEndDate: "2026-10-20",
          hasActiveSprint: false,
          activeSprint: null,
        }),
      },
    });
    renderDashboard(defaultProjectId);

    const summary = await screen.findByRole("region", {
      name: "Resumen del proyecto",
    });
    expect(summary.textContent).toContain("Finalización real");
    expect(summary.textContent).toContain("20/10/2026");
    expect(
      await screen.findByText("No hay ningún sprint en curso."),
    ).toBeTruthy();
  });

  // Escenario: US1-3 (sin sprint en curso no es un error)
  it("informa que no hay sprint en curso sin tratarlo como error", async () => {
    installDashboardBackend({
      project: {
        status: 200,
        body: projectWith({ hasActiveSprint: false, activeSprint: null }),
      },
    });
    renderDashboard(defaultProjectId);

    expect(
      await screen.findByRole("region", { name: "Resumen del proyecto" }),
    ).toBeTruthy();
    expect(
      await screen.findByText("No hay ningún sprint en curso."),
    ).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Escenario: US1-4 (último día del sprint)
  it("muestra 0 días restantes el último día del sprint", async () => {
    installDashboardBackend({
      sprint: {
        status: 200,
        body: sprintWith({ remainingDays: 0, overdueDays: null }),
      },
    });
    renderDashboard(defaultProjectId);

    expect(await screen.findByText("0 días restantes")).toBeTruthy();
  });

  // Escenario: US1-5 (sprint vencido y por cuánto)
  it("muestra el sprint vencido y por cuánto", async () => {
    installDashboardBackend({
      sprint: {
        status: 200,
        body: sprintWith({ remainingDays: null, overdueDays: 3 }),
      },
    });
    renderDashboard(defaultProjectId);

    expect(await screen.findByText("Vencido hace 3 días")).toBeTruthy();
  });

  // Escenario: US1-6 (días restantes de la feature de sprints, sin restar fechas)
  it("muestra los días restantes que entrega la feature de sprints", async () => {
    installDashboardBackend({
      sprint: {
        status: 200,
        // Fin lejano: restar fechas con hoy nunca daría 7.
        body: sprintWith(
          { remainingDays: 7, overdueDays: null },
          { endDate: "2027-03-01" },
        ),
      },
    });
    renderDashboard(defaultProjectId);

    expect(await screen.findByText("7 días restantes")).toBeTruthy();
  });

  // Escenario: US1-7 (un único integrante sin estado especial)
  it("muestra un único integrante sin estado especial", async () => {
    installDashboardBackend({
      project: {
        status: 200,
        body: projectWith({ members: [defaultProject.members[0]] }),
      },
    });
    renderDashboard(defaultProjectId);

    const summary = await screen.findByRole("region", {
      name: "Resumen del proyecto",
    });
    expect(within(summary).getByText("Ana Pérez")).toBeTruthy();
    expect(within(summary).queryByText("Bruno Díaz")).toBeNull();
    expect(summary.textContent).not.toMatch(/único|solo|sin integrantes/i);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Escenario: US1-8 (el proyecto no existe)
  it("informa que el proyecto no existe", async () => {
    installDashboardBackend({
      project: { status: 404, body: { error: "El proyecto no existe." } },
    });
    renderDashboard(defaultProjectId);

    await waitFor(() => {
      expect(screen.getByRole("alert").textContent).toContain(
        "El proyecto no existe.",
      );
    });
    expect(
      screen.queryByRole("region", { name: "Resumen del proyecto" }),
    ).toBeNull();
    expect(screen.queryByRole("region", { name: "Sprint activo" })).toBeNull();
  });

  // Escenario: US1-9 (quien no es integrante recibe lo mismo que ante un proyecto inexistente)
  it("responde a quien no es integrante igual que a un proyecto inexistente", async () => {
    // Proyecto ajeno: el backend responde 404 a la consulta del resumen.
    installDashboardBackend({
      project: { status: 404, body: { error: "El proyecto no existe." } },
    });
    const foreign = renderDashboard(defaultProjectId);
    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeTruthy();
    });
    const foreignText = foreign.container.textContent;
    cleanup();

    // Proyecto inexistente: otro identificador que el backend de prueba no conoce.
    installDashboardBackend();
    const missing = renderDashboard("00000000-0000-4000-8000-000000000000");
    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeTruthy();
    });

    expect(missing.container.textContent).toBe(foreignText);
    expect(foreignText).toContain("El proyecto no existe.");
  });

  // Sin etiqueta de escenario (SC-015, RN-18).
  it("con el 404 del resumen no hace ninguna otra consulta", async () => {
    const requests = installDashboardBackend({
      project: { status: 404, body: { error: "El proyecto no existe." } },
    });
    renderDashboard(defaultProjectId);

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeTruthy();
    });
    expect(requests).toEqual([
      { method: "GET", path: `/api/v1/projects/${defaultProjectId}` },
    ]);
  });

  // Sin etiqueta de escenario (SC-014).
  it("sin sesión lleva al inicio de sesión", async () => {
    const requests = installDashboardBackend();
    // renderDashboard guarda el token; se lo borra antes del montaje efectivo con un render propio.
    const { createElement } = await import("react");
    const { render } = await import("@testing-library/react");
    const { SessionProvider } = await import("@/providers/session-provider");
    const { DashboardView } =
      await import("@/features/dashboard/components/dashboard-view");
    render(
      createElement(
        SessionProvider,
        null,
        createElement(DashboardView, { projectId: defaultProjectId }),
      ),
    );

    await waitFor(() => {
      expect(routerMock.replace).toHaveBeenCalledWith("/login");
    });
    expect(requests).toHaveLength(0);
  });
});
