import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { CardDeck } from "@/features/poker/components/card-deck";
import { ParticipantList } from "@/features/poker/components/participant-list";
import type {
  Card,
  EstimationParticipant,
  RoundStatus,
  UserRef,
} from "@/types/poker";

// Pruebas del mazo de cartas y de la marca "Ya votó" de la sala de Planning Poker (T037). El mazo
// y la lista reciben lo que llega en cada instantánea del canal: el secreto del voto lo resuelve el
// backend con ViewFor (votes en null, hasVoted por participante y myVote solo para el autor). Acá se
// verifica qué muestra la vista con cada instantánea y qué carta juega cada pulsación; el envío del
// voto (PUT /vote) es de la sala (T044).
//
// Contrato que fijan estas pruebas para CardDeck:
// - Recibe cards (Card[]), myVote (Card | null), roundStatus (RoundStatus) y onVote(card).
// - Muestra un grupo con nombre accesible "Mazo de cartas" con un botón por carta, en el orden
//   recibido, cuyo nombre accesible es el texto exacto de la carta ("0" … "100" y "?").
// - Resalta la carta propia con aria-pressed="true"; el resto de las cartas lleva
//   aria-pressed="false". Sin voto propio ninguna carta está resaltada.
// - Pulsar una carta llama a onVote con esa carta.
// - Con la ronda revelada todas las cartas están deshabilitadas y pulsarlas no juega nada.
//
// Contrato que fijan estas pruebas para ParticipantList:
// - El elemento de quien ya votó (hasVoted) muestra el texto exacto "Ya votó", sin ningún valor
//   de carta; el de quien no votó no lo muestra.

// Las once cartas admitidas, en el orden de la escala (FR-020).
const ALL_CARDS: Card[] = [
  "0",
  "1",
  "2",
  "3",
  "5",
  "8",
  "13",
  "20",
  "40",
  "100",
  "?",
];

// Integrantes del proyecto que participan de la sesión.
const ana: UserRef = {
  id: "3f1a9c64-7b2e-4d58-9a10-5c8e2f6b1d04",
  fullName: "Ana Pérez",
};
const bruno: UserRef = {
  id: "7d5b2e18-0c93-4f42-b6a1-8e3f5c9d2a06",
  fullName: "Bruno Gómez",
};
const carla: UserRef = {
  id: "5a9e3c71-4f28-4b06-8d1e-6c2b7f0a9e13",
  fullName: "Carla Ruiz",
};

// Arma un participante conectado de la ronda vigente, con o sin voto emitido.
function participant(
  user: UserRef,
  hasVoted: boolean,
  isFacilitator = false,
): EstimationParticipant {
  return { user, connected: true, isFacilitator, hasVoted };
}

// Renderiza el mazo con las once cartas y devuelve la acción de voto observada.
function renderDeck(myVote: Card | null, roundStatus: RoundStatus = "voting") {
  const onVote = vi.fn<(card: Card) => void>();
  const view = render(
    <CardDeck
      cards={ALL_CARDS}
      myVote={myVote}
      roundStatus={roundStatus}
      onVote={onVote}
    />,
  );
  return { onVote, ...view };
}

// Devuelve el botón de una carta dentro del mazo.
function cardButton(card: Card): HTMLElement {
  const deck = screen.getByRole("group", { name: "Mazo de cartas" });
  return within(deck).getByRole("button", { name: card });
}

// Verifica que la única carta resaltada sea la indicada (o ninguna, con null).
function expectHighlighted(card: Card | null) {
  for (const each of ALL_CARDS) {
    expect(cardButton(each).getAttribute("aria-pressed"), each).toBe(
      each === card ? "true" : "false",
    );
  }
}

// Devuelve el elemento de la lista de participantes que corresponde a una persona.
function participantItem(user: UserRef): HTMLElement {
  const list = screen.getByRole("list", { name: "Participantes" });
  const item = within(list)
    .getAllByRole("listitem")
    .find((each) => each.textContent?.includes(user.fullName));
  if (item === undefined) {
    throw new Error(`${user.fullName} no figura en la lista`);
  }
  return item;
}

// Verifica la marca "Ya votó" de un participante y que su elemento no muestre ningún valor de carta.
function expectVoteMark(user: UserRef, hasVoted: boolean) {
  const item = participantItem(user);
  const mark = within(item).queryByText("Ya votó");
  if (hasVoted) {
    expect(mark, user.fullName).not.toBeNull();
  } else {
    expect(mark, user.fullName).toBeNull();
  }
  const words = (item.textContent ?? "").split(/\s+/);
  for (const card of ALL_CARDS) {
    expect(words, `${user.fullName} muestra la carta ${card}`).not.toContain(
      card,
    );
  }
}

