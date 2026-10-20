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
import { StoryList } from "@/features/backlog/components/story-list";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse, ValidationErrorResponse } from "@/types/auth";
import type {
  AppliedFilters,
  StoryListItem,
  StoryListResponse,
} from "@/types/story";

// Pruebas del listado del backlog (T031). Atraviesan el componente y el cliente HTTP real; solo se
// sustituye fetch, que es la frontera con el backend. El orden, el filtrado y el motivo del vacío
// los resuelve el backend (T030): acá se verifica que la vista pide el listado con los filtros
// elegidos y muestra lo que recibe, como lo recibe.
//
// Contrato que fijan estas pruebas para StoryList:
// - Recibe projectId y los filtros elegidos, y pide GET /api/v1/projects/{projectId}/stories con la
//   sesión de la pestaña; cada valor de un filtro viaja como parámetro repetido (?status=...).
// - Cada historia es un elemento de lista (role="listitem"), en el orden recibido, con su título,
//   la etiqueta en español de su prioridad y de su estado, "N SP" o "N Story Points" (el 0 incluido)
//   o "Sin estimar", la cantidad de criterios ("1 criterio", "N criterios", "0 criterios") y
//   "Lista para planificar" o "No lista para planificar".
// - Con backlog_empty muestra que el backlog todavía no tiene historias; con no_matches, que
//   ninguna historia coincide con los filtros, junto con los filtros aplicados en español. Los dos
//   mensajes son distintos y ninguno se presenta como error (role="alert").
// - Los rechazos del backend (400 por filtro inválido, 404 por proyecto inexistente) se muestran con
//   role="alert"; el del 400 incluye el detalle del campo con los valores admitidos.

// Sustituye el enrutador de Next.js por si el listado lo usa: fuera del App Router no hay
// enrutador montado.
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
type ResponseBody = StoryListResponse | ErrorResponse | ValidationErrorResponse;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const storiesPath = `/api/v1/projects/${projectId}/stories`;

const noFilters: AppliedFilters = { status: [], priority: [] };

const backlogEmptyMessage = /todavía no tiene historias/i;
const noMatchesMessage = /ninguna historia coincide/i;

// Historias del backlog tal como las ordena el backend (FR-032): Altas, Medias y Bajas y, dentro de
// cada prioridad, las más antiguas primero. Cubren los tres estados, Story Points en 5, 0, 8 y sin
// estimar, y 2, 1, 3 y 0 criterios.
const highOldest: StoryListItem = {
  id: "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
  title: "Exportar el reporte de velocidad a PDF",
  priority: "high",
  status: "pending",
  storyPoints: 5,
  acceptanceCriteriaCount: 2,
  readyToPlan: true,
};
const highNewest: StoryListItem = {
  id: "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",
  title: "Registrar el esfuerzo diario",
  priority: "high",
  status: "in_progress",
  storyPoints: 0,
  acceptanceCriteriaCount: 1,
  readyToPlan: true,
};
const mediumStory: StoryListItem = {
  id: "3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f",
  title: "Mostrar el burndown del sprint",
  priority: "medium",
  status: "completed",
  storyPoints: 8,
  acceptanceCriteriaCount: 3,
  readyToPlan: true,
};
const lowStory: StoryListItem = {
  id: "6d7e8f90-1a2b-4c3d-9e4f-5a6b7c8d9e0f",
  title: "Revisar los permisos de los integrantes",
  priority: "low",
  status: "pending",
  storyPoints: null,
  acceptanceCriteriaCount: 0,
  readyToPlan: false,
};

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(status: number, body: ResponseBody) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Hace que fetch responda siempre con el estado y el cuerpo indicados; cada llamada recibe una
// respuesta nueva porque el cuerpo de una Response solo se puede leer una vez.
function respondWith(fetchMock: FetchMock, status: number, body: ResponseBody) {
  fetchMock.mockImplementation(async () => jsonResponse(status, body));
}

// Arma la respuesta 200 del listado con las historias, los filtros y el motivo del vacío.
function listBody(
  stories: StoryListItem[],
  appliedFilters: AppliedFilters = noFilters,
  emptyReason: StoryListResponse["emptyReason"] = null,
): StoryListResponse {
  return { stories, appliedFilters, emptyReason };
}

// Extrae la ruta, los parámetros de consulta, el método y la cabecera Authorization de la llamada
// número index a fetch.
function requestAt(fetchMock: FetchMock, index: number) {
  const [input, init] = fetchMock.mock.calls[index];
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  const url = new URL(raw, "http://localhost");
  const headers =
    input instanceof Request ? input.headers : new Headers(init?.headers);
  return {
    path: url.pathname,
    query: url.searchParams,
    method: input instanceof Request ? input.method : (init?.method ?? "GET"),
    authorization: headers.get("Authorization"),
  };
}

// Monta el listado dentro del proveedor de sesión, como en el layout raíz.
function renderStoryList(filters: AppliedFilters = noFilters) {
  return render(
    <SessionProvider>
      <StoryList projectId={projectId} filters={filters} />
    </SessionProvider>,
  );
}

