import {
  afterEach,
  beforeEach,
  describe,
  expect,
  it,
  vi,
  type Mock,
} from "vitest";
import { act, cleanup, renderHook } from "@testing-library/react";
import { useEstimationStream } from "@/features/poker/use-estimation-stream";
import { writeToken } from "@/lib/session-storage";
import type { ErrorResponse } from "@/types/auth";
import type { EstimationSession, SessionClosedResponse } from "@/types/poker";

// Pruebas del hook del canal de una sesión de Planning Poker (T073). Atraviesan el hook, la llamada
// streamEstimationSession de features/poker/api.ts, el cliente HTTP real y el lector de
// text/event-stream; solo se sustituye fetch, que es la frontera con el backend. Cada apertura del
// canal recibe la respuesta que la prueba encola: un cuerpo text/event-stream que entrega sus
// eventos y se cierra (el corte), un fallo de red o un rechazo con su código.
//
// Contrato que fijan estas pruebas para useEstimationStream (research.md, sección 18):
// - Cuando el canal se corta (el cuerpo termina o la red falla) con la sesión todavía abierta, lo
//   vuelve a abrir con espera creciente: 1 segundo después del primer corte, 2 después del segundo
//   intento fallido, 5 después del tercero y 5 de ahí en adelante (tope 5).
// - Mientras reconecta conserva la última instantánea y no expone error, para que la sala no se
//   reemplace por un aviso.
// - La instantánea que llega al reconectar reemplaza a la anterior.
// - Ante un rechazo 409 (sesión cerrada), 404 (sesión inexistente o ajena) o 401 (sesión inválida)
//   al reconectar deja de intentar y expone el error recibido.
// - Cualquier otro rechazo al reconectar (por ejemplo, 500 o 503) se trata como un corte: reintenta
//   con la misma espera creciente.
// - Una reconexión que entrega una instantánea reinicia la espera: tras el corte siguiente, el
//   primer intento vuelve a ocurrir a 1 segundo.
// - Con una instantánea de la sesión cerrada, o después de desmontar el hook, no reconecta.

type FetchMock = Mock<typeof fetch>;
type ResponseBody = ErrorResponse | SessionClosedResponse;

// Respuesta de una apertura del canal: se arma recién al pedirla, porque el cuerpo de una Response
// solo se puede leer una vez.
type StreamReply = () => Promise<Response>;

const sessionToken = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhbmEifQ.firma";
const sessionId = "6f1c2a52-7d0e-4c8b-9a35-0b0c1f3e9d11";
const eventsPath = `/api/v1/estimation-sessions/${sessionId}/events`;

// Mensajes de contracts/openapi.yaml para los rechazos del canal.
const closedMessage = "La sesión de estimación ya está cerrada.";
const notFoundMessage = "La sesión de estimación no existe.";
const invalidSessionMessage = "Se requiere iniciar sesión.";

// Participantes de la sesión.
const ana = {
  id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
  fullName: "Ana Pérez",
};
const bruno = {
  id: "8c2d4e6f-1a3b-4c5d-9e7f-0a1b2c3d4e5f",
  fullName: "Bruno Díaz",
};

// Instantánea vista por Bruno, que votó 13 en la ronda 1 En votación de una sesión Abierta que
// facilita Ana. version y connected se ajustan en cada prueba.
function brunoView(version: number, connected: boolean): EstimationSession {
  return {
    id: sessionId,
    version,
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
      { user: ana, connected: true, isFacilitator: true, hasVoted: false },
      { user: bruno, connected, isFacilitator: false, hasVoted: true },
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
    myVote: "13",
    facilitatorAvailableAt: null,
  };
}

// Instantánea de la misma sesión ya Finalizada con 8 story points, tal como llega por el canal.
function finishedView(version: number): EstimationSession {
  return {
    ...brunoView(version, true),
    status: "finished",
    closedBy: ana,
    closedAt: "2026-10-23T14:30:00Z",
    agreedStoryPoints: 8,
  };
}

const encoder = new TextEncoder();

// Canal que entrega un evento state por cada sesión indicada y después se cierra, como un corte.
function streamThenCut(...sessions: EstimationSession[]): StreamReply {
  return async () => {
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        sessions.forEach((session) =>
          controller.enqueue(
            encoder.encode(
              `event: state\ndata: ${JSON.stringify(session)}\n\n`,
            ),
          ),
        );
        controller.close();
      },
    });
    return new Response(body, {
      status: 200,
      headers: { "Content-Type": "text/event-stream" },
    });
  };
}

