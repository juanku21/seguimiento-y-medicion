import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { SessionClosedNotice } from "@/features/poker/components/session-closed-notice";
import type { ScaleStoryPoints } from "@/types/poker";

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
