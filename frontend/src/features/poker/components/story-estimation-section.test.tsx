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
  OpenSessionExistsResponse,
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