// Apertura que falla por la red, como cuando el equipo sigue sin conexión.
const networkFailure: StreamReply = async () => {
  throw new TypeError("Failed to fetch");
};

// Apertura rechazada con el código y el cuerpo JSON indicados.
function rejection(status: number, body: ResponseBody): StreamReply {
  return async () =>
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    });
}

// Extrae la ruta de una llamada a fetch.
function requestPath(input: RequestInfo | URL): string {
  const raw =
    typeof input === "string"
      ? input
      : input instanceof URL
        ? input.href
        : input.url;
  return new URL(raw, "http://localhost").pathname;
}

// Simula el backend del canal: cada apertura recibe la siguiente respuesta encolada y, agotada la
// cola, la última se repite. Cualquier otra ruta falla, para que una llamada inesperada no pase
// inadvertida.
function serveEvents(...replies: StreamReply[]) {
  let next = 0;
  return async (input: RequestInfo | URL) => {
    if (requestPath(input) !== eventsPath) {
      throw new Error(`Ruta inesperada en la prueba: ${requestPath(input)}`);
    }
    const reply = replies[Math.min(next, replies.length - 1)];
    next += 1;
    return reply();
  };
}

// Cuenta las aperturas del canal pedidas hasta el momento.
function openings(fetchMock: FetchMock): number {
  return fetchMock.mock.calls.filter(
    ([input]) => requestPath(input) === eventsPath,
  ).length;
}

// Deja correr las promesas y las tareas pendientes (lectura del cuerpo, respuestas de fetch) sin
// mover el reloj simulado.
async function settle() {
  await act(async () => {
    for (let i = 0; i < 5; i++) {
      await vi.advanceTimersByTimeAsync(0);
      await new Promise<void>((resolve) => setImmediate(resolve));
    }
  });
}

// Avanza el reloj simulado los milisegundos indicados y deja correr lo que eso dispare.
async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
  await settle();
}

