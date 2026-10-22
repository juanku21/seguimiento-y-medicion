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
import { StorySprintHistory } from "@/features/sprints/components/story-sprint-history";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  StoryOutcome,
  StorySprintHistoryItem,
  StorySprintHistoryResponse,
  SprintStatus,
} from "@/types/sprint";

// Pruebas de la sección "Sprints en los que estuvo" de una historia (T075). Atraviesan el
// componente, la llamada del historial de api.ts (T080) y el cliente HTTP real; solo se sustituye
// fetch, que es la frontera con el backend. El orden y el resultado de cada sprint los resuelve el
// backend (FR-055, RN-17): acá se verifica qué pide la sección y qué muestra con cada respuesta.
//
// Contrato que fijan estas pruebas para StorySprintHistory:
// - Recibe storyId y pide GET /api/v1/stories/{storyId}/sprints con la sesión de la pestaña.
// - Muestra una región con el título "Sprints en los que estuvo" y, dentro, un elemento de lista
//   por sprint en el orden recibido, con el nombre del sprint, la etiqueta en español de su
//   estado, sus fechas previstas como "DD/MM/AAAA al DD/MM/AAAA" y, si el sprint está cerrado, la
//   etiqueta en español del resultado ("Completada" o "No completada"). En un sprint abierto
//   (outcome null) no muestra ningún resultado.
// - Una historia que nunca estuvo en un sprint muestra que todavía no estuvo en ningún sprint, sin
//   presentarlo como error.
// - Con un rechazo muestra su mensaje con role="alert".

// Sustituye el enrutador de Next.js por si la sección o la guardia de sesión lo usan: fuera del App
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
type ResponseBody = StorySprintHistoryResponse | ErrorResponse;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const storyId = "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d";
const historyPath = `/api/v1/stories/${storyId}/sprints`;
const regionName = "Sprints en los que estuvo";
const storyNotFoundMessage = "La historia no existe.";

// Arma un sprint del historial de la historia con su estado, fechas previstas y resultado.
function historyItem(
  sprintId: string,
  sprintName: string,
  sprintStatus: SprintStatus,
  startDate: string,
  endDate: string,
  outcome: StoryOutcome | null,
): StorySprintHistoryItem {
  return {
    sprintId,
    sprintName,
    sprintStatus,
    startDate,
    endDate,
    outcome,
    completedAt: outcome === "completed" ? `${endDate}T15:00:00Z` : null,
  };
}

// La historia estuvo en tres sprints hasta completarse, en el orden que devuelve el backend: fecha
// de inicio ascendente (US6-3).
const threeSprints = [
  historyItem(
    "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
    "Sprint 1",
    "closed",
    "2026-10-01",
    "2026-10-14",
    "not_completed",
  ),
  historyItem(
    "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",
    "Sprint 2",
    "closed",
    "2026-10-15",
    "2026-10-28",
    "not_completed",
  ),
  historyItem(
    "3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f",
    "Sprint 3",
    "closed",
    "2026-10-29",
    "2026-11-11",
    "completed",
  ),
];

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(status: number, body: ResponseBody) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Extrae la ruta, el método y la cabecera Authorization de una llamada a fetch.
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
    method: input instanceof Request ? input.method : (init?.method ?? "GET"),
    authorization: new Headers(init?.headers).get("Authorization"),
  };
}

// Simula el backend del historial de la historia con los sprints dados. Cualquier otra ruta
// responde 404, para que una llamada inesperada no pase inadvertida.
function serve(sprints: StorySprintHistoryItem[]) {
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const request = describeRequest(input, init);
    if (request.method === "GET" && request.path === historyPath) {
      return jsonResponse(200, { sprints });
    }
    return jsonResponse(404, { error: "Ruta inesperada en la prueba." });
  };
}

// Monta la sección dentro del proveedor de sesión, como en el layout raíz.
function renderHistory() {
  return render(
    <SessionProvider>
      <StorySprintHistory storyId={storyId} />
    </SessionProvider>,
  );
}

// Devuelve el texto de cada elemento de lista de la sección, en el orden mostrado.
async function shownItems(): Promise<string[]> {
  const region = await screen.findByRole("region", { name: regionName });
  return within(region)
    .getAllByRole("listitem")
    .map((item) => item.textContent ?? "");
}

describe("004/US6 - Consultar el historial de sprints", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y una historia que estuvo en tres
  // sprints.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(serve(threeSprints));
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso alternativo: una historia que estuvo en tres sprints hasta completarse muestra los tres en
  // el orden recibido, cada uno con su resultado: no completada, no completada y completada
  // (FR-055). El historial se pide con la sesión de la pestaña.
  it("Historial de una historia", async () => {
    renderHistory();

    const items = await shownItems();
    expect(items).toHaveLength(3);
    const expectations: [string, string, string][] = [
      ["Sprint 1", "01/10/2026 al 14/10/2026", "No completada"],
      ["Sprint 2", "15/10/2026 al 28/10/2026", "No completada"],
      ["Sprint 3", "29/10/2026 al 11/11/2026", "Completada"],
    ];
    expectations.forEach(([name, dates, outcome], index) => {
      expect(items[index], name).toContain(name);
      expect(items[index], name).toContain("Cerrado");
      expect(items[index], name).toContain(dates);
      expect(items[index], name).toContain(outcome);
    });
    expect(items[2]).not.toContain("No completada");

    const requests = fetchMock.mock.calls.map(([input, init]) =>
      describeRequest(input, init),
    );
    expect(requests).toHaveLength(1);
    expect(requests[0]).toMatchObject({
      path: historyPath,
      authorization: `Bearer ${sessionToken}`,
    });
  });

  // Regla de la interfaz: en el sprint abierto en el que está la historia todavía no hay resultado,
  // así que no se muestra ninguno.
  it("no muestra resultado en el sprint abierto", async () => {
    fetchMock.mockImplementation(
      serve([
        threeSprints[0],
        historyItem(
          "4d5e6f7a-8b9c-4d0e-9f1a-2b3c4d5e6f70",
          "Sprint 2",
          "active",
          "2026-10-15",
          "2026-10-28",
          null,
        ),
      ]),
    );
    renderHistory();

    const items = await shownItems();
    expect(items).toHaveLength(2);
    expect(items[0]).toContain("No completada");
    expect(items[1]).toContain("Activo");
    expect(items[1]).not.toMatch(/completada/i);
  });

  // Regla de la interfaz: una historia que nunca estuvo en un sprint lo indica, sin lista y sin
  // presentarlo como error.
  it("indica que la historia todavía no estuvo en ningún sprint", async () => {
    fetchMock.mockImplementation(serve([]));
    renderHistory();

    const region = await screen.findByRole("region", { name: regionName });
    expect(
      await within(region).findByText(/todavía no estuvo en ningún sprint/i),
    ).toBeTruthy();
    expect(within(region).queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Regla de la interfaz: la historia inexistente o ajena llega como 404 y se muestra su mensaje.
  it("muestra el rechazo de una historia inexistente", async () => {
    fetchMock.mockImplementation(async () =>
      jsonResponse(404, { error: storyNotFoundMessage }),
    );
    renderHistory();

    expect((await screen.findByRole("alert")).textContent).toContain(
      storyNotFoundMessage,
    );
  });
});
