import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { createElement } from "react";
import { ActiveSprintPanel } from "@/features/dashboard/components/active-sprint-panel";
import {
  defaultSprint,
  defaultSprintId,
  installDashboardBackend,
  sessionToken,
} from "@/features/dashboard/components/dashboard-test-backend";
import { clearSession, writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { SprintDetail } from "@/types/sprint";

// Pruebas unitarias de ActiveSprintPanel (T005): cubren RN-12, RN-13 y RN-14 con fetch sustituido
// por el backend de prueba compartido. El panel recibe el id del sprint activo y consulta el detalle
// del sprint (004) para mostrar los días restantes o vencidos tal como los entrega esa feature.

// Sustituye el enrutador de Next.js: la sesión lo usa fuera del App Router.
vi.mock("next/navigation", () => ({
  useRouter: () => ({
    replace: vi.fn(),
    push: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    refresh: vi.fn(),
    prefetch: vi.fn(),
  }),
  usePathname: () => "/",
  useSearchParams: () => new URLSearchParams(),
}));

// Monta el panel dentro del proveedor de sesión con la sesión de la pestaña.
function renderPanel(activeSprintId: string | null) {
  writeToken(sessionToken);
  return render(
    createElement(
      SessionProvider,
      null,
      createElement(ActiveSprintPanel, { activeSprintId }),
    ),
  );
}

// Arma un detalle de sprint con el cronograma indicado.
function sprintWith(schedule: SprintDetail["schedule"]): SprintDetail {
  return { ...defaultSprint, schedule };
}

beforeEach(() => {
  clearSession();
});

afterEach(() => {
  cleanup();
  clearSession();
  vi.unstubAllGlobals();
});

describe("Panel del sprint activo", () => {
  // Escenario: Proyecto sin sprint activo (RN-12)
  it("sin sprint activo lo informa sin error y sin consultar", async () => {
    const requests = installDashboardBackend();
    renderPanel(null);

    expect(
      await screen.findByText("No hay ningún sprint en curso."),
    ).toBeTruthy();
    expect(screen.queryByRole("alert")).toBeNull();
    expect(
      requests.filter((r) => r.path.startsWith("/api/v1/sprints/")),
    ).toHaveLength(0);
  });

  // Escenario: Sprint activo con días restantes (RN-13)
  it("muestra nombre, Sprint Goal, fechas y 10 días restantes", async () => {
    installDashboardBackend();
    renderPanel(defaultSprintId);

    expect(await screen.findByText("Sprint 5")).toBeTruthy();
    expect(
      screen.getByText("Entregar el tablero de métricas del proyecto."),
    ).toBeTruthy();
    expect(screen.getByText(/Inicio/).textContent).toContain("19/10/2026");
    expect(screen.getByText(/Fin/).textContent).toContain("08/11/2026");
    expect(screen.getByText("10 días restantes")).toBeTruthy();
  });

  // Escenario: Último día del sprint (RN-13)
  it("con 0 días restantes muestra 0 días restantes", async () => {
    installDashboardBackend({
      sprint: {
        status: 200,
        body: sprintWith({ remainingDays: 0, overdueDays: null }),
      },
    });
    renderPanel(defaultSprintId);

    expect(await screen.findByText("0 días restantes")).toBeTruthy();
  });

  // Escenario: Sprint vencido (RN-14)
  it("con sprint vencido lo informa en palabras y sin número negativo", async () => {
    installDashboardBackend({
      sprint: {
        status: 200,
        body: sprintWith({ remainingDays: null, overdueDays: 3 }),
      },
    });
    renderPanel(defaultSprintId);

    expect(await screen.findByText("Vencido hace 3 días")).toBeTruthy();
    const region = screen.getByRole("region", { name: "Sprint activo" });
    expect(region.textContent).not.toContain("−3");
    expect(region.textContent).not.toContain("-3");
  });

  // Escenario: Error al cargar el sprint activo
  it("con un 500 muestra el error con role alert", async () => {
    installDashboardBackend({
      sprint: { status: 500, body: { error: "Error interno." } },
    });
    renderPanel(defaultSprintId);

    await waitFor(() => {
      expect(screen.getByRole("alert").textContent).toContain(
        "No se pudo cargar el sprint activo.",
      );
    });
  });
});
