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
import { PokerRoom } from "@/features/poker/components/poker-room";
import { writeToken } from "@/lib/session-storage";
import { SessionProvider } from "@/providers/session-provider";
import type { ErrorResponse, UserProfile } from "@/types/auth";
import type {
  EstimationRound,
  EstimationSession,
  UserRef,
} from "@/types/poker";
import { formatInstant } from "@/utils/format-date";

// Pruebas de la toma del rol de facilitador (T077) y de la cancelación de la sesión (T104) desde la
// sala en vivo. Atraviesan la sala, el
// hook del canal, la llamada takeEstimationFacilitator de features/poker/api.ts y el cliente HTTP
// real; solo se sustituye fetch, que es la frontera con el backend. Si el rol se puede tomar lo
// resuelve el backend (FR-068, FR-069): acá se verifica que la sala envíe el pedido y muestre el
// motivo de un rechazo.
//
// Contrato que fijan estas pruebas para PokerRoom:
// - Con la sesión abierta, la sección Participantes muestra TakeFacilitatorButton con
//   facilitatorAvailableAt de la instantánea; con la sesión cerrada no lo muestra.
// - Pulsar "Tomar el rol" envía un único POST /api/v1/estimation-sessions/{sessionId}/facilitator,
//   sin cuerpo y con la sesión de la pestaña. La sesión resultante llega por el canal.
// - Un rechazo muestra su mensaje con role="alert"; un pedido aceptado después limpia ese aviso.
// - Con la sesión abierta, el facilitador que pulsa "Cancelar sesión" y después "Confirmar
//   cancelación" envía un único POST /api/v1/estimation-sessions/{sessionId}/cancel, sin cuerpo y
//   con la sesión de la pestaña; un rechazo muestra su mensaje en el aviso de la sección de la ronda.

// Sustituye el enrutador de Next.js que usa la guardia de sesión: fuera del App Router no hay
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

// Rechazo de la toma del rol de contracts/openapi.yaml: el facilitador vigente y, si está
// desconectado, los segundos que faltan.
type FacilitatorUnavailableBody = {
  error: string;
  facilitator: UserRef;
  retryAfterSeconds?: number;
};
// Rechazo por sesión cerrada de contracts/openapi.yaml: el mensaje y el resultado registrado.
type SessionClosedBody = {
  error: string;
  status: EstimationSession["status"];
  agreedStoryPoints: number | null;
  cancelReason: EstimationSession["cancelReason"];
};
type PostReply = {
  status: number;
  body:
    | EstimationSession
    | FacilitatorUnavailableBody
    | SessionClosedBody
    | ErrorResponse;
};

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJicnVubyJ9.firma";
const sessionId = "6f1c2a52-7d0e-4c8b-9a35-0b0c1f3e9d11";
const eventsPath = `/api/v1/estimation-sessions/${sessionId}/events`;
const takePath = `/api/v1/estimation-sessions/${sessionId}/facilitator`;
const cancelPath = `/api/v1/estimation-sessions/${sessionId}/cancel`;
const profilePath = "/api/v1/users/me";
const takeButtonName = "Tomar el rol";

// Mensajes de contracts/openapi.yaml para los rechazos de la toma del rol.
const presentMessage = "El facilitador está conectado.";
const notYetMessage = "El rol de facilitador todavía no está disponible.";
const closedMessage = "La sesión de estimación ya está cerrada.";

// Ana facilita la sesión y Bruno es quien mira la sala.
const ana: UserRef = {
  id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
  fullName: "Ana Pérez",
};
const bruno: UserRef = {
  id: "8c2d4e6f-1a3b-4c5d-9e7f-0a1b2c3d4e5f",
  fullName: "Bruno Díaz",
};
const brunoProfile: UserProfile = { ...bruno, email: "bruno@example.com" };

