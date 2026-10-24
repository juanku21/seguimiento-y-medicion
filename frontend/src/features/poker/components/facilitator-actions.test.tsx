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

// Pruebas del botón "Cancelar sesión" de las acciones del facilitador (T080). Quién puede cancelar
// y qué pasa con la historia, las rondas y los votos lo resuelve el backend (FR-058 a FR-060): acá
// se verifica que la vista ofrece la cancelación solo al facilitador y que la pide recién después
// de confirmarla; el envío de POST /cancel es de la sala (T084).
//
// Contrato que fijan estas pruebas para FacilitatorActions:
// - Recibe además onCancel(). Al facilitador le ofrece, con la ronda en votación o ya revelada, el
//   botón con nombre accesible exacto "Cancelar sesión".
// - Pulsar "Cancelar sesión" no llama a onCancel: muestra la pregunta
//   "¿Cancelar la sesión sin registrar estimación?" con los botones "Confirmar cancelación" y
//   "Volver". "Confirmar cancelación" llama a onCancel una vez; "Volver" cierra la pregunta sin
//   llamarlo.
// - A quien no es el facilitador no le ofrece "Cancelar sesión".

// Pregunta de confirmación de la cancelación.
const CANCEL_QUESTION = "¿Cancelar la sesión sin registrar estimación?";

// Renderiza las acciones con la cancelación y devuelve la acción de cancelar observada.
function renderWithCancel(isFacilitator: boolean, roundStatus: RoundStatus) {
  const onCancel = vi.fn<() => void>();
  render(
    <FacilitatorActions
      isFacilitator={isFacilitator}
      roundStatus={roundStatus}
      onReveal={vi.fn<() => void>()}
      onNewRound={vi.fn<() => void>()}
      onCancel={onCancel}
    />,
  );
  return { onCancel };
}

// Devuelve el botón "Cancelar sesión", o null si la vista no lo ofrece.
function cancelButton(): HTMLElement | null {
  return screen.queryByRole("button", { name: "Cancelar sesión" });
}

describe("005/US7 - Cancelar una sesión sin registrar estimación", () => {
  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
  });

  // Caso normal: el facilitador pulsa Cancelar sesión, confirma y pide la cancelación una sola vez
  // (FR-058). Vale con la ronda en votación y con la ronda ya revelada.
  it("Cancelación conserva estimación", () => {
    for (const roundStatus of ["voting", "revealed"] as const) {
      const { onCancel } = renderWithCancel(true, roundStatus);

      const button = cancelButton();
      expect(button).not.toBeNull();
      fireEvent.click(button as HTMLElement);
      fireEvent.click(
        screen.getByRole("button", { name: "Confirmar cancelación" }),
      );
      expect(onCancel).toHaveBeenCalledTimes(1);

      cleanup();
    }
  });

  // La cancelación no se puede deshacer: el botón pide confirmación antes de cancelar, y "Volver"
  // cierra la pregunta sin cancelar.
  it("pide confirmación antes de cancelar", () => {
    const { onCancel } = renderWithCancel(true, "voting");

    const button = cancelButton();
    expect(button).not.toBeNull();
    fireEvent.click(button as HTMLElement);

    expect(screen.getByText(CANCEL_QUESTION)).not.toBeNull();
    expect(
      screen.getByRole("button", { name: "Confirmar cancelación" }),
    ).not.toBeNull();
    expect(onCancel).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Volver" }));

    expect(screen.queryByText(CANCEL_QUESTION)).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Confirmar cancelación" }),
    ).toBeNull();
    expect(onCancel).not.toHaveBeenCalled();
  });

  // Caso de error: quien no es el facilitador no tiene el botón Cancelar sesión; la acción está
  // reservada al facilitador (FR-059, FR-064).
  it("Cancelar sin ser facilitador", () => {
    const { onCancel } = renderWithCancel(false, "voting");

    expect(cancelButton()).toBeNull();
    expect(onCancel).not.toHaveBeenCalled();
  });
});
