// Evento recibido por un canal text/event-stream: su nombre (campo event:, "message" si no viene)
// y sus datos (los campos data: del evento unidos por saltos de línea).
export type ServerSentEvent = {
  event: string;
  data: string;
};

// Lee el cuerpo de una respuesta text/event-stream hasta que termina y llama a onEvent una vez por
// cada evento completo, en el orden recibido. El decodificador trabaja en modo stream para que un
// carácter partido entre fragmentos se arme entero, y el texto pendiente se acumula hasta tener una
// línea completa. Un evento a medias al cerrarse el cuerpo se descarta, como indica el estándar.
export async function readEventStream(
  body: ReadableStream<Uint8Array>,
  onEvent: (event: ServerSentEvent) => void,
): Promise<void> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  const parser = createLineParser(onEvent);
  let pending = "";

  for (;;) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }
    pending += decoder.decode(value, { stream: true });
    pending = consumeLines(pending, parser);
  }
  consumeLines(pending + decoder.decode(), parser);
}

// Entrega al parser cada línea completa del texto (sin el "\r" final de un "\r\n") y devuelve el
// resto, que todavía no terminó de llegar.
function consumeLines(text: string, parser: (line: string) => void): string {
  let rest = text;
  let index = rest.indexOf("\n");
  while (index !== -1) {
    const line = rest.slice(0, index);
    parser(line.endsWith("\r") ? line.slice(0, -1) : line);
    rest = rest.slice(index + 1);
    index = rest.indexOf("\n");
  }
  return rest;
}

// Arma el procesador de líneas: acumula los campos event: y data: del evento en curso, ignora los
// comentarios (líneas que empiezan con ":") y, ante una línea en blanco, entrega el evento si tiene
// datos y reinicia el estado para el siguiente.
function createLineParser(
  onEvent: (event: ServerSentEvent) => void,
): (line: string) => void {
  let eventName = "";
  let dataLines: string[] = [];

  return (line: string) => {
    if (line === "") {
      if (dataLines.length > 0) {
        onEvent({ event: eventName || "message", data: dataLines.join("\n") });
      }
      eventName = "";
      dataLines = [];
      return;
    }
    if (line.startsWith(":")) {
      return;
    }

    const colon = line.indexOf(":");
    const field = colon === -1 ? line : line.slice(0, colon);
    const raw = colon === -1 ? "" : line.slice(colon + 1);
    const value = raw.startsWith(" ") ? raw.slice(1) : raw;

    if (field === "event") {
      eventName = value;
    } else if (field === "data") {
      dataLines.push(value);
    }
  };
}
