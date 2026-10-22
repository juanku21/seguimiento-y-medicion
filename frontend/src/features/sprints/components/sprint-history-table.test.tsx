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
import { SprintHistoryTable } from "@/features/sprints/components/sprint-history-table";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  SprintListItem,
  SprintListResponse,
  SprintResults,
  SprintStatus,
} from "@/types/sprint";

// Pruebas del historial de sprints cerrados (T075). Atraviesan el componente, la llamada de listado
// de api.ts (T080) y el cliente HTTP real; solo se sustituye fetch, que es la frontera con el
// backend. Los totales los congeló el backend al cerrar (FR-043, FR-051) y el filtro por estado lo
// aplica el backend: acá se verifica que el historial los pide filtrados y los muestra tal cual
// llegan.
//
// Contrato que fijan estas pruebas para SprintHistoryTable:
// - Recibe projectId y pide GET /api/v1/projects/{projectId}/sprints?status=closed con la sesión
//   de la pestaña.
// - Muestra una tabla con el nombre accesible "Historial de sprints cerrados", con una fila de
//   encabezados y una fila por sprint en el orden recibido. Las columnas, en este orden, son:
//   "Sprint", "Sprint Goal", "Inicio previsto", "Fin previsto", "Inicio real", "Cierre real",
//   "Historias planificadas", "Historias completadas", "Story Points comprometidos al inicio",
//   "Story Points planificados al cierre" y "Story Points completados".
// - Las fechas previstas se muestran como DD/MM/AAAA y las reales como la fecha DD/MM/AAAA del
//   instante en la zona horaria del navegador; los totales, como enteros.
// - Con emptyReason no_sprints muestra que el proyecto todavía no tiene sprints y con no_matches que
//   todavía no hay sprints cerrados; en ambos casos sin tabla y sin presentarlo como error.

// Sustituye el enrutador de Next.js por si el historial o la guardia de sesión lo usan: fuera del
// App Router no hay enrutador montado.
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
const tableName = "Historial de sprints cerrados";

// Arma los totales de la instantánea.
function results(
  committed: number,
  planned: number,
  completed: number,
  plannedCount: number,
  completedCount: number,
): SprintResults {
  return {
    committedStoryPoints: committed,
    plannedStoryPoints: planned,
    completedStoryPoints: completed,
    plannedStoryCount: plannedCount,
    completedStoryCount: completedCount,
  };
}

// Arma un sprint Cerrado con sus cuatro fechas y sus totales. Los instantes reales caen a media
// jornada en UTC para que su fecha sea la misma en cualquier zona horaria de América.
function closedSprint(
  id: string,
  name: string,
  dates: {
    startDate: string;
    endDate: string;
    startedOn: string;
    closedOn: string;
  },
  snapshot: SprintResults,
): SprintListItem {
  return {
    id,
    name,
    goal: `Objetivo de ${name}`,
    status: "closed",
    startDate: dates.startDate,
    endDate: dates.endDate,
    startedAt: `${dates.startedOn}T12:00:00Z`,
    closedAt: `${dates.closedOn}T15:00:00Z`,
    storyCount: snapshot.plannedStoryCount,
    results: snapshot,
  };
}

// Arma un sprint abierto (Planificado o Activo), sin cierre ni totales.
function openSprint(
  id: string,
  name: string,
  status: SprintStatus,
  startDate: string,
  endDate: string,
): SprintListItem {
  return {
    id,
    name,
    goal: `Objetivo de ${name}`,
    status,
    startDate,
    endDate,
    startedAt: status === "active" ? `${startDate}T12:00:00Z` : null,
    closedAt: null,
    storyCount: 2,
    results: null,
  };
}

