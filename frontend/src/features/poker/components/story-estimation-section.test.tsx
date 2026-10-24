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
import { StoryEstimationSection } from "@/features/poker/components/story-estimation-section";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse } from "@/types/auth";
import type {
  EstimationSession,
  EstimationSessionSummary,
  OpenSessionExistsResponse,
  StoryEstimationHistory,
} from "@/types/poker";

// Pruebas de la sección "Planning Poker" del detalle de una historia (T017). Atraviesan el
// componente, la llamada de apertura de features/poker/api.ts (T030) y el cliente HTTP real; solo se
// sustituyen fetch, que es la frontera con el backend, y el enrutador de Next.js. Las reglas de la
// apertura (una sola sesión abierta, historia Completada, proyecto Finalizado) las resuelve el
// backend: acá se verifica qué envía la sección y qué muestra con cada respuesta.
//
// Contrato que fijan estas pruebas para StoryEstimationSection:
// - Recibe projectId y storyId y muestra una región con el título "Planning Poker" que ofrece el
//   botón "Abrir Planning Poker".
// - El botón envía POST /api/v1/stories/{storyId}/estimation-sessions con la sesión de la pestaña.
// - Con un 201 lleva a la sala de la sesión creada:
//   /projects/{projectId}/backlog/{storyId}/poker/{sessionId}.
// - Con el 409 de sesión en curso (OpenSessionExistsResponse) muestra el mensaje con role="alert"
//   y ofrece el enlace "Unirse" a la sala de la sesión existente, sin navegar por su cuenta.
// - Con cualquier otro rechazo muestra su mensaje con role="alert", no ofrece "Unirse" y no navega.

// Sustituye el enrutador de Next.js: fuera del App Router no hay enrutador montado, y así la prueba
// observa a qué sala lleva la sección al abrir la sesión.
const router = vi.hoisted(() => ({
  push: vi.fn<(href: string) => void>(),
  replace: vi.fn<(href: string) => void>(),
  refresh: vi.fn<() => void>(),
  back: vi.fn<() => void>(),
  forward: vi.fn<() => void>(),
  prefetch: vi.fn<(href: string) => void>(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => router,
  useSearchParams: () => new URLSearchParams(""),
}));

type FetchMock = Mock<typeof fetch>;
type ResponseBody =
  EstimationSession | OpenSessionExistsResponse | ErrorResponse;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const projectId = "9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35";
const storyId = "3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f";
const openPath = `/api/v1/stories/${storyId}/estimation-sessions`;
const regionName = "Planning Poker";
const openButtonName = "Abrir Planning Poker";

// Mensajes de contracts/openapi.yaml para los rechazos de la apertura.
const sessionInProgressMessage =
  "La historia ya tiene una sesión de estimación en curso.";
const completedMessage = "Una historia completada no se estima.";
const readOnlyMessage = "El proyecto está finalizado y es de solo lectura.";

// Ruta de la sala en vivo de una sesión de la historia.
function roomPath(sessionId: string): string {
  return `/projects/${projectId}/backlog/${storyId}/poker/${sessionId}`;
}

// Quien abre la sesión.
const ana = {
  id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
  fullName: "Ana Pérez",
};

// Sesión recién abierta por Ana, tal como la devuelve el 201: Abierta, con la ronda 1 En votación y
// Ana como facilitadora y única participante conectada (US1-1).
const openedSession: EstimationSession = {
  id: "6f1c2a52-7d0e-4c8b-9a35-0b0c1f3e9d11",
  version: 1,
  status: "open",
  story: {
    id: storyId,
    projectId,
    title: "Exportar el reporte de velocidad a PDF",
    description: "El reporte se descarga desde el tablero del proyecto.",
    acceptanceCriteria: [
      "El PDF incluye la velocidad de los últimos cinco sprints.",
      "Al pie figura la fecha de generación.",
    ],
  },
  facilitator: ana,
  openedBy: ana,
  openedAt: "2026-10-23T14:00:00Z",
  closedBy: null,
  closedAt: null,
  agreedStoryPoints: null,
  cancelReason: null,
  cards: ["0", "1", "2", "3", "5", "8", "13", "20", "40", "100", "?"],
  participants: [
    { user: ana, connected: true, isFacilitator: true, hasVoted: false },
  ],
  rounds: [
    {
      number: 1,
      status: "voting",
      openedAt: "2026-10-23T14:00:00Z",
      revealedAt: null,
      votes: null,
      outcome: null,
      notVoted: [],
    },
  ],
  myVote: null,
  facilitatorAvailableAt: null,
};

// Sesión que ya estaba abierta sobre la historia cuando otro integrante intenta abrir una segunda.
const existingSessionId = "1d2e3f4a-5b6c-4d7e-8f9a-0b1c2d3e4f5a";

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

// Simula el backend de la apertura respondiendo al POST con el estado y el cuerpo indicados.
// Cualquier otra ruta responde 404, para que una llamada inesperada no pase inadvertida.
function serveOpen(status: number, body: ResponseBody) {
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const request = describeRequest(input, init);
    if (request.method === "POST" && request.path === openPath) {
      return jsonResponse(status, body);
    }
    return jsonResponse(404, { error: "Ruta inesperada en la prueba." });
  };
}

