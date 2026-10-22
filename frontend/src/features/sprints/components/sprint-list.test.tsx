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
import { SprintList } from "@/features/sprints/components/sprint-list";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  SprintListItem,
  SprintListResponse,
  SprintStatus,
} from "@/types/sprint";

// Pruebas del listado de sprints del proyecto (T075). Atraviesan el componente, la llamada de
// listado de api.ts (T080) y el cliente HTTP real; solo se sustituye fetch, que es la frontera con
// el backend. El orden, el filtrado y el motivo del vacío los resuelve el backend (FR-050, FR-052,
// FR-054): acá se verifica qué pide el listado y qué muestra con cada respuesta.
//
// Contrato que fijan estas pruebas para SprintList:
// - Recibe projectId y pide GET /api/v1/projects/{projectId}/sprints con la sesión de la pestaña y
//   sin filtro de estado.
// - Muestra una lista con el nombre accesible "Sprints del proyecto" y un elemento de lista por
//   sprint, en el orden recibido. Cada elemento tiene un enlace con el nombre del sprint hacia
//   /projects/{projectId}/sprints/{sprintId}, la etiqueta en español de su estado, las fechas
//   previstas como "DD/MM/AAAA al DD/MM/AAAA", el Sprint Goal y la cantidad de historias
//   ("1 historia" o "N historias").
// - Con emptyReason no_sprints muestra que el proyecto todavía no tiene sprints, sin lista y sin
//   presentarlo como error.
// - Con un rechazo muestra su mensaje con role="alert" y ningún dato de sprints.

// Sustituye el enrutador de Next.js por si el listado o la guardia de sesión lo usan: fuera del App
// Router no hay enrutador montado.
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
type ResponseBody = SprintListResponse | ErrorResponse;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const sprintsPath = `/api/v1/projects/${projectId}/sprints`;
const projectNotFoundMessage = "El proyecto no existe.";

// Arma un sprint del listado con el estado, las fechas previstas y la cantidad de historias
// indicadas. Los datos reales y los totales solo se completan donde el estado los tiene.
function sprintItem(
  id: string,
  name: string,
  status: SprintStatus,
  startDate: string,
  endDate: string,
  storyCount: number,
): SprintListItem {
  const closed = status === "closed";
  return {
    id,
    name,
    goal: `Objetivo de ${name}`,
    status,
    startDate,
    endDate,
    startedAt: status === "planned" ? null : `${startDate}T12:00:00Z`,
    closedAt: closed ? `${endDate}T15:00:00Z` : null,
    storyCount,
    results: closed
      ? {
          committedStoryPoints: 13,
          plannedStoryPoints: 13,
          completedStoryPoints: 8,
          plannedStoryCount: storyCount,
          completedStoryCount: 1,
        }
      : null,
  };
}

// Un sprint Planificado, uno Activo y uno Cerrado, en el orden que devuelve el backend: fecha de
// inicio descendente (FR-052).
const planned = sprintItem(
  "3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f",
  "Sprint 3",
  "planned",
  "2026-10-29",
  "2026-11-11",
  1,
);
const active = sprintItem(
  "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",
  "Sprint 2",
  "active",
  "2026-10-15",
  "2026-10-28",
  2,
);
const closed = sprintItem(
  "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
  "Sprint 1",
  "closed",
  "2026-10-01",
  "2026-10-14",
  4,
);

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(status: number, body: ResponseBody) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Extrae la ruta, los estados pedidos, el método y la cabecera Authorization de una llamada a fetch.
function describeRequest(input: RequestInfo | URL, init?: RequestInit) {
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  const url = new URL(raw, "http://localhost");
  return {
    path: url.pathname,
    statuses: url.searchParams.getAll("status"),
    method: input instanceof Request ? input.method : (init?.method ?? "GET"),
    authorization: new Headers(init?.headers).get("Authorization"),
  };
}

