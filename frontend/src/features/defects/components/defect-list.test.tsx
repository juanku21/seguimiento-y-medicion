import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { DefectList } from "@/features/defects/components/defect-list";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type {
  Defect,
  DefectFilterValues,
  ProjectDefects,
} from "@/types/defect";

// Pruebas del listado de defectos del proyecto (T040). Atraviesan el componente y el cliente HTTP
// real; solo se sustituye fetch.
//
// Contrato que fijan estas pruebas para DefectList:
// - Recibe projectId y filters; pide GET /api/v1/projects/{projectId}/defects con los filtros.
// - Muestra una fila (role="listitem") por defecto, en el orden recibido, con la descripción, la
//   severidad y el estado en español, el título de la historia, "Detectado en <sprint>" y, solo si
//   tiene sprint de resolución, "Resuelto en <sprint>".
// - Con la lista vacía muestra, sin role="alert", el texto de DEFECT_EMPTY_REASON_LABELS según
//   emptyReason: "Todavía no hay defectos registrados." (no_defects) u "Ningún defecto cumple los
//   filtros elegidos." (no_matches).

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

const noFilters: DefectFilterValues = {
  status: [],
  severity: [],
  storyId: "",
  sprintId: "",
};

const user = {
  id: "0d1c2b3a-4f5e-4d6c-8b7a-9e0f1a2b3c4d",
  fullName: "Ana Pérez",
};

const sprintTwo = {
  id: "c2222222-0000-4000-8000-000000000002",
  name: "Sprint 2",
  status: "closed" as const,
  startDate: "2026-09-15",
};
const sprintThree = {
  id: "a1111111-0000-4000-8000-000000000001",
  name: "Sprint 3",
  status: "active" as const,
  startDate: "2026-10-01",
};

// Defecto Crítico y En progreso, detectado en el Sprint 2 y sin sprint de resolución.
const unresolved: Defect = {
  id: "d1111111-0000-4000-8000-000000000001",
  projectId,
  description: "Total sin descuentos",
  severity: "critical",
  status: "in_progress",
  story: {
    id: "3f0c8a52-6a0e-4d1b-9a77-2f7e5b1c9d10",
    title: "Reporte de ventas mensual",
  },
  detectedSprint: sprintTwo,
  resolvedSprint: null,
  createdBy: user,
  updatedBy: user,
  statusChangedAt: "2026-10-24T13:00:00Z",
  statusChangedBy: user,
  createdAt: "2026-10-24T13:00:00Z",
  updatedAt: "2026-10-24T13:00:00Z",
};

// Defecto de severidad Baja, Resuelto en el Sprint 3, detectado en el Sprint 2.
const resolved: Defect = {
  ...unresolved,
  id: "d2222222-0000-4000-8000-000000000002",
  description: "Margen torcido en el PDF",
  severity: "low",
  status: "resolved",
  story: {
    id: "5a7b9c1d-2e3f-4a5b-8c6d-7e8f9a0b1c2d",
    title: "Alta de clientes",
  },
  resolvedSprint: sprintThree,
};

// Respuesta 200 del listado con el cuerpo dado.
function listResponse(body: ProjectDefects) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

function renderList(filters: DefectFilterValues = noFilters) {
  return render(
    <SessionProvider>
      <DefectList projectId={projectId} filters={filters} />
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

  // Escenario: sin filtros. Cada fila muestra descripción, severidad, estado, historia, sprint de
  // detección y, si lo tiene, sprint de resolución; el orden es el recibido (FR-043, FR-044).
  it("cada fila muestra severidad, estado, historia y los sprints", async () => {
    fetchMock.mockResolvedValue(
      listResponse({ defects: [unresolved, resolved], emptyReason: null }),
    );
    renderList();

    const items = await screen.findAllByRole("listitem");
    expect(items).toHaveLength(2);

    const first = items[0].textContent ?? "";
    expect(first).toContain("Total sin descuentos");
    expect(first).toContain("Crítica");
    expect(first).toContain("En progreso");
    expect(first).toContain("Reporte de ventas mensual");
    expect(first).toContain("Detectado en Sprint 2");
    expect(first).not.toContain("Resuelto en");

    const second = items[1].textContent ?? "";
    expect(second).toContain("Margen torcido en el PDF");
    expect(second).toContain("Baja");
    expect(second).toContain("Resuelto");
    expect(second).toContain("Alta de clientes");
    expect(second).toContain("Detectado en Sprint 2");
    expect(second).toContain("Resuelto en Sprint 3");
  });

  // Escenario: proyecto sin defectos. Aviso de no_defects, sin error y sin filas.
  it("el proyecto sin defectos muestra el aviso de que todavía no hay ninguno", async () => {
    fetchMock.mockResolvedValue(
      listResponse({ defects: [], emptyReason: "no_defects" }),
    );
    renderList();

    expect(
      await screen.findByText("Todavía no hay defectos registrados."),
    ).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText(/ningún defecto cumple/i)).toBeNull();
  });

  // Escenario: filtros sin coincidencias. Aviso distinto al anterior, sin error.
  it("los filtros sin coincidencias muestran un aviso distinto al del proyecto vacío", async () => {
    fetchMock.mockResolvedValue(
      listResponse({ defects: [], emptyReason: "no_matches" }),
    );
    renderList({ ...noFilters, severity: ["critical"] });

    expect(
      await screen.findByText("Ningún defecto cumple los filtros elegidos."),
    ).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText(/todavía no hay defectos/i)).toBeNull();
  });

  // Los filtros elegidos viajan en la consulta, con status y severity repetidos.
  it("pide el listado del proyecto con los filtros como parámetros repetidos", async () => {
    fetchMock.mockResolvedValue(
      listResponse({ defects: [unresolved], emptyReason: null }),
    );
    renderList({
      status: ["open", "in_progress"],
      severity: ["critical", "high"],
      storyId: "",
      sprintId: sprintTwo.id,
    });

    await screen.findAllByRole("listitem");
    const [input] = fetchMock.mock.calls[0];
    const url = new URL(
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.href
          : input.url,
      "http://localhost",
    );
    expect(url.pathname).toBe(`/api/v1/projects/${projectId}/defects`);
    expect(url.searchParams.getAll("status")).toEqual(["open", "in_progress"]);
    expect(url.searchParams.getAll("severity")).toEqual(["critical", "high"]);
    expect(url.searchParams.get("sprintId")).toBe(sprintTwo.id);
    expect(url.searchParams.has("storyId")).toBe(false);
  });

  // Un rechazo del backend (por ejemplo un 400 de filtro) se muestra como error con su mensaje.
  it("un rechazo del backend se muestra como error con su mensaje", async () => {
    fetchMock.mockResolvedValue(
      new Response(
        JSON.stringify({
          error: "Hay campos inválidos en la solicitud.",
          fields: [
            {
              field: "sprintId",
              message: "El filtro no corresponde al proyecto.",
            },
          ],
        }),
        { status: 400, headers: { "Content-Type": "application/json" } },
      ),
    );
    renderList({ ...noFilters, sprintId: sprintTwo.id });

    const alert = await screen.findByRole("alert");
    expect(within(alert).getByText(/campos inválidos/i)).toBeTruthy();
    expect(
      within(alert).getByText("El filtro no corresponde al proyecto."),
    ).toBeTruthy();
  });
});