// Devuelve los pedidos de apertura enviados, ya interpretados.
function openRequests(fetchMock: FetchMock) {
  return fetchMock.mock.calls
    .map(([input, init]) => describeRequest(input, init))
    .filter((request) => request.method === "POST");
}

// Monta la sección dentro del proveedor de sesión, como en el layout raíz, y devuelve su región.
async function renderSection(): Promise<HTMLElement> {
  render(
    <SessionProvider>
      <StoryEstimationSection projectId={projectId} storyId={storyId} />
    </SessionProvider>,
  );
  return screen.findByRole("region", { name: regionName });
}

// Pulsa el botón de apertura de la sección.
function clickOpen(region: HTMLElement) {
  fireEvent.click(within(region).getByRole("button", { name: openButtonName }));
}

// Verifica que se envió un único pedido de apertura con la sesión de la pestaña.
function expectSingleOpenRequest(fetchMock: FetchMock) {
  const requests = openRequests(fetchMock);
  expect(requests).toHaveLength(1);
  expect(requests[0]).toMatchObject({
    path: openPath,
    authorization: `Bearer ${sessionToken}`,
  });
}

// Verifica que un rechazo muestra su mensaje y deja la pantalla donde estaba, sin ofrecer Unirse.
async function expectRejected(
  fetchMock: FetchMock,
  region: HTMLElement,
  message: string,
) {
  const alert = await within(region).findByRole("alert");
  expect(alert.textContent).toContain(message);
  expect(within(region).queryByRole("link", { name: "Unirse" })).toBeNull();
  expect(router.push).not.toHaveBeenCalled();
  expectSingleOpenRequest(fetchMock);
}

