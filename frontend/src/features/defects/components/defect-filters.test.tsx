import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { useState } from "react";
import { DefectFilters } from "@/features/defects/components/defect-filters";
import { DefectList } from "@/features/defects/components/defect-list";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type {
  Defect,
  DefectFilterValues,
  DefectSeverity,
  DefectStatus,
  ProjectDefects,
} from "@/types/defect";

// Pruebas de los filtros del listado de defectos (T040). Combinan DefectFilters con DefectList y el
// cliente HTTP real, como los compondrá la página de defectos; solo se sustituye fetch por un
// backend de prueba que filtra un conjunto fijo de defectos.
//
// Contrato que fijan estas pruebas para DefectFilters:
// - Es un componente controlado: recibe projectId y los filtros elegidos (value) y comunica cada
//   cambio con onChange, entregando los cuatro filtros completos (status, severity, storyId y
//   sprintId).
// - Ofrece un grupo (role="group") llamado "Estado" con una casilla por estado (Abierto, En progreso,
//   Resuelto, Descartado) y otro llamado "Severidad" con una casilla por severidad (Crítica, Alta,
//   Media, Baja). Marcar varias casillas de un grupo elige varios valores del mismo filtro.
// - La consulta del listado lleva status y severity repetidos, uno por valor elegido, y no lleva
//   parámetros de los filtros vacíos.

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

// Construye un defecto del listado con los datos que usan las pruebas.
function defect(
  id: string,
  description: string,
  severity: DefectSeverity,
  status: DefectStatus,
): Defect {
  return {
    id,
    projectId,
    description,
    severity,
    status,
    story: {
      id: "3f0c8a52-6a0e-4d1b-9a77-2f7e5b1c9d10",
      title: "Reporte de ventas mensual",
    },
    detectedSprint: {
      id: "c2222222-0000-4000-8000-000000000002",
      name: "Sprint 2",
      status: "closed",
      startDate: "2026-10-01",
    },
    resolvedSprint: null,
    createdBy: user,
    updatedBy: user,
    statusChangedAt: "2026-10-24T13:00:00Z",
    statusChangedBy: user,
    createdAt: "2026-10-24T13:00:00Z",
    updatedAt: "2026-10-24T13:00:00Z",
  };
}

const criticalOpen = defect("d1", "Total sin descuentos", "critical", "open");
const criticalProgress = defect(
  "d2",
  "Falla el login con tilde",
  "critical",
  "in_progress",
);
const highOpen = defect("d3", "Exportación lenta", "high", "open");
const lowDiscarded = defect("d4", "Margen torcido", "low", "discarded");
const allDefects = [criticalOpen, criticalProgress, highOpen, lowDiscarded];

// Extrae la URL de una llamada a fetch.
function urlOf(input: RequestInfo | URL): URL {
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  return new URL(raw, "http://localhost");
}

