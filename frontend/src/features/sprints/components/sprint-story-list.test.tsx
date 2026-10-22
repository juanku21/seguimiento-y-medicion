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
import { SprintStoryList } from "@/features/sprints/components/sprint-story-list";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  SprintDetail,
  SprintStatus,
  SprintStoryItem,
} from "@/types/sprint";

// Pruebas de la lista de historias comprometidas en un sprint (T030). Atraviesan el componente, la
// llamada de baja de api.ts (T038) y el cliente HTTP real; solo se sustituye fetch, que es la
// frontera con el backend. Las reglas de la baja (RN-09, RN-21) las resuelve el backend: acá se
// verifica qué ofrece la lista, qué envía y qué muestra con cada respuesta.
//
// Contrato que fijan estas pruebas para SprintStoryList:
// - Recibe el sprint (SprintDetail) y onChanged, que recibe el SprintDetail devuelto por el backend
//   para que la pantalla de detalle lo muestre.
// - Cada historia del sprint es un elemento de lista (role="listitem"), en el orden recibido, con su
//   título, la etiqueta en español de su estado y sus Story Points ("N SP" o "N Story Points").
// - En un sprint abierto, cada historia que no está Completada tiene un botón "Quitar del sprint";
//   una historia Completada no lo tiene (RN-21) y en un sprint Cerrado ninguna lo tiene (RN-09).
// - El botón envía DELETE /api/v1/sprints/{sprintId}/stories?storyId={id} con la sesión de la
//   pestaña; con un 200 llama una vez a onChanged con el SprintDetail recibido y con un rechazo
//   muestra el motivo con role="alert" sin llamar a onChanged.
// - Un sprint sin historias muestra que no tiene historias comprometidas, sin presentarlo como
//   error.

// Sustituye el enrutador de Next.js por si la lista o la guardia de sesión lo usan: fuera del App
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
type ChangedMock = Mock<(sprint: SprintDetail) => void>;
type ResponseBody = SprintDetail | ErrorResponse;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "8c2d4e1a-5b3f-4a6c-9d7e-1f2a3b4c5d6e";
const sprintId = "6a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d";
const sprintStoriesPath = `/api/v1/sprints/${sprintId}/stories`;

// Mensaje de contracts/openapi.yaml para la baja de una historia Completada (US2-13).
const completedRemovalMessage =
  "No se puede quitar una historia Completada. Primero devolvela a En progreso.";
const removeButtonName = /quitar del sprint/i;
const emptySprintMessage = /no tiene historias/i;

// Historias comprometidas en el sprint: una Pendiente, una En progreso y una Completada.
const pendingStory: SprintStoryItem = {
  storyId: "1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d",
  title: "Exportar el reporte de velocidad a PDF",
  priority: "high",
  storyPoints: 5,
  status: "pending",
  completedAt: null,
  outcome: null,
};
const inProgressStory: SprintStoryItem = {
  storyId: "2b3c4d5e-6f7a-4b8c-9d0e-1f2a3b4c5d6e",
  title: "Registrar el esfuerzo diario",
  priority: "medium",
  storyPoints: 0,
  status: "in_progress",
  completedAt: null,
  outcome: null,
};
const completedStory: SprintStoryItem = {
  storyId: "3c4d5e6f-7a8b-4c9d-8e1f-2a3b4c5d6e7f",
  title: "Mostrar el burndown del sprint",
  priority: "low",
  storyPoints: 8,
  status: "completed",
  completedAt: "2026-10-20T15:00:00Z",
  outcome: null,
};

// Arma el sprint con el estado y las historias indicadas.
function sprintWith(
  status: SprintStatus,
  stories: SprintStoryItem[],
): SprintDetail {
  return {
    id: sprintId,
    projectId,
    name: "Sprint 2",
    goal: "Cerrar el alta de clientes",
    status,
    startDate: "2026-10-15",
    endDate: "2026-10-28",
    startedAt: status === "planned" ? null : "2026-10-15T12:00:00Z",
    closedAt: status === "closed" ? "2026-10-28T20:00:00Z" : null,
    createdBy: {
      id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
      fullName: "Ana Pérez",
    },
    closedBy: null,
    createdAt: "2026-10-10T12:00:00Z",
    updatedAt: "2026-10-10T12:00:00Z",
    stories,
    results: null,
    hoursPerStoryPoint: null,
    schedule: null,
  };
}

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

