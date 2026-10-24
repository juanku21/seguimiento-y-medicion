import { afterEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from "@testing-library/react";
import { FacilitatorActions } from "@/features/poker/components/facilitator-actions";
import { RoundHistory } from "@/features/poker/components/round-history";
import type {
  Card,
  EstimationRound,
  EstimationVote,
  RoundOutcome,
  RoundStatus,
  ScaleStoryPoints,
  UserRef,
} from "@/types/poker";

// Pruebas de la ronda nueva en la sala de Planning Poker (T057). Cerrar la ronda revelada, abrir la
// siguiente sin votos y conservar las anteriores con sus votos y su resultado lo resuelve el backend
// (FR-044, FR-045): la sala recibe en cada instantánea todas las rondas, la última es la vigente y
// solo las anteriores traen sus votos como historial. Acá se verifica qué muestra el historial con
// esas rondas y cuándo ofrece la vista la acción "Nueva ronda"; el envío de POST /rounds es de la
// sala (T060).
//
// Contrato que fijan estas pruebas para RoundHistory:
// - Recibe rounds (EstimationRound[]): todas las rondas de la instantánea en orden ascendente por
//   número; la última es la ronda vigente.
// - Muestra una región con nombre accesible "Historial de rondas" con una región por cada ronda
//   anterior a la vigente, en orden ascendente, con nombre accesible exacto "Ronda N".
// - Cada región de ronda muestra sus votos y su resultado como RoundResult: la lista "Votos" y la
//   región "Resultado de la ronda".
// - Nunca muestra la ronda vigente, esté en votación o ya revelada: su estado se ve en la sala.
// - Sin rondas anteriores no muestra la región del historial.
//
// Contrato que fijan estas pruebas para FacilitatorActions (se suma al de T049):
// - Recibe además onNewRound().
// - Al facilitador, con la ronda vigente revelada, le ofrece el botón con nombre accesible exacto
//   "Nueva ronda"; pulsarlo llama a onNewRound una vez.
// - Con la ronda vigente en votación no ofrece "Nueva ronda" (FR-046).
// - A quien no es el facilitador no le ofrece "Nueva ronda" (FR-047).

// Integrantes del proyecto que participan de la sesión, en orden alfabético por nombre.
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

// Arma el voto visible de un participante.
function vote(user: UserRef, card: Card): EstimationVote {
  return { user, card };
}

// Arma una ronda revelada con su número, sus votos y su resultado.
function revealedRound(
  number: number,
  votes: EstimationVote[],
  outcome: RoundOutcome,
): EstimationRound {
  return {
    number,
    status: "revealed",
    openedAt: "2026-10-23T14:00:00Z",
    revealedAt: "2026-10-23T14:03:00Z",
    votes,
    outcome,
    notVoted: [],
  };
}

// Arma la ronda vigente en votación: el backend la envía sin votos ni resultado (FR-023).
function votingRound(number: number): EstimationRound {
  return {
    number,
    status: "voting",
    openedAt: "2026-10-23T14:05:00Z",
    revealedAt: null,
    votes: null,
    outcome: null,
    notVoted: [],
  };
}

// Resultado con consenso en un valor, sin extremos.
function consensus(value: ScaleStoryPoints): RoundOutcome {
  return { kind: "consensus", value, min: null, max: null };
}

// Resultado con divergencia entre un mínimo y un máximo con sus autores.
function divergence(
  min: ScaleStoryPoints,
  minAuthors: UserRef[],
  max: ScaleStoryPoints,
  maxAuthors: UserRef[],
): RoundOutcome {
  return {
    kind: "divergence",
    value: null,
    min: { value: min, authors: minAuthors },
    max: { value: max, authors: maxAuthors },
  };
}

// Ronda 1 con los votos 3, 5 y 13 y su divergencia, como la del caso normal de la spec.
function divergentFirstRound(): EstimationRound {
  return revealedRound(
    1,
    [vote(ana, "3"), vote(bruno, "5"), vote(carla, "13")],
    divergence(3, [ana], 13, [carla]),
  );
}

// Devuelve la región del historial de rondas.
function historyRegion(): HTMLElement {
  return screen.getByRole("region", { name: "Historial de rondas" });
}

// Devuelve los nombres de las rondas mostradas en el historial, en el orden del documento. Busca
// cada nombre "Ronda N" por su nombre accesible, sin depender de cómo se lo asigne la vista.
function shownRoundNames(): string[] {
  const scope = within(historyRegion());
  const shown: { name: string; element: HTMLElement }[] = [];
  for (let number = 1; number <= 20; number++) {
    const name = `Ronda ${number}`;
    const element = scope.queryByRole("region", { name });
    if (element !== null) {
      shown.push({ name, element });
    }
  }
  expect(
    scope.getAllByRole("region", { name: /^Ronda \d+$/ }),
    "regiones de ronda",
  ).toHaveLength(shown.length);
  shown.sort((a, b) =>
    a.element.compareDocumentPosition(b.element) &
    Node.DOCUMENT_POSITION_FOLLOWING
      ? -1
      : 1,
  );
  return shown.map((each) => each.name);
}

// Devuelve la región de una ronda del historial.
function historyRound(number: number): HTMLElement {
  return within(historyRegion()).getByRole("region", {
    name: `Ronda ${number}`,
  });
}

// Verifica que una ronda del historial muestre cada voto con su autor y su carta, en el orden
// recibido, y la etiqueta de su resultado.
function expectRoundShown(
  number: number,
  votes: EstimationVote[],
  outcomeLabel: string,
) {
  const round = within(historyRound(number));
  const items = within(round.getByRole("list", { name: "Votos" })).getAllByRole(
    "listitem",
  );
  expect(items, `Ronda ${number}`).toHaveLength(votes.length);
  votes.forEach((each, index) => {
    expect(items[index].textContent, `Ronda ${number}`).toContain(
      each.user.fullName,
    );
    expect(
      within(items[index]).queryByText(each.card),
      `Ronda ${number}: ${each.user.fullName}`,
    ).not.toBeNull();
  });
  const outcome = round.getByRole("region", { name: "Resultado de la ronda" });
  expect(
    within(outcome).queryByText(outcomeLabel),
    `Ronda ${number}`,
  ).not.toBeNull();
}

// Renderiza las acciones del facilitador y devuelve las acciones observadas.
function renderActions(isFacilitator: boolean, roundStatus: RoundStatus) {
  const onReveal = vi.fn<() => void>();
  const onNewRound = vi.fn<() => void>();
  render(
    <FacilitatorActions
      isFacilitator={isFacilitator}
      roundStatus={roundStatus}
      onReveal={onReveal}
      onNewRound={onNewRound}
    />,
  );
  return { onReveal, onNewRound };
}

// Devuelve el botón "Nueva ronda", o null si la vista no lo ofrece.
function newRoundButton(): HTMLElement | null {
  return screen.queryByRole("button", { name: "Nueva ronda" });
}

describe("005/US4 - Repetir la ronda hasta acercar posiciones", () => {
  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
  });

  // Caso normal: con la ronda 1 revelada, el facilitador ve "Nueva ronda" y al pulsarlo pide la
  // ronda nueva; ya no se le ofrece revelar (FR-043).
  it("Nueva ronda", () => {
    const { onNewRound, onReveal } = renderActions(true, "revealed");

    const button = newRoundButton();
    expect(button).not.toBeNull();
    fireEvent.click(button as HTMLElement);
    expect(onNewRound).toHaveBeenCalledTimes(1);
    expect(onReveal).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "Revelar" })).toBeNull();
  });

  // Caso normal: con la ronda 2 en curso, el historial muestra la ronda 1 con el voto de cada
  // participante y su divergencia; la ronda 2 vigente no aparece ni muestra votos (FR-044, FR-045).
  it("Historial de rondas", () => {
    render(<RoundHistory rounds={[divergentFirstRound(), votingRound(2)]} />);

    expect(shownRoundNames()).toEqual(["Ronda 1"]);
    expectRoundShown(
      1,
      [vote(ana, "3"), vote(bruno, "5"), vote(carla, "13")],
      "Divergencia",
    );
    expect(
      within(historyRegion()).queryByRole("region", { name: "Ronda 2" }),
    ).toBeNull();
  });

  // Caso alternativo: la ronda 1 terminó en consenso y el facilitador abre la ronda 2 de todos
  // modos; la acción se ofrece igual y la ronda 1 queda en el historial con su consenso.
  it("Nueva ronda tras consenso", () => {
    const consensusVotes = [vote(ana, "8"), vote(bruno, "8"), vote(carla, "8")];
    const { onNewRound } = renderActions(true, "revealed");

    const button = newRoundButton();
    expect(button).not.toBeNull();
    fireEvent.click(button as HTMLElement);
    expect(onNewRound).toHaveBeenCalledTimes(1);
    cleanup();

    render(
      <RoundHistory
        rounds={[
          revealedRound(1, consensusVotes, consensus(8)),
          votingRound(2),
        ]}
      />,
    );
    expect(shownRoundNames()).toEqual(["Ronda 1"]);
    expectRoundShown(1, consensusVotes, "Consenso");
  });

  // Caso límite: en la ronda once, las diez rondas anteriores siguen completas en el historial, en
  // orden, cada una con sus votos (FR-048).
  it("Sin límite de rondas", () => {
    const previous = Array.from({ length: 10 }, (_, index) =>
      revealedRound(
        index + 1,
        [vote(ana, "3"), vote(bruno, "20")],
        divergence(3, [ana], 20, [bruno]),
      ),
    );
    render(<RoundHistory rounds={[...previous, votingRound(11)]} />);

    expect(shownRoundNames()).toEqual(
      Array.from({ length: 10 }, (_, index) => `Ronda ${index + 1}`),
    );
    for (let number = 1; number <= 10; number++) {
      expectRoundShown(
        number,
        [vote(ana, "3"), vote(bruno, "20")],
        "Divergencia",
      );
    }
  });

  // Caso de error: con la ronda vigente todavía en votación no se ofrece "Nueva ronda", para no
  // descartar votos sin mostrarlos; el facilitador solo puede revelar (FR-046).
  it("Ronda sin revelar", () => {
    const { onNewRound } = renderActions(true, "voting");

    expect(newRoundButton()).toBeNull();
    expect(screen.queryByRole("button", { name: "Revelar" })).not.toBeNull();
    expect(onNewRound).not.toHaveBeenCalled();
  });

  // Caso de error: quien no es el facilitador no tiene "Nueva ronda" con la ronda revelada; la
  // acción está reservada al facilitador (FR-047).
  it("Nueva ronda sin ser facilitador", () => {
    const { onNewRound } = renderActions(false, "revealed");

    expect(newRoundButton()).toBeNull();
    expect(onNewRound).not.toHaveBeenCalled();
  });

  // Regla de la interfaz: la ronda vigente ya revelada se ve en la sala y no se repite en el
  // historial, que solo muestra las rondas anteriores.
  it("no repite en el historial la ronda vigente revelada", () => {
    render(
      <RoundHistory
        rounds={[
          divergentFirstRound(),
          revealedRound(
            2,
            [vote(ana, "5"), vote(bruno, "8")],
            divergence(5, [ana], 8, [bruno]),
          ),
        ]}
      />,
    );

    expect(shownRoundNames()).toEqual(["Ronda 1"]);
  });

  // Regla de la interfaz: en la primera ronda no hay rondas anteriores y no se muestra el historial.
  it("no muestra el historial sin rondas anteriores", () => {
    render(<RoundHistory rounds={[votingRound(1)]} />);

    expect(
      screen.queryByRole("region", { name: "Historial de rondas" }),
    ).toBeNull();
  });
});