// Backend de prueba: el listado de defectos filtra con O dentro de un filtro y Y entre filtros, y
// elige el motivo del vacío; los listados de historias y sprints (por si los filtros de historia y
// sprint los piden) devuelven listas vacías.
function fakeBackend(defects: Defect[]) {
  return async (input: RequestInfo | URL) => {
    const url = urlOf(input);
    if (!url.pathname.endsWith("/defects")) {
      const empty = url.pathname.endsWith("/sprints")
        ? { sprints: [], appliedFilters: { status: [] }, emptyReason: null }
        : {
            stories: [],
            appliedFilters: { status: [], priority: [] },
            emptyReason: null,
          };
      return new Response(JSON.stringify(empty), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }
    const statuses = url.searchParams.getAll("status");
    const severities = url.searchParams.getAll("severity");
    const matches = defects.filter(
      (item) =>
        (statuses.length === 0 || statuses.includes(item.status)) &&
        (severities.length === 0 || severities.includes(item.severity)),
    );
    let emptyReason: ProjectDefects["emptyReason"] = null;
    if (matches.length === 0) {
      emptyReason = defects.length === 0 ? "no_defects" : "no_matches";
    }
    const body: ProjectDefects = { defects: matches, emptyReason };
    return new Response(JSON.stringify(body), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };
}

// Devuelve los parámetros de consulta de la última llamada al listado de defectos.
function lastDefectsQuery(fetchMock: FetchMock): URLSearchParams {
  const calls = fetchMock.mock.calls.filter(([input]) =>
    urlOf(input).pathname.endsWith("/defects"),
  );
  expect(calls.length).toBeGreaterThan(0);
  return urlOf(calls[calls.length - 1][0]).searchParams;
}

// Compone los filtros y el listado con el estado compartido, como la página de defectos.
function FilteredDefects() {
  const [filters, setFilters] = useState<DefectFilterValues>(noFilters);
  return (
    <>
      <DefectFilters
        projectId={projectId}
        value={filters}
        onChange={setFilters}
      />
      <DefectList projectId={projectId} filters={filters} />
    </>
  );
}

// Monta los filtros y el listado dentro del proveedor de sesión, como en el layout raíz.
function renderFilteredDefects() {
  return render(
    <SessionProvider>
      <FilteredDefects />
    </SessionProvider>,
  );
}

// Localiza la casilla con la etiqueta dada dentro del grupo de filtros indicado.
function checkbox(group: "Estado" | "Severidad", label: string): HTMLElement {
  const fieldset = screen.getByRole("group", { name: new RegExp(group, "i") });
  return within(fieldset).getByRole("checkbox", { name: label });
}

// Espera a que el listado muestre exactamente los defectos dados, en ese orden.
async function expectShownDescriptions(descriptions: string[]) {
  await waitFor(() => {
    const items = screen.queryAllByRole("listitem");
    expect(items).toHaveLength(descriptions.length);
    descriptions.forEach((description, index) => {
      expect(items[index].textContent).toContain(description);
    });
  });
}

describe("007/US3 - Consultar y filtrar los defectos", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida y el backend con los cuatro defectos.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>();
    fetchMock.mockImplementation(fakeBackend(allDefects));
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Escenario: filtro por estado. Marcar Abierto pide ?status=open y deja solo los Abiertos.
  it("el filtro por estado pide status y deja solo los defectos de ese estado", async () => {
    renderFilteredDefects();
    await expectShownDescriptions(allDefects.map((item) => item.description));

    fireEvent.click(checkbox("Estado", "Abierto"));

    await expectShownDescriptions([
      criticalOpen.description,
      highOpen.description,
    ]);
    const query = lastDefectsQuery(fetchMock);
    expect(query.getAll("status")).toEqual(["open"]);
    expect(query.getAll("severity")).toEqual([]);
    expect(query.has("storyId")).toBe(false);
    expect(query.has("sprintId")).toBe(false);
  });

  // Escenario: severidad y estado combinados. Los dos filtros viajan a la vez y se combinan con Y.
  it("severidad y estado combinados envían ambos filtros", async () => {
    renderFilteredDefects();
    await expectShownDescriptions(allDefects.map((item) => item.description));

    fireEvent.click(checkbox("Severidad", "Crítica"));
    fireEvent.click(checkbox("Estado", "En progreso"));

    await expectShownDescriptions([criticalProgress.description]);
    const query = lastDefectsQuery(fetchMock);
    expect(query.getAll("status")).toEqual(["in_progress"]);
    expect(query.getAll("severity")).toEqual(["critical"]);
  });

  // Varios valores de un mismo filtro se envían como parámetros repetidos, uno por valor, y se
  // combinan con O (quien quiere excluir los Descartados filtra por los otros tres, FR-040).
  it("varios valores de un filtro se envían como parámetros repetidos", async () => {
    renderFilteredDefects();
    await expectShownDescriptions(allDefects.map((item) => item.description));

    fireEvent.click(checkbox("Estado", "Abierto"));
    fireEvent.click(checkbox("Estado", "En progreso"));
    fireEvent.click(checkbox("Severidad", "Crítica"));
    fireEvent.click(checkbox("Severidad", "Alta"));

    await expectShownDescriptions([
      criticalOpen.description,
      criticalProgress.description,
      highOpen.description,
    ]);
    const query = lastDefectsQuery(fetchMock);
    expect([...query.getAll("status")].sort()).toEqual(["in_progress", "open"]);
    expect([...query.getAll("severity")].sort()).toEqual(["critical", "high"]);
    expect(query.toString().match(/status=/g)).toHaveLength(2);
    expect(query.toString().match(/severity=/g)).toHaveLength(2);
  });

  // Desmarcar el único filtro elegido vuelve a pedir el listado sin filtros.
  it("al quitar el filtro vuelve a pedir el listado completo", async () => {
    renderFilteredDefects();
    await expectShownDescriptions(allDefects.map((item) => item.description));

    fireEvent.click(checkbox("Estado", "Descartado"));
    await expectShownDescriptions([lowDiscarded.description]);
    fireEvent.click(checkbox("Estado", "Descartado"));

    await expectShownDescriptions(allDefects.map((item) => item.description));
    expect([...lastDefectsQuery(fetchMock).keys()]).toHaveLength(0);
  });

  // Los filtros ofrecen solo los estados y severidades admitidos, con sus etiquetas en español, de
  // modo que no se puede elegir un valor inexistente (FR-046).
  it("ofrece solo los estados y las severidades admitidos", () => {
    render(
      <SessionProvider>
        <DefectFilters
          projectId={projectId}
          value={noFilters}
          onChange={vi.fn()}
        />
      </SessionProvider>,
    );

    const statusGroup = screen.getByRole("group", { name: /estado/i });
    const severityGroup = screen.getByRole("group", { name: /severidad/i });
    expect(within(statusGroup).getAllByRole("checkbox")).toHaveLength(4);
    expect(within(severityGroup).getAllByRole("checkbox")).toHaveLength(4);
    for (const label of ["Abierto", "En progreso", "Resuelto", "Descartado"]) {
      expect(
        within(statusGroup).getByRole("checkbox", { name: label }),
      ).toBeTruthy();
    }
    for (const label of ["Crítica", "Alta", "Media", "Baja"]) {
      expect(
        within(severityGroup).getByRole("checkbox", { name: label }),
      ).toBeTruthy();
    }
  });

  // Los filtros son controlados: muestran marcados los valores recibidos y, al marcar otro,
  // entregan los cuatro filtros completos sin perder lo ya elegido.
  it("refleja los filtros recibidos y comunica el cambio conservando los demás", () => {
    const onChange = vi.fn<(next: DefectFilterValues) => void>();
    render(
      <SessionProvider>
        <DefectFilters
          projectId={projectId}
          value={{ ...noFilters, status: ["resolved"], severity: ["medium"] }}
          onChange={onChange}
        />
      </SessionProvider>,
    );

    expect((checkbox("Estado", "Resuelto") as HTMLInputElement).checked).toBe(
      true,
    );
    expect((checkbox("Severidad", "Media") as HTMLInputElement).checked).toBe(
      true,
    );

    fireEvent.click(checkbox("Severidad", "Alta"));

    expect(onChange).toHaveBeenCalledTimes(1);
    const [next] = onChange.mock.calls[0];
    expect(next.status).toEqual(["resolved"]);
    expect([...next.severity].sort()).toEqual(["high", "medium"]);
    expect(next.storyId).toBe("");
    expect(next.sprintId).toBe("");
  });
});