describe("005/US2 - Votar en secreto", () => {
  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
  });

  // Regla de la interfaz: el mazo ofrece las once cartas admitidas, en el orden de la escala y sin
  // ninguna resaltada mientras no hay voto propio (FR-020).
  it("muestra las once cartas de la escala", () => {
    renderDeck(null);

    const deck = screen.getByRole("group", { name: "Mazo de cartas" });
    const buttons = within(deck).getAllByRole("button");
    expect(buttons.map((button) => button.textContent?.trim())).toEqual(
      ALL_CARDS,
    );
    for (const button of buttons) {
      expect(button.hasAttribute("disabled"), button.textContent).toBe(false);
    }
    expectHighlighted(null);
  });

  // Caso normal: Ana juega la carta 5; Bruno, que todavía no votó, ve a Ana como "Ya votó" sin el
  // valor y su propio mazo sigue sin carta resaltada (FR-023).
  it("Voto oculto", () => {
    const { onVote } = renderDeck(null);
    fireEvent.click(cardButton("5"));
    expect(onVote).toHaveBeenCalledTimes(1);
    expect(onVote).toHaveBeenCalledWith("5");
    cleanup();

    renderDeck(null);
    render(
      <ParticipantList
        participants={[
          participant(ana, true, true),
          participant(bruno, false),
          participant(carla, false),
        ]}
      />,
    );

    expectHighlighted(null);
    expectVoteMark(ana, true);
    expectVoteMark(bruno, false);
    expectVoteMark(carla, false);
  });

  // Caso normal: quien jugó la carta 5 la ve resaltada y la distingue del resto (FR-024).
  it("Propia carta visible", () => {
    renderDeck("5");

    expectHighlighted("5");
  });

  // Caso alternativo: quien jugó el 8 juega el 13; el mazo resalta el 13 en lugar del 8 y para los
  // demás sigue figurando solamente como "Ya votó" (FR-022).
  it("Cambio de voto", () => {
    const { onVote, rerender } = renderDeck("8");
    expectHighlighted("8");

    fireEvent.click(cardButton("13"));
    expect(onVote).toHaveBeenCalledTimes(1);
    expect(onVote).toHaveBeenCalledWith("13");

    rerender(
      <CardDeck
        cards={ALL_CARDS}
        myVote="13"
        roundStatus="voting"
        onVote={onVote}
      />,
    );
    expectHighlighted("13");
    cleanup();

    render(
      <ParticipantList
        participants={[participant(ana, true, true), participant(bruno, true)]}
      />,
    );
    expectVoteMark(ana, true);
    expectVoteMark(bruno, true);
  });

  // Caso alternativo: quien no sabe cuánto cuesta juega "?", que se resalta como cualquier otra
  // carta y para los demás figura como "Ya votó" (FR-020).
  it("Carta desconocida", () => {
    const { onVote, rerender } = renderDeck(null);
    fireEvent.click(cardButton("?"));
    expect(onVote).toHaveBeenCalledTimes(1);
    expect(onVote).toHaveBeenCalledWith("?");

    rerender(
      <CardDeck
        cards={ALL_CARDS}
        myVote="?"
        roundStatus="voting"
        onVote={onVote}
      />,
    );
    expectHighlighted("?");
    cleanup();

    render(<ParticipantList participants={[participant(carla, true)]} />);
    expectVoteMark(carla, true);
  });

  // Caso límite: el 0 y el 100 son los extremos válidos de la escala y se pueden jugar.
  it("Extremos de la escala", () => {
    const { onVote } = renderDeck(null);

    fireEvent.click(cardButton("0"));
    fireEvent.click(cardButton("100"));

    expect(onVote.mock.calls).toEqual([["0"], ["100"]]);
  });

  // Caso de error: con la ronda revelada el mazo queda deshabilitado; ninguna carta se puede jugar
  // (FR-026).
  it("Ronda revelada", () => {
    const { onVote } = renderDeck("5", "revealed");

    for (const card of ALL_CARDS) {
      expect(cardButton(card).hasAttribute("disabled"), card).toBe(true);
      fireEvent.click(cardButton(card));
    }
    expect(onVote).not.toHaveBeenCalled();
  });
});
