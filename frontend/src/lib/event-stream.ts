// Evento recibido por un canal text/event-stream: su nombre (campo event:, "message" si no viene)
// y sus datos (los campos data: del evento unidos por saltos de línea).
export type ServerSentEvent = {
  event: string;
  data: string;
};

// Lee el cuerpo de una respuesta text/event-stream hasta que termina y llama a onEvent una vez por
// cada evento completo, en el orden recibido. Firma mínima de T017: la implementación es de T029.
export async function readEventStream(
  body: ReadableStream<Uint8Array>,
  onEvent: (event: ServerSentEvent) => void,
): Promise<void> {
  void body;
  void onEvent;
  throw new Error("not implemented");
}