// Tres sprints cerrados con resultados distintos, en el orden que devuelve el backend (US6-1).
const sprint3 = closedSprint(
  "3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f",
  "Sprint 3",
  {
    startDate: "2026-10-29",
    endDate: "2026-11-11",
    startedOn: "2026-10-29",
    closedOn: "2026-11-11",
  },
  results(8, 8, 8, 2, 2),
);
const sprint2 = closedSprint(
  "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",
  "Sprint 2",
  {
    startDate: "2026-10-15",
    endDate: "2026-10-28",
    startedOn: "2026-10-15",
    closedOn: "2026-10-28",
  },
  results(20, 36, 13, 5, 3),
);
// Previsto del 2026-10-01 al 2026-10-14, arrancó el 2026-10-03 y se cerró el 2026-10-10 (US6-9).
const sprint1 = closedSprint(
  "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
  "Sprint 1",
  {
    startDate: "2026-10-01",
    endDate: "2026-10-14",
    startedOn: "2026-10-03",
    closedOn: "2026-10-10",
  },
  results(20, 21, 13, 4, 2),
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

// Monta el historial dentro del proveedor de sesión, como en el layout raíz.
function renderHistory() {
  return render(
    <SessionProvider>
      <SprintHistoryTable projectId={projectId} />
    </SessionProvider>,
  );
}

// Lee la tabla del historial como una lista de filas, cada una con el texto de sus celdas indexado
// por el encabezado de su columna.
async function readHistory(): Promise<Record<string, string>[]> {
  const table = await screen.findByRole("table", { name: tableName });
  const headers = within(table)
    .getAllByRole("columnheader")
    .map((header) => header.textContent?.trim() ?? "");
  const rows = within(table)
    .getAllByRole("row")
    .filter((row) => within(row).queryAllByRole("columnheader").length === 0);
  return rows.map((row) => {
    const cells = Array.from(row.querySelectorAll("td, th"));
    return Object.fromEntries(
      headers.map((header, index) => [
        header,
        cells[index]?.textContent?.trim() ?? "",
      ]),
    );
  });
}

// Fila esperada del historial para un sprint cerrado, con sus fechas ya formateadas.
function expectedRow(
  sprint: SprintListItem,
  startedOn: string,
  closedOn: string,
): Record<string, string> {
  const snapshot = sprint.results as SprintResults;
  return {
    Sprint: sprint.name,
    "Sprint Goal": sprint.goal,
    "Inicio previsto": formatDay(sprint.startDate),
    "Fin previsto": formatDay(sprint.endDate),
    "Inicio real": startedOn,
    "Cierre real": closedOn,
    "Historias planificadas": String(snapshot.plannedStoryCount),
    "Historias completadas": String(snapshot.completedStoryCount),
    "Story Points comprometidos al inicio": String(
      snapshot.committedStoryPoints,
    ),
    "Story Points planificados al cierre": String(snapshot.plannedStoryPoints),
    "Story Points completados": String(snapshot.completedStoryPoints),
  };
}

// Pasa una fecha AAAA-MM-DD a DD/MM/AAAA para comparar con lo mostrado.
function formatDay(isoDate: string): string {
  const [year, month, day] = isoDate.split("-");
  return `${day}/${month}/${year}`;
}

describe("004/US6 - Consultar el historial de sprints", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un proyecto con tres sprints
  // cerrados.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(serve([sprint3, sprint2, sprint1]));
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: con tres sprints cerrados, el historial los muestra a los tres en el orden
  // recibido con nombre, Sprint Goal, cuatro fechas, historias planificadas y completadas, y los
  // Story Points comprometidos al inicio, planificados al cierre y completados (FR-051).
  it("Historial de cerrados", async () => {
    renderHistory();

    expect(await readHistory()).toEqual([
      expectedRow(sprint3, "29/10/2026", "11/11/2026"),
      expectedRow(sprint2, "15/10/2026", "28/10/2026"),
      expectedRow(sprint1, "03/10/2026", "10/10/2026"),
    ]);
  });

  // Caso alternativo: con un sprint Cerrado, uno Activo y uno Planificado, el historial pide solo
  // los cerrados con la sesión y muestra únicamente el Cerrado.
  it("Listado completo y filtro", async () => {
    fetchMock.mockImplementation(
      serve([
        openSprint(
          "5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a81",
          "Sprint 3",
          "planned",
          "2026-10-29",
          "2026-11-11",
        ),
        openSprint(
          "4d5e6f7a-8b9c-4d0e-9f1a-2b3c4d5e6f70",
          "Sprint 2",
          "active",
          "2026-10-15",
          "2026-10-28",
        ),
        sprint1,
      ]),
    );
    renderHistory();

    const rows = await readHistory();
    expect(rows.map((row) => row.Sprint)).toEqual(["Sprint 1"]);

    const requests = fetchMock.mock.calls.map(([input, init]) =>
      describeRequest(input, init),
    );
    expect(requests).toHaveLength(1);
    expect(requests[0]).toMatchObject({
      path: sprintsPath,
      statuses: ["closed"],
      authorization: `Bearer ${sessionToken}`,
    });
  });

  // Caso límite: un sprint previsto del 2026-10-01 al 2026-10-14 que arrancó el 2026-10-03 y se
  // cerró el 2026-10-10 muestra las cuatro fechas por separado.
  it("Cuatro fechas", async () => {
    fetchMock.mockImplementation(serve([sprint1]));
    renderHistory();

    const [row] = await readHistory();
    expect(row["Inicio previsto"]).toBe("01/10/2026");
    expect(row["Fin previsto"]).toBe("14/10/2026");
    expect(row["Inicio real"]).toBe("03/10/2026");
    expect(row["Cierre real"]).toBe("10/10/2026");
  });

  // Caso límite: sprints cerrados que no completaron ninguna historia se listan con cero Story
  // Points completados, sin que eso sea un error.
  it("Cerrados sin completadas", async () => {
    const empty1 = closedSprint(
      "6f7a8b9c-0d1e-4f2a-9b3c-4d5e6f7a8b92",
      "Sprint 2",
      {
        startDate: "2026-10-15",
        endDate: "2026-10-28",
        startedOn: "2026-10-15",
        closedOn: "2026-10-28",
      },
      results(13, 13, 0, 3, 0),
    );
    const empty2 = closedSprint(
      "7a8b9c0d-1e2f-4a3b-8c4d-5e6f7a8b9ca3",
      "Sprint 1",
      {
        startDate: "2026-10-01",
        endDate: "2026-10-14",
        startedOn: "2026-10-01",
        closedOn: "2026-10-14",
      },
      results(8, 8, 0, 2, 0),
    );
    fetchMock.mockImplementation(serve([empty1, empty2]));
    renderHistory();

    const rows = await readHistory();
    expect(rows.map((row) => row.Sprint)).toEqual(["Sprint 2", "Sprint 1"]);
    for (const row of rows) {
      expect(row["Story Points completados"], row.Sprint).toBe("0");
      expect(row["Historias completadas"], row.Sprint).toBe("0");
    }
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso límite: un proyecto sin ningún sprint muestra la indicación de que todavía no hay sprints,
  // sin tabla y sin presentarlo como error (FR-054).
  it("Proyecto sin sprints", async () => {
    fetchMock.mockImplementation(serve([]));
    renderHistory();

    expect(
      await screen.findByText(/el proyecto todavía no tiene sprints/i),
    ).toBeTruthy();
    expect(screen.queryByRole("table", { name: tableName })).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Regla de la interfaz: un proyecto con sprints pero ninguno cerrado (emptyReason no_matches)
  // muestra que todavía no hay sprints cerrados, sin tabla y sin presentarlo como error.
  it("indica que todavía no hay sprints cerrados", async () => {
    fetchMock.mockImplementation(
      serve([
        openSprint(
          "4d5e6f7a-8b9c-4d0e-9f1a-2b3c4d5e6f70",
          "Sprint 1",
          "active",
          "2026-10-15",
          "2026-10-28",
        ),
      ]),
    );
    renderHistory();

    expect(
      await screen.findByText(/todavía no hay sprints cerrados/i),
    ).toBeTruthy();
    expect(screen.queryByRole("table", { name: tableName })).toBeNull();
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