// Instantánea vista por Bruno, único participante conectado, con Ana desconectada hace más de 5
// minutos: el rol quedó disponible hace un minuto según el reloj de la prueba.
function facilitatorGoneView(): EstimationSession {
  return {
    id: sessionId,
    version: 4,
    status: "open",
    story: {
      id: "3c4d5e6f-7a8b-4c9d-8e0f-1a2b3c4d5e6f",
      projectId: "9b2f1c0a-5d3e-4a71-8c62-0f4e7a1b9d35",
      title: "Exportar el reporte de velocidad a PDF",
      description: null,
      acceptanceCriteria: [],
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
      { user: ana, connected: false, isFacilitator: true, hasVoted: false },
      { user: bruno, connected: true, isFacilitator: false, hasVoted: false },
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
    facilitatorAvailableAt: new Date(Date.now() - 60_000).toISOString(),
  };
}

// La misma sesión con Bruno como facilitador, como la devuelve el 200 de la toma del rol.
function takenView(): EstimationSession {
  const view = facilitatorGoneView();
  return {
    ...view,
    version: 5,
    facilitator: bruno,
    participants: [
      { user: ana, connected: false, isFacilitator: false, hasVoted: false },
      { user: bruno, connected: true, isFacilitator: true, hasVoted: false },
    ],
    facilitatorAvailableAt: null,
  };
}

// La misma sesión ya Finalizada con 8 story points, conservando el momento vencido del rol.
function finishedView(): EstimationSession {
  return {
    ...facilitatorGoneView(),
    status: "finished",
    closedBy: bruno,
    closedAt: "2026-10-23T14:30:00Z",
    agreedStoryPoints: 8,
  };
}

const encoder = new TextEncoder();

// Construye una respuesta JSON como la que devuelve el backend.
function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// Canal que entrega la instantánea y queda abierto hasta que la sala lo aborta al desmontarse, para
// que la prueba no dependa de la reconexión.
function openStream(session: EstimationSession, signal?: AbortSignal | null) {
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      controller.enqueue(
        encoder.encode(`event: state\ndata: ${JSON.stringify(session)}\n\n`),
      );
      signal?.addEventListener("abort", () =>
        controller.error(new DOMException("Aborted", "AbortError")),
      );
    },
  });
  return new Response(body, {
    status: 200,
    headers: { "Content-Type": "text/event-stream" },
  });
}

// Extrae la ruta, el método, la cabecera Authorization y el cuerpo de una llamada a fetch.
function describeRequest(input: RequestInfo | URL, init?: RequestInit) {
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  return {
    path: new URL(raw, "http://localhost").pathname,
    method: input instanceof Request ? input.method : (init?.method ?? "GET"),
    authorization: new Headers(init?.headers).get("Authorization"),
    body: init?.body,
  };
}

// Simula el backend de la sala: el canal entrega la instantánea indicada, el perfil propio es el de
// Bruno y cada POST a la ruta indicada recibe la siguiente respuesta encolada (agotada la cola, se
// repite la última). Cualquier otra ruta responde 404, para que una llamada inesperada no pase
// inadvertida.
function serveRoomPost(
  session: EstimationSession,
  postPath: string,
  ...replies: PostReply[]
) {
  let next = 0;
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const request = describeRequest(input, init);
    if (request.method === "GET" && request.path === eventsPath) {
      return openStream(session, init?.signal);
    }
    if (request.method === "GET" && request.path === profilePath) {
      return jsonResponse(200, brunoProfile);
    }
    if (request.method === "POST" && request.path === postPath) {
      const reply = replies[Math.min(next, replies.length - 1)];
      next += 1;
      return jsonResponse(reply.status, reply.body);
    }
    return jsonResponse(404, { error: "Ruta inesperada en la prueba." });
  };
}

// Simula el backend de la sala con las respuestas encoladas para la toma del rol.
function serveRoom(session: EstimationSession, ...takeReplies: PostReply[]) {
  return serveRoomPost(session, takePath, ...takeReplies);
}

// Devuelve los POST enviados a la ruta indicada, ya interpretados.
function postRequests(fetchMock: FetchMock, path: string) {
  return fetchMock.mock.calls
    .map(([input, init]) => describeRequest(input, init))
    .filter((request) => request.method === "POST" && request.path === path);
}