// Extrae la ruta, los identificadores de la consulta, el método y la cabecera Authorization de la
// llamada número index a fetch.
function requestAt(fetchMock: FetchMock, index: number) {
  const [input, init] = fetchMock.mock.calls[index];
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  const url = new URL(raw, "http://localhost");
  return {
    path: url.pathname,
    storyIds: url.searchParams.getAll("storyId"),
    method: input instanceof Request ? input.method : (init?.method ?? "GET"),
    authorization: new Headers(init?.headers).get("Authorization"),
  };
}

// Monta la lista dentro del proveedor de sesión, como en el layout raíz.
function renderList(sprint: SprintDetail, onChanged: ChangedMock) {
  return render(
    <SessionProvider>
      <SprintStoryList sprint={sprint} onChanged={onChanged} />
    </SessionProvider>,
  );
}

// Devuelve el elemento de lista que muestra la historia dada.
function itemOf(story: SprintStoryItem): HTMLElement {
  const item = screen
    .getAllByRole("listitem")
    .find((element) => element.textContent?.includes(story.title));
  expect(item, story.title).toBeDefined();
  return item as HTMLElement;
}

describe("004/US2 - Comprometer historias en un sprint", () => {
  let fetchMock: FetchMock;
  let onChanged: ChangedMock;

  // Cada prueba arranca con una sesión válida en la pestaña y un fetch que responde 200 con el
  // sprint Planificado sin la historia Pendiente.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    onChanged = vi.fn<(sprint: SprintDetail) => void>();
    fetchMock = vi.fn<typeof fetch>();
    respondWith(fetchMock, 200, sprintWith("planned", [inProgressStory]));
    vi.stubGlobal("fetch", fetchMock);
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso alternativo: la lista muestra cada historia con su estado y sus Story Points; al quitar una
  // de un sprint Planificado envía DELETE con su identificador y la sesión, y entrega a la pantalla
  // el detalle sin esa historia.
  it("Quitar de un sprint Planificado", async () => {
    const updated = sprintWith("planned", [inProgressStory]);
    renderList(
      sprintWith("planned", [pendingStory, inProgressStory]),
      onChanged,
    );

    const items = screen.getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(items[0].textContent).toContain(pendingStory.title);
    expect(items[0].textContent).toContain("Pendiente");
    expect(items[0].textContent).toMatch(/\b5 (SP|Story Points)\b/);
    expect(items[1].textContent).toContain("En progreso");
    expect(items[1].textContent).toMatch(/\b0 (SP|Story Points)\b/);

    fireEvent.click(
      within(itemOf(pendingStory)).getByRole("button", {
        name: removeButtonName,
      }),
    );

    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1));
    expect(onChanged).toHaveBeenCalledWith(updated);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const request = requestAt(fetchMock, 0);
    expect(request.method).toBe("DELETE");
    expect(request.path).toBe(sprintStoriesPath);
    expect(request.storyIds).toEqual([pendingStory.storyId]);
    expect(request.authorization).toBe(`Bearer ${sessionToken}`);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso límite: quitar la única historia de un sprint Activo no es un error; la pantalla recibe el
  // sprint Activo y vacío, y la lista vacía lo informa sin presentarlo como error.
  it("Quitar la única historia de un sprint Activo", async () => {
    const emptied = sprintWith("active", []);
    respondWith(fetchMock, 200, emptied);
    renderList(sprintWith("active", [pendingStory]), onChanged);

    fireEvent.click(
      within(itemOf(pendingStory)).getByRole("button", {
        name: removeButtonName,
      }),
    );

    await waitFor(() => expect(onChanged).toHaveBeenCalledWith(emptied));
    expect(screen.queryByRole("alert")).toBeNull();
    cleanup();

    renderList(emptied, onChanged);
    expect(screen.getByText(emptySprintMessage)).toBeTruthy();
    expect(screen.queryAllByRole("listitem")).toHaveLength(0);
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // RN-21: la lista no ofrece quitar una historia Completada, pero sí las Pendientes y En progreso
  // del mismo sprint Activo.
  it("no ofrece quitar una historia Completada", () => {
    renderList(
      sprintWith("active", [pendingStory, inProgressStory, completedStory]),
      onChanged,
    );

    expect(itemOf(completedStory).textContent).toContain("Completada");
    expect(
      within(itemOf(completedStory)).queryByRole("button", {
        name: removeButtonName,
      }),
    ).toBeNull();
    for (const story of [pendingStory, inProgressStory]) {
      expect(
        within(itemOf(story)).getByRole("button", { name: removeButtonName }),
        story.title,
      ).toBeTruthy();
    }
  });

  // Caso de error: si la historia se completó después de mostrarse la lista, el backend rechaza la
  // baja con el motivo; la lista lo muestra y la pantalla no recibe ningún detalle.
  it("Quitar una historia Completada", async () => {
    respondWith(fetchMock, 409, { error: completedRemovalMessage });
    renderList(sprintWith("active", [pendingStory]), onChanged);

    fireEvent.click(
      within(itemOf(pendingStory)).getByRole("button", {
        name: removeButtonName,
      }),
    );

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(completedRemovalMessage);
    expect(onChanged).not.toHaveBeenCalled();
  });

  // Caso de error: un sprint Cerrado no admite cambios, así que la lista muestra sus historias sin
  // ofrecer quitar ninguna.
  it("Sprint Cerrado", () => {
    renderList(
      sprintWith("closed", [
        { ...pendingStory, outcome: "not_completed" },
        { ...completedStory, outcome: "completed" },
      ]),
      onChanged,
    );

    expect(screen.getAllByRole("listitem")).toHaveLength(2);
    expect(screen.queryByRole("button", { name: removeButtonName })).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("004/US5 - Cerrar un sprint y congelar su resultado", () => {
  let onChanged: ChangedMock;

  // Cada prueba arranca con una sesión válida en la pestaña; la lista de un sprint Cerrado no llama
  // al backend, así que fetch se sustituye solo para detectar una llamada indebida.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    onChanged = vi.fn<(sprint: SprintDetail) => void>();
    vi.stubGlobal("fetch", vi.fn<typeof fetch>());
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal (FR-042, FR-043): el sprint Cerrado conserva el registro de sus historias con el
  // resultado que tuvo cada una. La historia no completada ya volvió al backlog como Pendiente, pero
  // la lista muestra su resultado congelado ("No completada") y no su estado vigente.
  it("Cierre parcial", () => {
    const returnedStory: SprintStoryItem = {
      ...pendingStory,
      status: "pending",
      outcome: "not_completed",
    };
    const doneStory: SprintStoryItem = {
      ...completedStory,
      status: "completed",
      outcome: "completed",
    };
    renderList(sprintWith("closed", [returnedStory, doneStory]), onChanged);

    expect(screen.getAllByRole("listitem")).toHaveLength(2);
    const returnedText = itemOf(returnedStory).textContent;
    expect(returnedText).toContain("No completada");
    expect(returnedText).not.toContain("Pendiente");
    expect(itemOf(doneStory).textContent).toContain("Completada");
    expect(itemOf(doneStory).textContent).not.toContain("No completada");
  });

  // Caso alternativo (FR-043, FR-044): una historia que no se completó en este sprint y después se
  // completó en otro tiene hoy el estado Completada en el backlog; el sprint Cerrado sigue mostrando
  // "No completada", porque el resultado congelado no cambia por modificaciones posteriores.
  it("Inmutable tras el cierre", () => {
    const laterCompleted: SprintStoryItem = {
      ...inProgressStory,
      status: "completed",
      completedAt: "2026-11-12T15:00:00Z",
      outcome: "not_completed",
    };
    renderList(sprintWith("closed", [laterCompleted]), onChanged);

    const text = itemOf(laterCompleted).textContent;
    expect(text).toContain("No completada");
    // "Completada" con mayúscula solo aparece si se muestra el estado vigente en lugar del resultado.
    expect(text).not.toMatch(/Completada/);
  });
});