describe("005/US1 - Abrir una sesión y reunir al equipo", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña, un fetch sustituido y el enrutador sin
  // llamadas previas.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(serveOpen(201, openedSession));
    vi.stubGlobal("fetch", fetchMock);
    router.push.mockClear();
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: sobre una historia Pendiente de un proyecto En curso, el integrante abre la sesión
  // con el botón; la sección envía la apertura con su sesión y lo lleva a la sala de la sesión
  // creada, donde queda como facilitador (FR-001, FR-002).
  it("Apertura de sesión", async () => {
    const region = await renderSection();

    clickOpen(region);

    await waitFor(() => expect(router.push).toHaveBeenCalledTimes(1));
    expect(router.push).toHaveBeenCalledWith(roomPath(openedSession.id));
    expectSingleOpenRequest(fetchMock);
    expect(within(region).queryByRole("alert")).toBeNull();
  });

  // Caso de error: la historia ya tiene una sesión Abierta; la apertura se rechaza indicando que hay
  // una sesión en curso y la sección ofrece unirse a esa sesión, sin crear otra (FR-006).
  it("Segunda sesión abierta", async () => {
    fetchMock.mockImplementation(
      serveOpen(409, {
        error: sessionInProgressMessage,
        sessionId: existingSessionId,
      }),
    );
    const region = await renderSection();

    clickOpen(region);

    const alert = await within(region).findByRole("alert");
    expect(alert.textContent).toContain(sessionInProgressMessage);
    const join = within(region).getByRole("link", { name: "Unirse" });
    expect(join.getAttribute("href")).toBe(roomPath(existingSessionId));
    expect(router.push).not.toHaveBeenCalled();
    expectSingleOpenRequest(fetchMock);
  });

  // Caso de error: una historia Completada no se estima; la sección muestra el motivo del rechazo
  // (FR-007).
  it("Historia completada", async () => {
    fetchMock.mockImplementation(serveOpen(409, { error: completedMessage }));
    const region = await renderSection();

    clickOpen(region);

    await expectRejected(fetchMock, region, completedMessage);
  });

  // Caso de error: en un proyecto Finalizado no se abren sesiones porque es de solo lectura; la
  // sección muestra el motivo del rechazo (FR-008).
  it("Proyecto finalizado", async () => {
    fetchMock.mockImplementation(serveOpen(409, { error: readOnlyMessage }));
    const region = await renderSection();

    clickOpen(region);

    await expectRejected(fetchMock, region, readOnlyMessage);
  });

  // Regla de la interfaz: la sección ofrece abrir la sesión y no envía la apertura hasta que el
  // integrante pulsa el botón.
  it("ofrece Abrir Planning Poker sin abrir la sesión por su cuenta", async () => {
    const region = await renderSection();

    expect(
      within(region).getByRole("button", { name: openButtonName }),
    ).toBeTruthy();
    expect(openRequests(fetchMock)).toHaveLength(0);
    expect(router.push).not.toHaveBeenCalled();
  });
});

// Pruebas del historial de estimación en la sección "Planning Poker" (T087). Igual que las de la
// apertura, atraviesan el componente, la llamada de features/poker/api.ts y el cliente HTTP real, y
// solo sustituyen fetch y el enrutador. El orden de las sesiones, el ocultamiento de los votos de la
// sesión abierta y la indicación de historia sin sesiones los resuelve el backend: acá se verifica
// qué pide la sección y qué muestra con cada respuesta.
//
// Contrato que fijan estas pruebas para el historial de StoryEstimationSection:
// - Al montarse envía GET /api/v1/stories/{storyId}/estimation-sessions con la sesión de la pestaña,
//   sin esperar a que se pulse ningún botón.
// - Lista las sesiones cerradas en una lista con el nombre accesible "Historial de estimación", un
//   elemento por sesión y en el orden recibido. Cada elemento muestra el estado en español
//   (Finalizada o Cancelada), el valor acordado como "<n> Story Points" solo si lo hay, el motivo de
//   una cancelación del sistema en español, la cantidad de rondas ("1 ronda", "<n> rondas") y el
//   enlace "Ver sesión" a /projects/{projectId}/backlog/{storyId}/poker/{sessionId}.
// - Con una sesión abierta muestra "Sesión en curso" con el nombre del facilitador, "Ronda <n>" y el
//   enlace "Unirse" a la sala de esa sesión.
// - Con emptyReason no_sessions muestra "La historia no tiene sesiones de estimación." sin
//   role="alert", porque no es un error, y no muestra la lista.

const historyListName = "Historial de estimación";
const noSessionsMessage = "La historia no tiene sesiones de estimación.";