// Simula el backend del listado: filtra los sprints dados por los estados pedidos y arma el motivo
// del vacío como lo hace el servicio. Cualquier otra ruta responde 404, para que una llamada
// inesperada no pase inadvertida.
function serve(sprints: SprintListItem[]) {
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const request = describeRequest(input, init);
    if (request.method !== "GET" || request.path !== sprintsPath) {
      return jsonResponse(404, { error: "Ruta inesperada en la prueba." });
    }
    const statuses = request.statuses as SprintStatus[];
    const matching =
      statuses.length === 0
        ? sprints
        : sprints.filter((sprint) => statuses.includes(sprint.status));
    return jsonResponse(200, {
      sprints: matching,
      appliedFilters: { status: statuses },
      emptyReason:
        sprints.length === 0
          ? "no_sprints"
          : matching.length === 0
            ? "no_matches"
            : null,
    });
  };
}

// Monta el listado dentro del proveedor de sesión, como en el layout raíz.
function renderList() {
  return render(
    <SessionProvider>
      <SprintList projectId={projectId} />
    </SessionProvider>,
  );
}

// Devuelve el elemento de lista del sprint dado, buscando el enlace con su nombre.
function itemOf(list: HTMLElement, sprint: SprintListItem): HTMLElement {
  const item = within(list)
    .getByRole("link", { name: sprint.name })
    .closest("li");
  expect(item, sprint.name).not.toBeNull();
  return item as HTMLElement;
}

describe("004/US6 - Consultar el historial de sprints", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un proyecto con un sprint de cada
  // estado.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(serve([planned, active, closed]));
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso alternativo: con un sprint Cerrado, uno Activo y uno Planificado, el listado completo los
  // muestra a los tres con su estado, en el orden recibido y con sus datos (FR-050, FR-052). El
  // listado se pide con la sesión y sin filtro de estado.
  it("Listado completo y filtro", async () => {
    renderList();

    const list = await screen.findByRole("list", {
      name: "Sprints del proyecto",
    });
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(3);
    expect(
      items.map((item) => within(item).getByRole("link").textContent),
    ).toEqual(["Sprint 3", "Sprint 2", "Sprint 1"]);

    const expectations: [SprintListItem, string, string, string][] = [
      [planned, "Planificado", "29/10/2026 al 11/11/2026", "1 historia"],
      [active, "Activo", "15/10/2026 al 28/10/2026", "2 historias"],
      [closed, "Cerrado", "01/10/2026 al 14/10/2026", "4 historias"],
    ];
    for (const [sprint, statusLabel, dates, storyCount] of expectations) {
      const item = itemOf(list, sprint);
      const text = item.textContent ?? "";
      expect(text, sprint.name).toContain(statusLabel);
      expect(text, sprint.name).toContain(dates);
      expect(text, sprint.name).toContain(sprint.goal);
      expect(text, sprint.name).toContain(storyCount);
      expect(
        within(item)
          .getByRole("link", { name: sprint.name })
          .getAttribute("href"),
      ).toBe(`/projects/${projectId}/sprints/${sprint.id}`);
    }

    const requests = fetchMock.mock.calls.map(([input, init]) =>
      describeRequest(input, init),
    );
    expect(requests).toHaveLength(1);
    expect(requests[0]).toMatchObject({
      path: sprintsPath,
      statuses: [],
      authorization: `Bearer ${sessionToken}`,
    });
  });

  // Caso límite: un proyecto sin ningún sprint muestra la indicación de que todavía no hay sprints,
  // sin lista y sin presentarlo como error (FR-054).
  it("Proyecto sin sprints", async () => {
    fetchMock.mockImplementation(serve([]));
    renderList();

    expect(
      await screen.findByText(/el proyecto todavía no tiene sprints/i),
    ).toBeTruthy();
    expect(
      screen.queryByRole("list", { name: "Sprints del proyecto" }),
    ).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso de error: quien no es integrante recibe el 404 de proyecto inexistente; se muestra su
  // mensaje y ningún dato de sprints (FR-058).
  it("No integrante", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(404, { error: projectNotFoundMessage }),
    );
    renderList();

    expect((await screen.findByRole("alert")).textContent).toContain(
      projectNotFoundMessage,
    );
    expect(
      screen.queryByRole("list", { name: "Sprints del proyecto" }),
    ).toBeNull();
    expect(screen.queryByText(/Sprint \d/)).toBeNull();
  });
});
