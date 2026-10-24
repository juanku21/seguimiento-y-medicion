"use client";

import { useEffect, useState } from "react";
import {
  isEstimationSession,
  streamEstimationSession,
} from "@/features/poker/api";
import type { ApiError } from "@/lib/api-client";
import { readEventStream, type ServerSentEvent } from "@/lib/event-stream";
import type { EstimationEvent, EstimationSession } from "@/types/poker";

// Estado del canal de una sesión: la última instantánea recibida (null hasta la primera) y el
// rechazo con el que se detuvo el canal, si lo hubo.
export type EstimationStreamState = {
  session: EstimationSession | null;
  error: ApiError | null;
};

// Interpreta un evento del canal: state con una sesión válida en data o removed. Un evento de otro
// nombre o con datos que no son una sesión se ignora devolviendo null, sin fallar.
function parseEstimationEvent(event: ServerSentEvent): EstimationEvent | null {
  if (event.event === "removed") {
    return { event: "removed" };
  }
  if (event.event !== "state") {
    return null;
  }
  try {
    const data: unknown = JSON.parse(event.data);
    return isEstimationSession(data) ? { event: "state", session: data } : null;
  } catch {
    return null;
  }
}

// Abre el canal de eventos de la sesión con la sesión de la pestaña y devuelve la última
// instantánea. Descarta las instantáneas de version menor a la vigente, que llegaron tarde; la de
// igual version se acepta porque el backend la repite ante cambios de la historia (FR-074). Ante un
// rechazo (404 de sesión inexistente o ajena, 401 de sesión inválida, que además descarta la sesión
// en el cliente HTTP) el canal se detiene y expone el error. Con una instantánea de la sesión
// cerrada (Finalizada o Cancelada) el canal se detiene y queda esa instantánea (005/US5). Al
// desmontar o cambiar de sesión se aborta la petición y se ignora todo lo que llegue después.
export function useEstimationStream(sessionId: string): EstimationStreamState {
  const [state, setState] = useState<EstimationStreamState>({
    session: null,
    error: null,
  });

  useEffect(() => {
    const controller = new AbortController();
    let currentVersion: number | null = null;

    // Entrega cada instantánea vigente al estado y deja pasar los demás eventos sin efecto.
    const handleEvent = (raw: ServerSentEvent) => {
      const event = parseEstimationEvent(raw);
      if (event === null || event.event !== "state") {
        return;
      }
      if (currentVersion !== null && event.session.version < currentVersion) {
        return;
      }
      currentVersion = event.session.version;
      setState({ session: event.session, error: null });
      // Una sesión cerrada ya no cambia: se aborta la petición para no seguir leyendo el canal.
      if (event.session.status !== "open") {
        controller.abort();
      }
    };

    // Abre el canal y lo lee hasta que el servidor lo cierra; un corte o el aborto al desmontar
    // terminan la lectura sin error visible.
    const run = async () => {
      const result = await streamEstimationSession(
        sessionId,
        controller.signal,
      );
      if (controller.signal.aborted) {
        return;
      }
      if (!result.ok) {
        setState((previous) => ({ ...previous, error: result.error }));
        return;
      }
      try {
        await readEventStream(result.data, handleEvent);
      } catch {
        // El corte o el aborto ya terminaron la lectura: no queda nada que hacer.
      }
    };

    void run();
    return () => {
      controller.abort();
    };
  }, [sessionId]);

  return state;
}