// Integrantes que abren, facilitan y cierran las sesiones del historial.
const bruno = {
  id: "7c2d4e6f-8a9b-4c0d-9e1f-2a3b4c5d6e7f",
  fullName: "Bruno Díaz",
};

// Sesión Finalizada con tres rondas y 5 Story Points acordados, abierta y cerrada por Ana.
const finishedSession: EstimationSessionSummary = {
  id: "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d",
  status: "finished",
  openedBy: ana,
  openedAt: "2026-10-20T10:00:00Z",
  closedBy: ana,
  closedAt: "2026-10-20T10:25:00Z",
  agreedStoryPoints: 5,
  cancelReason: null,
  roundCount: 3,
};

// Sesión Finalizada más reciente que la anterior, con 13 Story Points acordados en dos rondas.
const laterFinishedSession: EstimationSessionSummary = {
  id: "b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e",
  status: "finished",
  openedBy: bruno,
  openedAt: "2026-10-22T15:00:00Z",
  closedBy: bruno,
  closedAt: "2026-10-22T15:10:00Z",
  agreedStoryPoints: 13,
  cancelReason: null,
  roundCount: 2,
};

// Sesión Cancelada por el sistema por inactividad después de una ronda, sin valor acordado.
const canceledSession: EstimationSessionSummary = {
  id: "c3d4e5f6-a7b8-4c9d-8e0f-2a3b4c5d6e7f",
  status: "canceled",
  openedBy: ana,
  openedAt: "2026-10-21T09:00:00Z",
  closedBy: null,
  closedAt: "2026-10-21T10:00:00Z",
  agreedStoryPoints: null,
  cancelReason: "inactivity",
  roundCount: 1,
};

// Construye la respuesta del historial con la sesión abierta y las sesiones cerradas indicadas.
function history(
  sessions: EstimationSessionSummary[],
  openSession: StoryEstimationHistory["openSession"] = null,
): StoryEstimationHistory {
  return { openSession, sessions, emptyReason: null };
}

// Simula el backend del historial respondiendo al GET con el cuerpo indicado. Cualquier otra ruta
// responde 404, para que una llamada inesperada no pase inadvertida.
function serveHistory(body: StoryEstimationHistory) {
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const request = describeRequest(input, init);
    if (request.method === "GET" && request.path === openPath) {
      return new Response(JSON.stringify(body), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }
    return jsonResponse(404, { error: "Ruta inesperada en la prueba." });
  };
}

// Verifica que se envió un único pedido del historial con la sesión de la pestaña.
function expectSingleHistoryRequest(fetchMock: FetchMock) {
  const requests = fetchMock.mock.calls
    .map(([input, init]) => describeRequest(input, init))
    .filter((request) => request.method === "GET");
  expect(requests).toHaveLength(1);
  expect(requests[0]).toMatchObject({
    path: openPath,
    authorization: `Bearer ${sessionToken}`,
  });
}

// Espera a que la sección muestre la lista del historial y devuelve sus elementos en orden.
async function findHistoryItems(region: HTMLElement): Promise<HTMLElement[]> {
  const list = await within(region).findByRole("list", {
    name: historyListName,
  });
  return within(list).getAllByRole("listitem");
}

// Verifica que un elemento del historial enlaza a la vista de su sesión.
function expectSessionLink(item: HTMLElement, sessionId: string) {
  const link = within(item).getByRole("link", { name: "Ver sesión" });
  expect(link.getAttribute("href")).toBe(roomPath(sessionId));
}