describe("005/US6 - Sobrevivir a las desconexiones y relevar al facilitador", () => {
  let fetchMock: FetchMock;

  // Sesión de la pestaña y reloj simulado; solo se simulan los temporizadores y la fecha, no las
  // microtareas ni setImmediate, de los que depende la lectura del cuerpo.
  beforeEach(() => {
    sessionStorage.clear();
    writeToken(sessionToken);
    vi.useFakeTimers({
      toFake: [
        "setTimeout",
        "clearTimeout",
        "setInterval",
        "clearInterval",
        "Date",
      ],
    });
  });

  // Al desmontar el hook se aborta el canal; después se restauran fetch y el reloj.
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  // Regla del canal: tras un corte, el hook vuelve a abrirlo esperando 1, 2 y 5 segundos, y de ahí
  // en adelante 5, mientras la red sigue fallando; entretanto conserva la última instantánea y no
  // expone error (research.md, sección 18).
  it("reconecta tras un corte con espera de 1, 2 y 5 segundos, con tope de 5", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveEvents(streamThenCut(brunoView(3, true)), networkFailure),
    );
    vi.stubGlobal("fetch", fetchMock);

    const { result } = renderHook(() => useEstimationStream(sessionId));
    await settle();
    expect(openings(fetchMock)).toBe(1);
    expect(result.current.session?.version).toBe(3);

    // Primer intento: 1 segundo después del corte.
    await advance(999);
    expect(openings(fetchMock)).toBe(1);
    await advance(1);
    expect(openings(fetchMock)).toBe(2);

    // Segundo intento: 2 segundos después del primero fallido.
    await advance(1999);
    expect(openings(fetchMock)).toBe(2);
    await advance(1);
    expect(openings(fetchMock)).toBe(3);

    // Tercer intento: 5 segundos después del segundo fallido.
    await advance(4999);
    expect(openings(fetchMock)).toBe(3);
    await advance(1);
    expect(openings(fetchMock)).toBe(4);

    // Tope: los intentos siguientes siguen esperando 5 segundos.
    await advance(4999);
    expect(openings(fetchMock)).toBe(4);
    await advance(1);
    expect(openings(fetchMock)).toBe(5);

    expect(result.current.session?.version).toBe(3);
    expect(result.current.error).toBeNull();
  });

  // Caso normal: Bruno votó 13 y se le cortó la conexión; al reconectar en la misma ronda el canal
  // le entrega la sesión en la que vuelve a figurar conectado con su voto 13 (FR-015, FR-016).
  it("Reconexión con voto", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveEvents(
        streamThenCut(brunoView(3, true)),
        streamThenCut(brunoView(5, true)),
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const { result } = renderHook(() => useEstimationStream(sessionId));
    await settle();
    await advance(1000);

    expect(openings(fetchMock)).toBeGreaterThanOrEqual(2);
    expect(result.current.session?.version).toBe(5);
    expect(result.current.session?.myVote).toBe("13");
    expect(
      result.current.session?.participants.find(
        (participant) => participant.user.id === bruno.id,
      )?.connected,
    ).toBe(true);
    expect(result.current.error).toBeNull();
  });

  // Regla del canal: si al reconectar la sesión ya está cerrada (409), no existe o es ajena (404) o
  // la sesión de la pestaña es inválida (401), el hook deja de intentar y expone el rechazo
  // (research.md, sección 18).
  it.each([
    {
      status: 409,
      body: {
        error: closedMessage,
        status: "finished",
        agreedStoryPoints: 8,
        cancelReason: null,
      } satisfies SessionClosedResponse,
    },
    { status: 404, body: { error: notFoundMessage } },
    { status: 401, body: { error: invalidSessionMessage } },
  ])(
    "se detiene ante $status al reconectar",
    async ({ status, body }: { status: number; body: ResponseBody }) => {
      fetchMock = vi.fn<typeof fetch>(
        serveEvents(streamThenCut(brunoView(3, true)), rejection(status, body)),
      );
      vi.stubGlobal("fetch", fetchMock);

      const { result } = renderHook(() => useEstimationStream(sessionId));
      await settle();
      await advance(1000);
      expect(openings(fetchMock)).toBe(2);

      // Pasado más de un ciclo completo de esperas no hay nuevos intentos.
      await advance(30000);
      expect(openings(fetchMock)).toBe(2);
      expect(result.current.error?.status).toBe(status);
      expect(result.current.error?.message).toBe(body.error);
    },
  );
  // Regla del canal: una reconexión que entrega una instantánea reinicia la espera, así que ante el
  // corte siguiente el primer intento vuelve a ocurrir a 1 segundo y no a la espera acumulada
  // (research.md, sección 18).
  it("reinicia la espera a 1 segundo después de una reconexión exitosa", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveEvents(
        streamThenCut(brunoView(3, true)),
        networkFailure,
        networkFailure,
        streamThenCut(brunoView(5, true)),
        networkFailure,
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const { result } = renderHook(() => useEstimationStream(sessionId));
    await settle();

    // Dos intentos fallidos (1 y 2 segundos) y la reconexión exitosa a los 5 segundos, que vuelve a
    // cortarse después de entregar la instantánea.
    await advance(1000);
    await advance(2000);
    await advance(5000);
    expect(openings(fetchMock)).toBe(4);
    expect(result.current.session?.version).toBe(5);

    // Tras el corte posterior el intento ocurre a 1 segundo, no a 5.
    await advance(999);
    expect(openings(fetchMock)).toBe(4);
    await advance(1);
    expect(openings(fetchMock)).toBe(5);
  });

  // Regla del canal: un rechazo del servidor distinto de 409, 404 y 401 no detiene el canal; el hook
  // reintenta con la misma espera de 1, 2 y 5 segundos y conserva la última instantánea sin exponer
  // error (research.md, sección 18).
  it.each([
    { status: 500, body: { error: "Error interno del servidor." } },
    { status: 503, body: { error: "Servicio no disponible." } },
  ])(
    "reintenta con la misma espera ante $status al reconectar",
    async ({ status, body }: { status: number; body: ResponseBody }) => {
      fetchMock = vi.fn<typeof fetch>(
        serveEvents(streamThenCut(brunoView(3, true)), rejection(status, body)),
      );
      vi.stubGlobal("fetch", fetchMock);

      const { result } = renderHook(() => useEstimationStream(sessionId));
      await settle();

      // Esperas de 1, 2 y 5 segundos, y luego 5 de tope.
      for (const [wait, expected] of [
        [1000, 2],
        [2000, 3],
        [5000, 4],
        [5000, 5],
      ]) {
        await advance(wait - 1);
        expect(openings(fetchMock)).toBe(expected - 1);
        await advance(1);
        expect(openings(fetchMock)).toBe(expected);
      }

      expect(result.current.session?.version).toBe(3);
      expect(result.current.error).toBeNull();
    },
  );

  // Regla del canal: una instantánea de la sesión cerrada termina el canal; aunque el servidor lo
  // cierre después, el hook no vuelve a abrirlo y conserva esa instantánea (005/US5).
  it("no reconecta después de recibir una instantánea de sesión cerrada", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveEvents(
        streamThenCut(finishedView(7)),
        streamThenCut(brunoView(8, true)),
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const { result } = renderHook(() => useEstimationStream(sessionId));
    await settle();
    await advance(30000);

    expect(openings(fetchMock)).toBe(1);
    expect(result.current.session?.status).toBe("finished");
    expect(result.current.session?.version).toBe(7);
  });

  // Regla del canal: al desmontar el hook se abandona el canal; un corte previo no deja un intento
  // pendiente que vuelva a abrirlo.
  it("no reconecta después de desmontar el hook", async () => {
    fetchMock = vi.fn<typeof fetch>(
      serveEvents(
        streamThenCut(brunoView(3, true)),
        streamThenCut(brunoView(5, true)),
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    const { unmount } = renderHook(() => useEstimationStream(sessionId));
    await settle();
    unmount();
    await advance(30000);

    expect(openings(fetchMock)).toBe(1);
  });

  // Regla del canal (T077): la primera apertura recibe el mismo trato que una reconexión. Un fallo de
  // red o un rechazo distinto de 409, 404 y 401 no expone error: la sala sigue en "Conectando con la
  // sesión…" (sin instantánea) y el hook reintenta con la espera de 1, 2 y 5 segundos, con tope de
  // 5, hasta que una apertura entrega la instantánea (research.md, sección 18).
  it.each([
    { name: "un fallo de red", reply: networkFailure },
    {
      name: "un 500",
      reply: rejection(500, { error: "Error interno del servidor." }),
    },
    {
      name: "un 503",
      reply: rejection(503, { error: "Servicio no disponible." }),
    },
  ])(
    "reintenta la primera apertura ante $name sin exponer error",
    async ({ reply }: { reply: StreamReply }) => {
      fetchMock = vi.fn<typeof fetch>(
        serveEvents(
          reply,
          reply,
          reply,
          reply,
          streamThenCut(brunoView(3, true)),
        ),
      );
      vi.stubGlobal("fetch", fetchMock);

      const { result } = renderHook(() => useEstimationStream(sessionId));
      await settle();
      expect(openings(fetchMock)).toBe(1);
      expect(result.current.error).toBeNull();
      expect(result.current.session).toBeNull();

      // Esperas de 1, 2 y 5 segundos, y luego 5 de tope, sin instantánea ni error entretanto.
      for (const [wait, expected] of [
        [1000, 2],
        [2000, 3],
        [5000, 4],
      ]) {
        await advance(wait - 1);
        expect(openings(fetchMock)).toBe(expected - 1);
        await advance(1);
        expect(openings(fetchMock)).toBe(expected);
        expect(result.current.error).toBeNull();
        expect(result.current.session).toBeNull();
      }

      // La quinta apertura, 5 segundos después, entrega la instantánea.
      await advance(4999);
      expect(openings(fetchMock)).toBe(4);
      await advance(1);
      expect(openings(fetchMock)).toBe(5);
      expect(result.current.session?.version).toBe(3);
      expect(result.current.error).toBeNull();
    },
  );

  // Regla del canal (T077): en la primera apertura, un rechazo 409 (sesión cerrada), 404 (sesión
  // inexistente o ajena) o 401 (sesión inválida) detiene el canal y expone el error recibido, sin
  // reintentar (research.md, sección 18).
  it.each([
    {
      status: 409,
      body: {
        error: closedMessage,
        status: "finished",
        agreedStoryPoints: 8,
        cancelReason: null,
      } satisfies SessionClosedResponse,
    },
    { status: 404, body: { error: notFoundMessage } },
    { status: 401, body: { error: invalidSessionMessage } },
  ])(
    "se detiene ante $status en la primera apertura",
    async ({ status, body }: { status: number; body: ResponseBody }) => {
      fetchMock = vi.fn<typeof fetch>(
        serveEvents(rejection(status, body), streamThenCut(brunoView(3, true))),
      );
      vi.stubGlobal("fetch", fetchMock);

      const { result } = renderHook(() => useEstimationStream(sessionId));
      await settle();
      await advance(30000);

      expect(openings(fetchMock)).toBe(1);
      expect(result.current.session).toBeNull();
      expect(result.current.error?.status).toBe(status);
      expect(result.current.error?.message).toBe(body.error);
    },
  );
});
