import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { SessionClosedNotice } from "@/features/poker/components/session-closed-notice";
import type { CancelReason, ScaleStoryPoints } from "@/types/poker";

// Pruebas del aviso de sesión cerrada en la sala de Planning Poker (T064). Cerrar la sesión y
// escribir los Story Points en la historia lo resuelve el backend (FR-052): la sala recibe el
// último state con la sesión cerrada (o el 409 de sesión cerrada) y muestra el aviso con su
// resultado. Acá se verifica qué muestra el aviso para una sesión Finalizada; los motivos de
// cancelación los cubre T080 (005/US7).
//
// Contrato que fijan estas pruebas para SessionClosedNotice:
// - Recibe status ("finished" o "canceled"), agreedStoryPoints (el valor acordado, null si no lo
//   hay) y cancelReason (null si no la canceló el sistema), como llegan en EstimationSession y en
//   SessionClosedResponse.
// - Muestra una región con nombre accesible "Sesión cerrada" con el estado en español
//   ("Finalizada") y el valor acordado como "<N> Story Points".

// Monta el aviso de una sesión Finalizada con el valor acordado indicado.
function renderFinished(agreedStoryPoints: ScaleStoryPoints) {
  render(
    <SessionClosedNotice
      status="finished"
      agreedStoryPoints={agreedStoryPoints}
      cancelReason={null}
    />,
  );
}

// Devuelve el texto de la región del aviso de cierre.
function noticeText(): string {
  return (
    screen.getByRole("region", { name: "Sesión cerrada" }).textContent ?? ""
  );
}

describe("005/US5 - Registrar la estimación acordada", () => {
  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
  });

  // Caso normal: registrado 8, cada participante ve la sesión Finalizada con 8 Story Points
  // (FR-052).
  it("Registro con consenso", () => {
    renderFinished(8);

    const text = noticeText();
    expect(text).toContain("Finalizada");
    expect(text).toContain("8 Story Points");
    expect(text).not.toContain("Cancelada");
  });

  // Caso alternativo: la sesión se cierra con 20 sobre una historia que estaba en 3; el aviso
  // muestra el valor acordado de la sesión, que reemplaza al anterior.
  it("Reemplaza estimación anterior", () => {
    renderFinished(20);

    const text = noticeText();
    expect(text).toContain("Finalizada");
    expect(text).toContain("20 Story Points");
  });

  // Regla de la interfaz: el 0 acordado se muestra como "0 Story Points", distinto de
  // "sin estimar" (RC-05).
  it("muestra 0 Story Points como valor acordado", () => {
    renderFinished(0);

    const text = noticeText();
    expect(text).toContain("Finalizada");
    expect(text).toContain("0 Story Points");
    expect(text).not.toMatch(/sin estimar/i);
  });
});

// Pruebas del motivo de cancelación en el aviso de sesión cerrada (T080). Cancelar la sesión y
// registrar el motivo lo resuelve el backend (FR-058, FR-061, FR-063): la sala recibe el último
// state con la sesión Cancelada (o el 409 de sesión cerrada) y muestra el aviso. Acá se verifica que
// el aviso muestra el estado y, si la canceló el sistema, el motivo en español.
//
// Contrato que fijan estas pruebas para SessionClosedNotice:
// - Con status "canceled" muestra "Cancelada" y ningún valor en Story Points.
// - Con cancelReason muestra su motivo en español: "Por inactividad" (inactivity), "Historia
//   completada" (story_completed) y "Proyecto finalizado" (project_finished).
// - Con cancelReason null (la canceló el facilitador) no muestra ninguno de esos motivos.

// Motivos de cancelación del sistema con su texto esperado en español.
const CANCEL_REASON_TEXTS: Record<CancelReason, string> = {
  inactivity: "Por inactividad",
  story_completed: "Historia completada",
  project_finished: "Proyecto finalizado",
};

// Monta el aviso de una sesión Cancelada con el motivo indicado.
function renderCanceled(cancelReason: CancelReason | null) {
  render(
    <SessionClosedNotice
      status="canceled"
      agreedStoryPoints={null}
      cancelReason={cancelReason}
    />,
  );
}

describe("005/US7 - Cancelar una sesión sin registrar estimación", () => {
  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
  });

  // Caso normal: cancelada por el facilitador, cada participante ve la sesión Cancelada, sin valor
  // acordado y sin motivo del sistema (FR-058, FR-060).
  it("Cancelación conserva estimación", () => {
    renderCanceled(null);

    const text = noticeText();
    expect(text).toContain("Cancelada");
    expect(text).not.toContain("Story Points");
    for (const reasonText of Object.values(CANCEL_REASON_TEXTS)) {
      expect(text).not.toContain(reasonText);
    }
  });

  // Caso límite: tras 60 minutos sin hechos, el aviso muestra la sesión Cancelada por inactividad
  // (FR-063, RN-31).
  it("Cancelación por inactividad", () => {
    renderCanceled("inactivity");

    const text = noticeText();
    expect(text).toContain("Cancelada");
    expect(text).toContain(CANCEL_REASON_TEXTS.inactivity);
  });

  // Regla de la interfaz: cada motivo de cancelación del sistema se muestra en español
  // (FR-061, FR-063, Principio VIII).
  it("muestra el motivo en español para cada cancelReason", () => {
    for (const [reason, reasonText] of Object.entries(CANCEL_REASON_TEXTS)) {
      renderCanceled(reason as CancelReason);

      const text = noticeText();
      expect(text).toContain("Cancelada");
      expect(text).toContain(reasonText);

      cleanup();
    }
  });
});