// Espera a que se muestren las historias y devuelve el texto de cada elemento de la lista.
async function shownItems(): Promise<string[]> {
  const items = await screen.findAllByRole("listitem");
  return items.map((item) => item.textContent ?? "");
}

// Indica si el texto de una fila muestra los Story Points dados como "N SP" o "N Story Points".
function showsStoryPoints(text: string, points: number): boolean {
  return new RegExp(`\\b${points} (SP|Story Points)\\b`).test(text);
}

describe("003/US2 - Ver el backlog ordenado y filtrado", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch que devuelve el backlog
  // completo, ya ordenado por el backend.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>();
    respondWith(
      fetchMock,
      200,
      listBody([highOldest, highNewest, mediumStory, lowStory]),
    );
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: sin filtros, el listado pide GET /stories del proyecto con la sesión de la pestaña
  // y muestra las historias en el orden recibido (Altas, Medias y Bajas, las más antiguas primero),
  // cada una con prioridad, estado, Story Points o "Sin estimar", cantidad de criterios y si está
  // lista para planificar (FR-032, FR-033).
  it("Orden por prioridad y fecha", async () => {
    renderStoryList();

    const items = await shownItems();
    expect(items).toHaveLength(4);
    expect(items[0]).toContain(highOldest.title);
    expect(items[1]).toContain(highNewest.title);
    expect(items[2]).toContain(mediumStory.title);
    expect(items[3]).toContain(lowStory.title);

    expect(items[0]).toContain("Alta");
    expect(items[1]).toContain("Alta");
    expect(items[2]).toContain("Media");
    expect(items[3]).toContain("Baja");

    expect(items[0]).toContain("Pendiente");
    expect(items[1]).toContain("En progreso");
    expect(items[2]).toContain("Completada");
    expect(items[3]).toContain("Pendiente");

    expect(showsStoryPoints(items[0], 5)).toBe(true);
    expect(showsStoryPoints(items[1], 0)).toBe(true);
    expect(items[1]).not.toMatch(/sin estimar/i);
    expect(showsStoryPoints(items[2], 8)).toBe(true);
    expect(items[3]).toMatch(/sin estimar/i);

    expect(items[0]).toMatch(/\b2 criterios\b/);
    expect(items[1]).toMatch(/\b1 criterio\b/);
    expect(items[1]).not.toMatch(/\b1 criterios\b/);
    expect(items[2]).toMatch(/\b3 criterios\b/);
    expect(items[3]).toMatch(/\b0 criterios\b/);

    for (const ready of [items[0], items[1], items[2]]) {
      expect(ready).toMatch(/lista para planificar/i);
      expect(ready).not.toMatch(/no lista para planificar/i);
    }
    expect(items[3]).toMatch(/no lista para planificar/i);

    const request = requestAt(fetchMock, 0);
    expect(request.path.endsWith(storiesPath)).toBe(true);
    expect(request.method).toBe("GET");
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
    expect([...request.query.keys()]).toHaveLength(0);
  });

  // Caso alternativo: con el filtro de estado Pendiente, el listado lo envía como ?status=pending
  // y muestra solo las historias recibidas, en el orden recibido (FR-034).
  it("Filtro por estado", async () => {
    const filters: AppliedFilters = { status: ["pending"], priority: [] };
    respondWith(fetchMock, 200, listBody([highOldest, lowStory], filters));
    renderStoryList(filters);

    const items = await shownItems();
    expect(items).toHaveLength(2);
    expect(items[0]).toContain(highOldest.title);
    expect(items[1]).toContain(lowStory.title);

    const request = requestAt(fetchMock, 0);
    expect(request.query.getAll("status")).toEqual(["pending"]);
    expect(request.query.getAll("priority")).toEqual([]);
  });

  // Caso alternativo: los filtros de prioridad Alta y estado Pendiente viajan juntos en la misma
  // consulta y se muestran solo las historias que cumplen ambos (FR-034).
  it("Filtros combinados", async () => {
    const filters: AppliedFilters = { status: ["pending"], priority: ["high"] };
    respondWith(fetchMock, 200, listBody([highOldest], filters));
    renderStoryList(filters);

    const items = await shownItems();
    expect(items).toHaveLength(1);
    expect(items[0]).toContain(highOldest.title);

    const request = requestAt(fetchMock, 0);
    expect(request.query.getAll("status")).toEqual(["pending"]);
    expect(request.query.getAll("priority")).toEqual(["high"]);
  });

  // Varios valores en un mismo filtro viajan como parámetros repetidos, no separados por comas
  // (FR-034, research.md sección 9).
  it("envía cada valor de un filtro como parámetro repetido", async () => {
    const filters: AppliedFilters = {
      status: ["pending", "in_progress"],
      priority: ["high", "low"],
    };
    respondWith(
      fetchMock,
      200,
      listBody([highOldest, highNewest, lowStory], filters),
    );
    renderStoryList(filters);

    await shownItems();
    const request = requestAt(fetchMock, 0);
    expect(request.query.getAll("status")).toEqual(["pending", "in_progress"]);
    expect(request.query.getAll("priority")).toEqual(["high", "low"]);
  });

  // Caso límite: un proyecto sin historias muestra que el backlog todavía no tiene historias, sin
  // filas, sin presentarlo como error y sin el mensaje de filtros sin coincidencias (FR-035).
  it("Backlog vacío", async () => {
    respondWith(fetchMock, 200, listBody([], noFilters, "backlog_empty"));
    renderStoryList();

    expect(await screen.findByText(backlogEmptyMessage)).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText(noMatchesMessage)).toBeNull();
  });

  // Caso límite: filtros sin coincidencias muestran que ninguna historia coincide, con los filtros
  // aplicados en español, sin filas, sin presentarlo como error y sin el mensaje de backlog vacío
  // (FR-036).
  it("Filtro sin coincidencias", async () => {
    const filters: AppliedFilters = { status: [], priority: ["high"] };
    respondWith(fetchMock, 200, listBody([], filters, "no_matches"));
    renderStoryList(filters);

    expect(await screen.findByText(noMatchesMessage)).toBeTruthy();
    expect(document.body.textContent).toContain("Alta");
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.queryByText(backlogEmptyMessage)).toBeNull();
  });

  // Los dos vacíos se distinguen: el texto que se muestra ante backlog_empty no es el mismo que
  // ante no_matches, aunque en ambos casos la lista llegue vacía (FR-035, FR-036).
  it("muestra mensajes distintos para el backlog vacío y para los filtros sin coincidencias", async () => {
    respondWith(fetchMock, 200, listBody([], noFilters, "backlog_empty"));
    const emptyView = renderStoryList();
    await screen.findByText(backlogEmptyMessage);
    const emptyText = emptyView.container.textContent ?? "";
    emptyView.unmount();

    const filters: AppliedFilters = { status: [], priority: ["high"] };
    respondWith(fetchMock, 200, listBody([], filters, "no_matches"));
    const noMatchesView = renderStoryList(filters);
    await screen.findByText(noMatchesMessage);
    const noMatchesText = noMatchesView.container.textContent ?? "";

    expect(emptyText.trim()).not.toBe("");
    expect(noMatchesText.trim()).not.toBe("");
    expect(emptyText).not.toBe(noMatchesText);
  });

  // Caso límite: dos historias empatadas en prioridad y fecha llegan desempatadas por el backend;
  // consultadas dos veces, se muestran las dos veces en el orden recibido, sin que la vista las
  // reordene por título ni por ningún otro criterio (FR-032, RN-17).
  it("Orden estable ante empate", async () => {
    const tiedFirst: StoryListItem = {
      ...highOldest,
      id: "0f1e2d3c-4b5a-4968-8776-655443322110",
      title: "Validar el formato de las fechas",
    };
    const tiedSecond: StoryListItem = {
      ...highOldest,
      id: "f0e1d2c3-b4a5-4697-8879-5a4b3c2d1e0f",
      title: "Avisar al equipo del cambio de sprint",
    };
    respondWith(fetchMock, 200, listBody([tiedFirst, tiedSecond]));

    const firstView = renderStoryList();
    const first = await shownItems();
    firstView.unmount();
    renderStoryList();
    const second = await shownItems();

    expect(fetchMock).toHaveBeenCalledTimes(2);
    for (const items of [first, second]) {
      expect(items).toHaveLength(2);
      expect(items[0]).toContain(tiedFirst.title);
      expect(items[1]).toContain(tiedSecond.title);
    }
  });

  // Caso de error: si el backend rechaza un filtro con 400, el listado lo muestra como error con el
  // detalle de los valores admitidos y no muestra historias ni mensajes de vacío (FR-037).
  it("Filtro inválido", async () => {
    const fieldMessage =
      "El estado debe ser uno de: pending, in_progress, completed.";
    respondWith(fetchMock, 400, {
      error: "Hay campos inválidos en la solicitud.",
      fields: [{ field: "status", message: fieldMessage }],
    });
    renderStoryList({ status: ["pending"], priority: [] });

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(fieldMessage);
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByText(backlogEmptyMessage)).toBeNull();
    expect(screen.queryByText(noMatchesMessage)).toBeNull();
  });

  // Caso de error: quien no es integrante recibe 404 de proyecto inexistente; el listado muestra
  // ese mensaje como error y no revela ninguna historia ni un backlog vacío (FR-046, FR-047).
  it("No integrante", async () => {
    respondWith(fetchMock, 404, { error: "El proyecto no existe." });
    renderStoryList();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("El proyecto no existe.");
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByText(backlogEmptyMessage)).toBeNull();
    expect(screen.queryByText(noMatchesMessage)).toBeNull();
  });
});
