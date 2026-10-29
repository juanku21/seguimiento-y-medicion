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
import { ProjectEffortComparisonSection } from "@/features/effort/components/project-effort-comparison";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type { ProjectEffortComparison } from "@/types/effort";

// Pruebas de la sección de comparación del proyecto (T078). Atraviesan el componente, la llamada de
// api.ts y el cliente HTTP real; solo se sustituye fetch por un backend falso que responde
// GET /api/v1/projects/{projectId}/effort-comparison como lo indica contracts/openapi.yaml.
//
// Contrato que fijan estas pruebas para ProjectEffortComparisonSection:
// - Recibe projectId y se monta dentro de SessionProvider.
// - Mientras espera la respuesta muestra "Cargando comparación…" con role="status".
// - Con la respuesta muestra la tabla "Comparación estimado contra real por historia".
// - Un proyecto con todas las historias sin estimar se muestra completo, sin role="alert" (AC9).
// - Ante 404 o 500 muestra el mensaje del error con role="alert" y no muestra la tabla (AC10).

// Sustituye el enrutador de Next.js: la guardia de sesión lo usa.
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
const comparisonPath = `/api/v1/projects/${projectId}/effort-comparison`;
const tableName = "Comparación estimado contra real por historia";

// Comparación con una historia estimada y otra sin estimar.
const comparison: ProjectEffortComparison = {
  hoursPerStoryPoint: 6,
  stories: [
    {
      storyId: "2b9f1c3e-8d4a-4f5b-9e6c-7a8b9c0d1e2f",
      title: "Alta de clientes",
      storyPoints: 5,
      estimatedHours: 30,
      actualHours: 36,
      deviationHours: 6,
      deviationPercent: 20,
    },
    {
      storyId: "3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f",
      title: "Reporte de velocidad",
      storyPoints: null,
      estimatedHours: null,
      actualHours: 8,
      deviationHours: null,
      deviationPercent: null,
    },
  ],
};

// Comparación donde ninguna historia tiene Story Points (AC9).
const allUnestimated: ProjectEffortComparison = {
  hoursPerStoryPoint: 6,
  stories: comparison.stories.map((story) => ({
    ...story,
    storyPoints: null,
    estimatedHours: null,
    deviationHours: null,
    deviationPercent: null,
  })),
};

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(
  status: number,
  body: ProjectEffortComparison | ErrorResponse,
) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Extrae la ruta de una llamada a fetch.
function pathOf(input: RequestInfo | URL): string {
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  return new URL(raw, "http://localhost").pathname;
}

// Monta la sección dentro del proveedor de sesión, como en el layout raíz.
function renderSection() {
  render(
    <SessionProvider>
      <ProjectEffortComparisonSection projectId={projectId} />
    </SessionProvider>,
  );
}

describe("006/US5 - Comparar el esfuerzo estimado con el real", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña.
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

  it("Caso normal — la sección pide la comparación del proyecto y muestra la tabla", async () => {
    fetchMock.mockImplementation(async () => jsonResponse(200, comparison));
    renderSection();

    const table = await screen.findByRole("table", { name: tableName });
    expect(within(table).getByText("Alta de clientes")).toBeTruthy();
    expect(within(table).getByText("Reporte de velocidad")).toBeTruthy();
    expect(within(table).getByText("+6 h")).toBeTruthy();
    const paths = fetchMock.mock.calls.map(([input]) => pathOf(input));
    expect(paths).toContain(comparisonPath);
  });

  it("Caso límite — mientras espera la respuesta muestra el estado de carga", async () => {
    fetchMock.mockImplementation(() => new Promise<Response>(() => {}));
    renderSection();

    const status = await screen.findByRole("status");
    expect(status.textContent).toContain("Cargando comparación");
    expect(screen.queryByRole("table")).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("Caso límite — proyecto con todas las historias sin estimar", async () => {
    fetchMock.mockImplementation(async () => jsonResponse(200, allUnestimated));
    renderSection();

    const table = await screen.findByRole("table", { name: tableName });
    expect(within(table).getAllByText("Sin estimar")).toHaveLength(2);
    expect(within(table).getAllByText("No calculable").length).toBeGreaterThan(
      0,
    );
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("Caso de error — el proyecto no existe o no se puede consultar (404)", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(404, { error: "Proyecto no encontrado." }),
    );
    renderSection();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Proyecto no encontrado.");
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("Caso de error — falla interna del servidor (500)", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(500, { error: "Error interno del servidor." }),
    );
    renderSection();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Error interno del servidor.");
    expect(screen.queryByRole("table")).toBeNull();
  });
});