// Devuelve los pedidos de toma del rol enviados, ya interpretados.
function takeRequests(fetchMock: FetchMock) {
  return postRequests(fetchMock, takePath);
}

// Monta la sala dentro del proveedor de sesión, como en el layout raíz, y espera la instantánea.
async function renderRoom() {
  render(
    <SessionProvider>
      <PokerRoom sessionId={sessionId} />
    </SessionProvider>,
  );
  await screen.findByRole("heading", { name: "Participantes" });
}

// Pulsa el botón "Tomar el rol" de la sala.
async function clickTake() {
  fireEvent.click(await screen.findByRole("button", { name: takeButtonName }));
}

describe("005/US6 - Sobrevivir a las desconexiones y relevar al facilitador", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con la sesión de Bruno en la pestaña; fetch se sustituye en cada prueba.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
  });

  // Al desmontar la sala se aborta el canal; después se restaura fetch.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso límite: con Ana desconectada hace más de 5 minutos y Bruno como único conectado, Bruno
  // pulsa "Tomar el rol" y la sala envía un único pedido sin cuerpo con su sesión (FR-068).
  it("Único participante toma el rol", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveRoom(facilitatorGoneView(), { status: 200, body: takenView() }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await renderRoom();
    await clickTake();

    await waitFor(() => expect(takeRequests(fetchMock)).toHaveLength(1));
    expect(takeRequests(fetchMock)[0]).toMatchObject({
      authorization: `Bearer ${sessionToken}`,
      body: undefined,
    });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Caso de error: Ana volvió antes de que Bruno pulsara; el backend rechaza el pedido porque el
  // facilitador está presente y la sala muestra el motivo (FR-069).
  it("Facilitador presente", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveRoom(facilitatorGoneView(), {
        status: 409,
        body: { error: presentMessage, facilitator: ana },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await renderRoom();
    await clickTake();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(presentMessage);
    expect(takeRequests(fetchMock)).toHaveLength(1);
  });

  // Caso de error: el backend todavía no libera el rol (por ejemplo, por diferencia de relojes) y
  // rechaza el pedido indicando cuánto falta; la sala muestra el motivo (FR-069).
  it("Rol todavía no disponible", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveRoom(facilitatorGoneView(), {
        status: 409,
        body: {
          error: notYetMessage,
          facilitator: ana,
          retryAfterSeconds: 180,
        },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await renderRoom();
    await clickTake();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(notYetMessage);
    expect(takeRequests(fetchMock)).toHaveLength(1);
  });

  // Regla de la sala: un pedido aceptado después de un rechazo limpia el aviso anterior.
  it("limpia el aviso del rechazo cuando la toma del rol se acepta", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveRoom(
        facilitatorGoneView(),
        { status: 409, body: { error: notYetMessage, facilitator: ana } },
        { status: 200, body: takenView() },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    await renderRoom();
    await clickTake();
    await screen.findByRole("alert");

    await clickTake();
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
    expect(takeRequests(fetchMock)).toHaveLength(2);
  });

  // Regla de la sala: con la sesión cerrada no se ofrece tomar el rol, aunque la instantánea
  // conserve un momento vencido.
  it("no ofrece tomar el rol con la sesión cerrada", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveRoom(finishedView(), { status: 200, body: takenView() }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await renderRoom();

    expect(screen.queryByRole("button", { name: takeButtonName })).toBeNull();
  });
});

describe("005/US7 - Cancelar una sesión sin registrar estimación", () => {
  let fetchMock: FetchMock;

  // Cada prueba arranca con la sesión de Bruno en la pestaña; fetch se sustituye en cada prueba.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
  });

  // Al desmontar la sala se aborta el canal; después se restaura fetch.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: Bruno facilita la sesión abierta; pedir la cancelación no envía nada hasta que la
  // confirma, y al confirmarla la sala envía un único pedido sin cuerpo con su sesión (FR-058). La
  // sesión Cancelada llega por el canal; la conservación de la estimación la verifica el backend.
  it("Cancelación conserva estimación", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveRoomPost(takenView(), cancelPath, {
        status: 200,
        body: { ...takenView(), status: "canceled", closedBy: bruno },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await renderRoom();
    fireEvent.click(
      await screen.findByRole("button", { name: "Cancelar sesión" }),
    );
    expect(postRequests(fetchMock, cancelPath)).toHaveLength(0);

    fireEvent.click(
      await screen.findByRole("button", { name: "Confirmar cancelación" }),
    );

    await waitFor(() =>
      expect(postRequests(fetchMock, cancelPath)).toHaveLength(1),
    );
    expect(postRequests(fetchMock, cancelPath)[0]).toMatchObject({
      authorization: `Bearer ${sessionToken}`,
      body: undefined,
    });
    expect(screen.queryByRole("alert")).toBeNull();
  });

  // Regla de la sala: si el backend rechaza la cancelación confirmada (por ejemplo, porque la sesión
  // ya se cerró), el mensaje recibido se muestra en el aviso de la sección de la ronda (FR-060).
  it("muestra en el aviso de la ronda el motivo de una cancelación rechazada", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveRoomPost(takenView(), cancelPath, {
        status: 409,
        body: {
          error: closedMessage,
          status: "finished",
          agreedStoryPoints: 8,
          cancelReason: null,
        },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await renderRoom();
    fireEvent.click(
      await screen.findByRole("button", { name: "Cancelar sesión" }),
    );
    fireEvent.click(
      await screen.findByRole("button", { name: "Confirmar cancelación" }),
    );

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain(closedMessage);
    const roundSection = screen
      .getByRole("heading", { name: "Ronda 1" })
      .closest("section");
    expect(roundSection?.contains(alert)).toBe(true);
    expect(postRequests(fetchMock, cancelPath)).toHaveLength(1);
  });
});

// Pruebas de la vista de una sesión cerrada (T087a). Si el canal responde 409 porque la sesión ya
// está Finalizada o Cancelada (FR-018), la pantalla no se une ni muestra el rechazo como error:
// consulta la sesión con GET /api/v1/estimation-sessions/{sessionId}, que no une a quien pide, y
// muestra su resultado con todas sus rondas (US8-1, US8-3, US8-6). Solo se sustituye fetch.
//
// Contrato que fijan estas pruebas para la vista cerrada:
// - Muestra la fecha (DD/MM/AAAA en la zona del navegador) de apertura y de cierre, el nombre de
//   quien la abrió y el de quien la cerró, o "El sistema" si closedBy es null.
// - Muestra el estado (Finalizada o Cancelada) y el valor acordado como "<n> Story Points".
// - Muestra todas las rondas, incluida la última, cada una con su lista "Votos" y su región
//   "Resultado de la ronda", como RoundResult.

// Carla abrió la sesión y Diego la cerró; ninguno votó, así que sus nombres solo pueden aparecer
// como autores de la apertura y del cierre. Elena votó y después dejó de integrar el proyecto.
const carla: UserRef = {
  id: "1d2e3f40-5a6b-4c7d-8e9f-0a1b2c3d4e50",
  fullName: "Carla Gómez",
};
const diego: UserRef = {
  id: "2e3f4051-6b7c-4d8e-9f0a-1b2c3d4e5f61",
  fullName: "Diego Romero",
};
const elena: UserRef = {
  id: "3f405162-7c8d-4e9f-8a1b-2c3d4e5f6a72",
  fullName: "Elena Ruiz",
};
const sessionPath = `/api/v1/estimation-sessions/${sessionId}`;

// Ronda revelada con los votos de Ana y Bruno y el resultado calculado por el backend.
function revealedRound(
  number: number,
  anaCard: "3" | "5" | "8",
  brunoCard: "8" | "13",
): EstimationRound {
  const consensus = anaCard === brunoCard;
  return {
    number,
    status: "revealed",
    openedAt: `2026-10-20T10:0${number}:00Z`,
    revealedAt: `2026-10-20T10:0${number}:30Z`,
    votes: [
      { user: ana, card: anaCard },
      { user: bruno, card: brunoCard },
    ],
    outcome: consensus
      ? { kind: "consensus", value: 8 }
      : {
          kind: "divergence",
          min: { value: Number(anaCard) as 3 | 5, authors: [ana] },
          max: { value: Number(brunoCard) as 8 | 13, authors: [bruno] },
        },
    notVoted: [],
  };
}

// Sesión Finalizada con 8 story points en su tercera ronda: abierta por Carla un día y cerrada por
// Diego al día siguiente, para que las dos fechas se distingan.
function closedFinishedView(): EstimationSession {
  return {
    ...facilitatorGoneView(),
    version: 12,
    status: "finished",
    facilitator: diego,
    openedBy: carla,
    openedAt: "2026-10-20T10:00:00Z",
    closedBy: diego,
    closedAt: "2026-10-21T10:30:00Z",
    agreedStoryPoints: 8,
    participants: [
      { user: ana, connected: false, isFacilitator: false, hasVoted: true },
      { user: bruno, connected: false, isFacilitator: false, hasVoted: true },
    ],
    rounds: [
      revealedRound(1, "3", "13"),
      revealedRound(2, "5", "13"),
      revealedRound(3, "8", "8"),
    ],
    facilitatorAvailableAt: null,
  };
}

// Sesión Cancelada por el sistema por inactividad, con sus dos rondas reveladas y sin valor.
function closedCanceledView(): EstimationSession {
  const view = closedFinishedView();
  return {
    ...view,
    status: "canceled",
    closedBy: null,
    agreedStoryPoints: null,
    cancelReason: "inactivity",
    rounds: view.rounds.slice(0, 2),
  };
}

// Simula el backend de una sesión ya cerrada: el canal responde 409 con el resultado registrado, el
// GET de la sesión la devuelve completa y el perfil propio es el de Bruno. Cualquier otra ruta
// responde 404.
function serveClosedSession(session: EstimationSession) {
  return async (input: RequestInfo | URL, init?: RequestInit) => {
    const request = describeRequest(input, init);
    if (request.method === "GET" && request.path === eventsPath) {
      return jsonResponse(409, {
        error: closedMessage,
        status: session.status,
        agreedStoryPoints: session.agreedStoryPoints,
        cancelReason: session.cancelReason,
      });
    }
    if (request.method === "GET" && request.path === sessionPath) {
      return jsonResponse(200, session);
    }
    if (request.method === "GET" && request.path === profilePath) {
      return jsonResponse(200, brunoProfile);
    }
    return jsonResponse(404, { error: "Ruta inesperada en la prueba." });
  };
}

// Devuelve los GET enviados a la ruta indicada, ya interpretados.
function getRequests(fetchMock: FetchMock, path: string) {
  return fetchMock.mock.calls
    .map(([input, init]) => describeRequest(input, init))
    .filter((request) => request.method === "GET" && request.path === path);
}

// Monta la pantalla de la sesión cerrada y espera las listas de votos de sus rondas.
async function renderClosedRoom() {
  render(
    <SessionProvider>
      <PokerRoom sessionId={sessionId} />
    </SessionProvider>,
  );
  return screen.findAllByRole("list", { name: "Votos" });
}

// Verifica la regla común de la vista cerrada: un único intento de canal, un único GET de la sesión
// con la sesión de la pestaña y ningún aviso de error.
function expectClosedViewRequests(fetchMock: FetchMock) {
  expect(getRequests(fetchMock, eventsPath)).toHaveLength(1);
  const sessionRequests = getRequests(fetchMock, sessionPath);
  expect(sessionRequests).toHaveLength(1);
  expect(sessionRequests[0].authorization).toBe(`Bearer ${sessionToken}`);
  expect(screen.queryByRole("alert")).toBeNull();
}

describe("005/US8 - Consultar el historial de estimación de una historia", () => {
  let fetchMock: FetchMock;
  const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone;

  // Cada prueba arranca con la sesión de Bruno en la pestaña; fetch se sustituye en cada prueba.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
  });

  // Se desmonta la pantalla y se restaura fetch.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // Caso normal: la sesión Finalizada de tres rondas muestra sus fechas de apertura y de cierre,
  // quién la abrió y quién la cerró, el valor acordado y cada ronda con los votos y su resultado
  // (US8-1, FR-018).
  it("Sesión finalizada con rondas", async () => {
    const view = closedFinishedView();
    fetchMock = vi.fn<typeof fetch>(serveClosedSession(view));
    vi.stubGlobal("fetch", fetchMock);

    const voteLists = await renderClosedRoom();

    const page = document.body.textContent ?? "";
    expect(page).toContain(formatInstant(view.openedAt, timeZone));
    expect(page).toContain(formatInstant(view.closedAt ?? "", timeZone));
    expect(page).toContain(carla.fullName);
    expect(page).toContain(diego.fullName);
    expect(page).toContain("Finalizada");
    expect(page).toContain("8 Story Points");

    expect(voteLists).toHaveLength(3);
    const expectedCards = [
      ["3", "13"],
      ["5", "13"],
      ["8", "8"],
    ];
    voteLists.forEach((list, index) => {
      const items = within(list).getAllByRole("listitem");
      expect(items).toHaveLength(2);
      expect(items[0].textContent).toContain(ana.fullName);
      expect(items[0].textContent).toContain(expectedCards[index][0]);
      expect(items[1].textContent).toContain(bruno.fullName);
      expect(items[1].textContent).toContain(expectedCards[index][1]);
    });
    const outcomes = screen
      .getAllByRole("region", { name: "Resultado de la ronda" })
      .map((region) => region.textContent ?? "");
    expect(outcomes).toHaveLength(3);
    expect(outcomes[0]).toContain("Divergencia");
    expect(outcomes[1]).toContain("Divergencia");
    expect(outcomes[2]).toContain("Consenso");

    expectClosedViewRequests(fetchMock);
  });

  // Caso alternativo: la sesión Cancelada por el sistema figura como Cancelada, sin valor acordado,
  // con su motivo y sus rondas completas, e indica que la cerró el sistema (US8-3, FR-063).
  it("Sesión cancelada", async () => {
    fetchMock = vi.fn<typeof fetch>(serveClosedSession(closedCanceledView()));
    vi.stubGlobal("fetch", fetchMock);

    const voteLists = await renderClosedRoom();

    const page = document.body.textContent ?? "";
    expect(page).toContain("Cancelada");
    expect(page).toContain("Por inactividad");
    expect(page).toContain("El sistema");
    expect(page).not.toContain(diego.fullName);
    expect(page).not.toContain("Story Points");
    expect(voteLists).toHaveLength(2);
    expect(
      screen.getAllByRole("region", { name: "Resultado de la ronda" }),
    ).toHaveLength(2);

    expectClosedViewRequests(fetchMock);
  });

  // Caso límite: Elena votó en la sesión y después dejó de integrar el proyecto; su voto sigue con
  // su nombre aunque ya no figure entre los participantes (US8-6, FR-019).
  it("Ex integrante", async () => {
    const view = closedFinishedView();
    const [first, ...rest] = view.rounds;
    const withElena: EstimationSession = {
      ...view,
      rounds: [
        {
          ...first,
          votes: [...(first.votes ?? []), { user: elena, card: "5" }],
        },
        ...rest,
      ],
    };
    fetchMock = vi.fn<typeof fetch>(serveClosedSession(withElena));
    vi.stubGlobal("fetch", fetchMock);

    const voteLists = await renderClosedRoom();

    const elenaVote = within(voteLists[0])
      .getAllByRole("listitem")
      .find((item) => item.textContent?.includes(elena.fullName));
    expect(elenaVote?.textContent).toContain("5");

    expectClosedViewRequests(fetchMock);
  });
});
