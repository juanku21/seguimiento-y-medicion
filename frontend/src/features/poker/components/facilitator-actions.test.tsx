import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { FacilitatorActions } from "@/features/poker/components/facilitator-actions";
import type { RoundStatus } from "@/types/poker";

// Pruebas de las acciones del facilitador de la sala de Planning Poker (T049). Quién es el
// facilitador y si la ronda admite revelarse lo resuelve el backend: la sala pasa si quien mira
// tiene el rol según la instantánea y el estado de la ronda vigente. Acá se verifica qué acciones
// ofrece la vista y qué acción dispara cada pulsación; el envío de POST /reveal es de la sala (T055).
//
// Contrato que fijan estas pruebas para FacilitatorActions:
// - Recibe isFacilitator (boolean), roundStatus (RoundStatus) y onReveal().
// - Al facilitador, con la ronda vigente en votación, le ofrece el botón con nombre accesible
//   exacto "Revelar"; pulsarlo llama a onReveal una vez.
// - A quien no es el facilitador no le ofrece el botón "Revelar".
// - Con la ronda vigente ya revelada no ofrece el botón "Revelar" (FR-026).

// Renderiza las acciones y devuelve la acción de revelar observada.
function renderActions(isFacilitator: boolean, roundStatus: RoundStatus) {
  const onReveal = vi.fn<() => void>();
  render(
    <FacilitatorActions
      isFacilitator={isFacilitator}
      roundStatus={roundStatus}
      onReveal={onReveal}
    />,
  );
  return { onReveal };
}

// Devuelve el botón "Revelar", o null si la vista no lo ofrece.
function revealButton(): HTMLElement | null {
  return screen.queryByRole("button", { name: "Revelar" });
}

describe("005/US3 - Revelar los votos y ver las diferencias", () => {
  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
  });

  // Caso normal: con la ronda en votación, el facilitador ve el botón Revelar y al pulsarlo pide la
  // revelación manual (FR-032).
  it("Revelación manual con consenso", () => {
    const { onReveal } = renderActions(true, "voting");

    const button = revealButton();
    expect(button).not.toBeNull();
    fireEvent.click(button as HTMLElement);
    expect(onReveal).toHaveBeenCalledTimes(1);
  });

  // Caso de error: quien no es el facilitador no tiene el botón Revelar; la acción está reservada
  // al facilitador (FR-034, FR-064).
  it("Revelar sin ser facilitador", () => {
    const { onReveal } = renderActions(false, "voting");

    expect(revealButton()).toBeNull();
    expect(onReveal).not.toHaveBeenCalled();
  });

  // Regla de la interfaz: con la ronda vigente ya revelada no se ofrece revelarla otra vez, ni
  // siquiera al facilitador (FR-026).
  it("no ofrece Revelar con la ronda ya revelada", () => {
    renderActions(true, "revealed");

    expect(revealButton()).toBeNull();
  });
});
