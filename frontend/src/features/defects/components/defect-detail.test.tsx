import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { DefectDetail } from "@/features/defects/components/defect-detail";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { Defect, DefectStatus } from "@/types/defect";

// Pruebas del detalle de un defecto Descartado (T065). Atraviesan DefectDetail, el cliente HTTP real
// y un fetch simulado. Contrato: un Descartado muestra sus datos y su estado, pero sin botones de
// estado ni formulario de corrección (sin campos editables ni "Guardar cambios").

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    refresh: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    prefetch: vi.fn(),
  }),
  useSearchParams: () => new URLSearchParams(""),
}));

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJicnVubyJ9.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const defectId = "7d8e9f0a-1b2c-4d3e-8f4a-5b6c7d8e9f0a";
const author = {
  id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
  fullName: "Ana Pérez",
};
const description = "El total del reporte no suma los descuentos";

// Defecto detectado en un sprint cerrado, en el estado indicado.
function defect(status: DefectStatus): Defect {
  return {
    id: defectId,
    projectId,
    description,
    severity: "high",
    status,
    story: {
      id: "3f0c8a52-6a0e-4d1b-9a77-2f7e5b1c9d10",
      title: "Reporte de ventas mensual",
    },
    detectedSprint: {
      id: "c2222222-0000-4000-8000-000000000002",
      name: "Sprint 2",
      status: "closed",
      startDate: "2026-09-15",
    },
    resolvedSprint: null,
    createdBy: author,
    updatedBy: author,
    statusChangedAt: "2026-10-24T13:00:00Z",
    statusChangedBy: author,
    createdAt: "2026-10-20T10:00:00Z",
    updatedAt: "2026-10-24T13:00:00Z",
  };
}

// Sustituye fetch: devuelve el defecto con el estado indicado y listas vacías para el resto.
function mockServer(status: DefectStatus) {
  vi.stubGlobal(
    "fetch",
    vi.fn<typeof fetch>(async (input) => {
      const raw =
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.href
            : input.url;
      const url = new URL(raw, "http://localhost");
      const body = url.pathname.endsWith(`/api/v1/defects/${defectId}`)
        ? defect(status)
        : { stories: [], sprints: [], emptyReason: "no_matches" };
      return new Response(JSON.stringify(body), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }),
  );
}

describe("007/US6 - Descartar un defecto que no corresponde", () => {
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Escenario: un Descartado se muestra sin acciones ni campos editables (US6, FR-018, SC-012).
  it("un defecto Descartado se muestra sin acciones ni campos editables", async () => {
    mockServer("discarded");
    render(
      <SessionProvider>
        <DefectDetail defectId={defectId} />
      </SessionProvider>,
    );

    expect(await screen.findByText(/^Descartado/)).toBeTruthy();
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.getByText(description)).toBeTruthy();
  });

  // Contraste: un Abierto sí ofrece acciones y formulario, así la prueba anterior no pasa en vacío.
  it("un defecto Abierto sí ofrece acciones y campos editables", async () => {
    mockServer("open");
    render(
      <SessionProvider>
        <DefectDetail defectId={defectId} />
      </SessionProvider>,
    );

    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: /guardar cambios/i }),
      ).toBeTruthy(),
    );
    expect(
      screen.getByRole("button", { name: /^pasar a en progreso$/i }),
    ).toBeTruthy();
  });
});
