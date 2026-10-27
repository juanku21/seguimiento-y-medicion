import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { StoryDefectsSection } from "@/features/defects/components/story-defects-section";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { Defect, StoryDefects } from "@/types/defect";

// Pruebas de la sección "Defectos" del detalle de una historia (T040, SC-003).
//
// Contrato que fijan estas pruebas para StoryDefectsSection:
// - Recibe projectId y storyId; pide GET /api/v1/stories/{storyId}/defects.
// - Lista los defectos de la historia (role="listitem") con descripción, severidad y estado en
//   español, y siempre ofrece un enlace "Registrar defecto" a
//   /projects/{projectId}/defects/new?storyId={storyId}, que deja la historia preseleccionada.
// - Sin defectos muestra "Todavía no hay defectos registrados." sin role="alert".

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

type FetchMock = Mock<typeof fetch>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const storyId = "3f0c8a52-6a0e-4d1b-9a77-2f7e5b1c9d10";

const user = {
  id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
  fullName: "Ana Pérez",
};

const storyDefect: Defect = {
  id: "d1111111-0000-4000-8000-000000000001",
  projectId,
  description: "Total sin descuentos",
  severity: "high",
  status: "open",
  story: { id: storyId, title: "Reporte de ventas mensual" },
  detectedSprint: {
    id: "c2222222-0000-4000-8000-000000000002",
    name: "Sprint 2",
    status: "closed",
    startDate: "2026-09-15",
  },
  resolvedSprint: null,
  createdBy: user,
  updatedBy: user,
  statusChangedAt: "2026-10-24T13:00:00Z",
  statusChangedBy: user,
  createdAt: "2026-10-24T13:00:00Z",
  updatedAt: "2026-10-24T13:00:00Z",
};

// Respuesta 200 del listado de la historia con el cuerpo dado.
function listResponse(body: StoryDefects) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

function renderSection() {
  return render(
    <SessionProvider>
      <StoryDefectsSection projectId={projectId} storyId={storyId} />
    </SessionProvider>,
  );
}

describe("007/US3 - Consultar y filtrar los defectos", () => {
  let fetchMock: FetchMock;

  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>();
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Escenario: defectos de una historia. La sección lista los de la historia y pide la ruta de la
  // historia.
  it("la sección de la historia lista sus defectos", async () => {
    fetchMock.mockResolvedValue(
      listResponse({ defects: [storyDefect], emptyReason: null }),
    );
    renderSection();

    const items = await screen.findAllByRole("listitem");
    expect(items).toHaveLength(1);
    expect(items[0].textContent).toContain("Total sin descuentos");
    expect(items[0].textContent).toContain("Alta");
    expect(items[0].textContent).toContain("Abierto");

    const [input] = fetchMock.mock.calls[0];
    const raw =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url;
    expect(new URL(raw, "http://localhost").pathname).toBe(
      `/api/v1/stories/${storyId}/defects`,
    );
  });

  // Escenario: SC-003. La sección ofrece "Registrar defecto" con la historia preseleccionada.
  it("ofrece Registrar defecto con la historia preseleccionada", async () => {
    fetchMock.mockResolvedValue(
      listResponse({ defects: [storyDefect], emptyReason: null }),
    );
    renderSection();
    await screen.findAllByRole("listitem");

    const link = screen.getByRole("link", { name: /registrar defecto/i });
    expect(link.getAttribute("href")).toBe(
      `/projects/${projectId}/defects/new?storyId=${storyId}`,
    );
  });

  // Escenario: historia sin defectos. Aviso de que no hay ninguno, sin error, y el enlace para
  // registrar el primero sigue disponible.
  it("la historia sin defectos muestra el aviso y conserva el enlace para registrar", async () => {
    fetchMock.mockResolvedValue(
      listResponse({ defects: [], emptyReason: "no_defects" }),
    );
    renderSection();

    expect(
      await screen.findByText("Todavía no hay defectos registrados."),
    ).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(
      screen
        .getByRole("link", { name: /registrar defecto/i })
        .getAttribute("href"),
    ).toBe(`/projects/${projectId}/defects/new?storyId=${storyId}`);
  });
});
