import { describe, expect, it } from "vitest";
import { readEventStream, type ServerSentEvent } from "@/lib/event-stream";

// Pruebas unitarias del lector de text/event-stream (T017), que la sala de Planning Poker usa para
// leer el canal GET /api/v1/estimation-sessions/{sessionId}/events sobre fetch (research.md,
// secciones 2 y 18). El cuerpo se arma con fragmentos de bytes elegidos por la prueba, para simular
// cómo los entrega la red. Es una utilidad sin escenario propio de spec.md: el describe no lleva el
// prefijo 005/USn para no alterar el conteo de escenarios, igual que las pruebas de format-date.
//
// Contrato que fijan estas pruebas para readEventStream(body, onEvent):
// - Lee el cuerpo hasta que termina y resuelve la promesa recién entonces.
// - Llama a onEvent una vez por evento completo (terminado en una línea en blanco), en orden, con
//   el nombre del campo event: ("message" si no viene) y los campos data: unidos por "\n".
// - Ignora los comentarios (líneas que empiezan con ":", como ": ping").
// - Acepta líneas terminadas en "\n" o en "\r\n" y eventos o caracteres partidos entre fragmentos.

const encoder = new TextEncoder();

// Arma un cuerpo de respuesta que entrega, en orden, cada fragmento de bytes indicado.
function streamOf(chunks: Uint8Array[]): ReadableStream<Uint8Array> {
  return new ReadableStream<Uint8Array>({
    start(controller) {
      chunks.forEach((chunk) => controller.enqueue(chunk));
      controller.close();
    },
  });
}

// Arma un cuerpo que entrega cada texto como un fragmento separado.
function streamOfText(...parts: string[]): ReadableStream<Uint8Array> {
  return streamOf(parts.map((part) => encoder.encode(part)));
}

// Lee el cuerpo completo y devuelve los eventos recibidos, en el orden en que llegaron.
async function collect(
  body: ReadableStream<Uint8Array>,
): Promise<ServerSentEvent[]> {
  const events: ServerSentEvent[] = [];
  await readEventStream(body, (event) => events.push(event));
  return events;
}

// Instantánea mínima como la que viaja en el data: de un evento state.
const snapshot = JSON.stringify({
  id: "6f1c2a52-7d0e-4c8b-9a35-0b0c1f3e9d11",
  version: 4,
  status: "open",
});

describe("Lector de text/event-stream", () => {
  // Caso normal: un evento completo llega en un único fragmento.
  it("entrega un evento que llega en un fragmento", async () => {
    const events = await collect(
      streamOfText(`event: state\ndata: ${snapshot}\n\n`),
    );

    expect(events).toEqual([{ event: "state", data: snapshot }]);
  });

  // Caso alternativo: el evento llega partido en dos fragmentos, cortado en medio del JSON; no se
  // entrega nada hasta completar la línea en blanco.
  it("entrega un evento partido en dos fragmentos", async () => {
    const text = `event: state\ndata: ${snapshot}\n\n`;
    const cut = text.indexOf("version");

    const events = await collect(
      streamOfText(text.slice(0, cut), text.slice(cut)),
    );

    expect(events).toEqual([{ event: "state", data: snapshot }]);
  });

  // Caso límite: el corte entre fragmentos cae en medio de los bytes de un carácter acentuado; el
  // texto se decodifica entero, sin caracteres de reemplazo.
  it("decodifica un carácter partido entre dos fragmentos", async () => {
    const bytes = encoder.encode("event: state\ndata: Sesión abierta\n\n");
    const accent = encoder.encode("ó");
    const start = bytes.findIndex(
      (byte, index) => byte === accent[0] && bytes[index + 1] === accent[1],
    );

    const events = await collect(
      streamOf([bytes.slice(0, start + 1), bytes.slice(start + 1)]),
    );

    expect(events).toEqual([{ event: "state", data: "Sesión abierta" }]);
  });

  // Caso alternativo: varios campos data: de un mismo evento se unen con saltos de línea.
  it("une varios data: de un evento con saltos de línea", async () => {
    const events = await collect(
      streamOfText(
        "event: state\ndata: primera\ndata: segunda\ndata: tercera\n\n",
      ),
    );

    expect(events).toEqual([
      { event: "state", data: "primera\nsegunda\ntercera" },
    ]);
  });

  // Caso alternativo: los comentarios ": ping" que mantienen viva la conexión no producen eventos
  // ni se mezclan con los datos del evento siguiente.
  it("ignora los comentarios : ping", async () => {
    const events = await collect(
      streamOfText(
        ": ping\n\n",
        `event: state\ndata: ${snapshot}\n\n`,
        ": ping\n\n",
        'event: state\n: ping\ndata: {"version":5}\n\n',
      ),
    );

    expect(events).toEqual([
      { event: "state", data: snapshot },
      { event: "state", data: '{"version":5}' },
    ]);
  });

  // Caso alternativo: con líneas terminadas en \r\n el evento se reconoce igual y los datos no
  // arrastran el \r.
  it("acepta líneas terminadas en \\r\\n", async () => {
    const events = await collect(
      streamOfText(`event: state\r\ndata: ${snapshot}\r\n\r\n`),
    );

    expect(events).toEqual([{ event: "state", data: snapshot }]);
  });

  // Caso alternativo: varios eventos en un mismo fragmento se entregan por separado y en orden; uno
  // sin campo event: se nombra "message".
  it("entrega en orden varios eventos de un mismo fragmento", async () => {
    const events = await collect(
      streamOfText(
        'event: state\ndata: {"version":1}\n\nevent: removed\ndata: {}\n\ndata: sin nombre\n\n',
      ),
    );

    expect(events).toEqual([
      { event: "state", data: '{"version":1}' },
      { event: "removed", data: "{}" },
      { event: "message", data: "sin nombre" },
    ]);
  });
});
