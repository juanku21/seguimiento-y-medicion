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

// Esperas antes de cada reintento de abrir el canal: 1, 2 y 5 segundos, y 5 de ahí en adelante
// (research.md, sección 18).
const RECONNECT_DELAYS_MS = [1000, 2000, 5000];

// Rechazos que detienen el canal en lugar de reintentar: sesión inválida (401), sesión inexistente
// o ajena (404) y sesión cerrada (409).
const STOP_STATUSES = new Set([401, 404, 409]);

// Abre el canal de eventos de la sesión con la sesión de la pestaña y devuelve la última
// instantánea. Descarta las instantáneas de version menor a la vigente, que llegaron tarde; la de
// igual version se acepta porque el backend la repite ante cambios de la historia (FR-074). Si el
// canal se corta, falla la red o llega otro rechazo, lo vuelve a abrir con espera creciente de 1, 2
// y 5 segundos (tope 5), conservando la última instantánea y sin exponer error; cada instantánea
// recibida reinicia la espera. Ante un rechazo 409, 404 o 401 (que además descarta la sesión en el
// cliente HTTP) el canal se detiene y expone el error. Con una instantánea de la sesión cerrada
// (Finalizada o Cancelada) el canal se detiene y queda esa instantánea (005/US5). Al desmontar o
// cambiar de sesión se aborta la petición, se cancela el reintento pendiente y se ignora todo lo que
// llegue después.
export function useEstimationStream(sessionId: string): EstimationStreamState {
  const [state, setState] = useState<EstimationStreamState>({
    session: null,
    error: null,
  });

  useEffect(() => {
    const controller = new AbortController();
    let currentVersion: number | null = null;
    let attempt = 0;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;

    // Entrega cada instantánea vigente al estado, reinicia la espera de reconexión y deja pasar los
    // demás eventos sin efecto.
    const handleEvent = (raw: ServerSentEvent) => {
      const event = parseEstimationEvent(raw);
      if (event === null || event.event !== "state") {
        return;
      }
      attempt = 0;
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

    // Programa el próximo intento de abrir el canal con la espera que corresponde al número de
    // intentos fallidos desde la última instantánea.
    const scheduleReconnect = () => {
      const delay =
        RECONNECT_DELAYS_MS[Math.min(attempt, RECONNECT_DELAYS_MS.length - 1)];
      attempt += 1;
      retryTimer = setTimeout(() => void run(), delay);
    };

    // Abre el canal y lo lee hasta que el servidor lo cierra. Un rechazo que detiene el canal se
    // expone; un corte, un fallo de red o cualquier otro rechazo programan un reintento, salvo que el
    // canal se haya abortado por desmontar o por recibir la sesión cerrada.
    const run = async () => {
      const result = await streamEstimationSession(
        sessionId,
        controller.signal,
      );
      if (controller.signal.aborted) {
        return;
      }
      if (!result.ok) {
        if (STOP_STATUSES.has(result.error.status)) {
          const { error } = result;
          setState((previous) => ({ ...previous, error }));
          return;
        }
        scheduleReconnect();
        return;
      }
      try {
        await readEventStream(result.data, handleEvent);
      } catch {
        // El corte o el aborto ya terminaron la lectura: se decide abajo si se reintenta.
      }
      if (!controller.signal.aborted) {
        scheduleReconnect();
      }
    };

    void run();
    return () => {
      controller.abort();
      clearTimeout(retryTimer);
    };
  }, [sessionId]);

  return state;
}