describe("005/US8 - Consultar el historial de estimación de una historia", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con una sesión válida en la pestaña, un fetch sustituido que responde un
  // historial vacío y el enrutador sin llamadas previas.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    fetchMock = vi.fn<typeof fetch>(
      serveHistory({
        openSession: null,
        sessions: [],
        emptyReason: "no_sessions",
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    router.push.mockClear();
  });

  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: la historia tiene una sesión Finalizada de tres rondas; la sección la lista con su
  // estado, su valor acordado y su cantidad de rondas, y enlaza a la sesión para ver sus rondas y
  // votos (US8-1, FR-079).
  it("Sesión finalizada con rondas", async () => {
    fetchMock.mockImplementation(serveHistory(history([finishedSession])));
    const region = await renderSection();

    const items = await findHistoryItems(region);

    expect(items).toHaveLength(1);
    expect(items[0].textContent).toContain("Finalizada");
    expect(items[0].textContent).toContain("5 Story Points");
    expect(items[0].textContent).toContain("3 rondas");
    expectSessionLink(items[0], finishedSession.id);
    expectSingleHistoryRequest(fetchMock);
    expect(router.push).not.toHaveBeenCalled();
  });

  // Caso normal: la historia se estimó dos veces; la sección muestra ambas sesiones en el orden
  // recibido, de la más reciente a la más antigua, cada una con su valor acordado (US8-2, FR-080).
  it("Orden de sesiones", async () => {
    fetchMock.mockImplementation(
      serveHistory(history([laterFinishedSession, finishedSession])),
    );
    const region = await renderSection();

    const items = await findHistoryItems(region);

    expect(items).toHaveLength(2);
    expect(items[0].textContent).toContain("13 Story Points");
    expect(items[0].textContent).toContain("2 rondas");
    expectSessionLink(items[0], laterFinishedSession.id);
    expect(items[1].textContent).toContain("5 Story Points");
    expect(items[1].textContent).toContain("3 rondas");
    expectSessionLink(items[1], finishedSession.id);
  });

  // Caso alternativo: la sesión Cancelada figura como Cancelada, sin valor acordado, con su motivo
  // y con sus rondas (US8-3, FR-063).
  it("Sesión cancelada", async () => {
    fetchMock.mockImplementation(serveHistory(history([canceledSession])));
    const region = await renderSection();

    const items = await findHistoryItems(region);

    expect(items).toHaveLength(1);
    expect(items[0].textContent).toContain("Cancelada");
    expect(items[0].textContent).toContain("Por inactividad");
    expect(items[0].textContent).toContain("1 ronda");
    expect(items[0].textContent).not.toContain("Story Points");
    expectSessionLink(items[0], canceledSession.id);
  });

  // Caso alternativo: la historia tiene una sesión Abierta; la sección indica que hay una sesión en
  // curso con su facilitador y su número de ronda, y ofrece unirse a ella (US8-4, FR-081).
  it("Sesión en curso", async () => {
    const openSession = {
      id: existingSessionId,
      facilitator: bruno,
      roundNumber: 2,
      roundStatus: "voting" as const,
    };
    fetchMock.mockImplementation(
      serveHistory(history([finishedSession], openSession)),
    );
    const region = await renderSection();

    const join = await within(region).findByRole("link", { name: "Unirse" });

    expect(join.getAttribute("href")).toBe(roomPath(existingSessionId));
    expect(region.textContent).toContain("Sesión en curso");
    expect(region.textContent).toContain(bruno.fullName);
    expect(region.textContent).toContain("Ronda 2");
    expect(region.textContent).not.toContain(noSessionsMessage);
    expect(await findHistoryItems(region)).toHaveLength(1);
    expect(router.push).not.toHaveBeenCalled();
  });

  // Caso límite: la historia nunca se estimó por Planning Poker; la sección muestra la indicación de
  // que no tiene sesiones, sin tratarlo como error y sin lista (US8-5, FR-083).
  it("Historia sin sesiones", async () => {
    const region = await renderSection();

    expect(await within(region).findByText(noSessionsMessage)).toBeTruthy();
    expect(
      within(region).queryByRole("list", { name: historyListName }),
    ).toBeNull();
    expect(within(region).queryByRole("alert")).toBeNull();
    expect(within(region).queryByRole("link", { name: "Unirse" })).toBeNull();
    expectSingleHistoryRequest(fetchMock);
  });
});
