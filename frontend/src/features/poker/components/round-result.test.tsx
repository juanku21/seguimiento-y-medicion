import { afterEach, describe, expect, it } from "vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import { RoundResult } from "@/features/poker/components/round-result";
import type {
  Card,
  EstimationRound,
  EstimationVote,
  RoundOutcome,
  ScaleStoryPoints,
  UserRef,
} from "@/types/poker";

// Pruebas del resultado de una ronda revelada de la sala de Planning Poker (T049). El resultado
// recibe la ronda tal como llega en cada instantánea del canal: el consenso, la divergencia con sus
// extremos y sus autores, y la lista de conectados sin voto los calcula el backend (Summarize y
// ViewFor). Acá se verifica qué muestra la vista con cada ronda revelada, sin calcular nada.
//
// Contrato que fijan estas pruebas para RoundResult:
// - Recibe round (EstimationRound) con status "revealed", votes y outcome.
// - Muestra una lista con nombre accesible "Votos" con un elemento por voto, en el orden recibido,
//   con el nombre completo del autor y su carta como texto exacto ("?" se muestra como tal); a
//   continuación, un elemento por cada participante de notVoted con su nombre y el texto exacto
//   "Sin voto".
// - Muestra una región con nombre accesible "Resultado de la ronda" con la etiqueta del tipo de
//   resultado como texto exacto ("Consenso", "Divergencia" o "Sin votos numéricos").
// - En consenso, la región muestra el valor como texto exacto y no muestra mínimo ni máximo.
// - En divergencia, la región muestra un grupo con nombre accesible "Mínimo" y otro "Máximo"; cada
//   uno muestra su valor como texto exacto y una lista con un elemento por autor, en el orden
//   recibido (el backend los ordena por nombre).
// - Sin votos numéricos, la región no muestra consenso, mínimo ni máximo.
// - Nunca muestra promedio ni mediana (FR-042).

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
const dario: UserRef = {
  id: "9c2d4e60-1b7a-4f35-8e09-3a6f1d2c8b57",
  fullName: "Darío López",
};

// Arma el voto visible de un participante.
function vote(user: UserRef, card: Card): EstimationVote {
  return { user, card };
}

// Arma la ronda vigente ya revelada con sus votos, su resultado y los conectados sin voto.
function revealedRound(
  votes: EstimationVote[],
  outcome: RoundOutcome,
  notVoted: UserRef[] = [],
): EstimationRound {
  return {
    number: 1,
    status: "revealed",
    openedAt: "2026-10-23T14:00:00Z",
    revealedAt: "2026-10-23T14:03:00Z",
    votes,
    outcome,
    notVoted,
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

// Resultado de una ronda sin ningún voto numérico.
const NO_NUMERIC_VOTES: RoundOutcome = {
  kind: "no_numeric_votes",
  value: null,
  min: null,
  max: null,
};

// Devuelve los elementos de la lista de votos, en el orden mostrado.
function shownVotes(): HTMLElement[] {
  const list = screen.getByRole("list", { name: "Votos" });
  return within(list).getAllByRole("listitem");
}

// Verifica que un elemento de la lista de votos muestre a su autor con la carta indicada, o con
// "Sin voto" cuando card es null.
function expectVote(item: HTMLElement, user: UserRef, card: Card | null) {
  expect(item.textContent, user.fullName).toContain(user.fullName);
  const scope = within(item);
  if (card === null) {
    expect(scope.queryByText("Sin voto"), user.fullName).not.toBeNull();
  } else {
    expect(scope.queryByText(card), user.fullName).not.toBeNull();
    expect(scope.queryByText("Sin voto"), user.fullName).toBeNull();
  }
}

// Devuelve la región del resultado de la ronda.
function outcomeRegion(): HTMLElement {
  return screen.getByRole("region", { name: "Resultado de la ronda" });
}

// Verifica que la región muestre solo la etiqueta del tipo de resultado indicada.
function expectOutcomeLabel(label: string) {
  const scope = within(outcomeRegion());
  for (const each of ["Consenso", "Divergencia", "Sin votos numéricos"]) {
    if (each === label) {
      expect(scope.queryByText(each), each).not.toBeNull();
    } else {
      expect(scope.queryByText(each), each).toBeNull();
    }
  }
}

// Verifica que la región no muestre ningún extremo.
function expectNoExtremes() {
  const scope = within(outcomeRegion());
  expect(scope.queryByRole("group", { name: "Mínimo" })).toBeNull();
  expect(scope.queryByRole("group", { name: "Máximo" })).toBeNull();
}

// Verifica un extremo de la divergencia: su valor y todos sus autores en el orden recibido.
function expectExtreme(
  name: "Mínimo" | "Máximo",
  value: ScaleStoryPoints,
  authors: UserRef[],
) {
  const group = within(outcomeRegion()).getByRole("group", { name });
  const scope = within(group);
  expect(scope.queryByText(`${value}`), name).not.toBeNull();
  const items = scope.getAllByRole("listitem");
  expect(
    items.map((item) => item.textContent?.trim()),
    name,
  ).toEqual(authors.map((author) => author.fullName));
}

// Verifica que la vista no muestre ninguna agregación fuera del consenso, el mínimo y el máximo.
function expectNoAggregates() {
  const text = document.body.textContent ?? "";
  expect(text).not.toMatch(/promedio|mediana/i);
}

describe("005/US3 - Revelar los votos y ver las diferencias", () => {
  // Sin globals de Vitest, la limpieza del DOM de Testing Library se hace a mano.
  afterEach(() => {
    cleanup();
  });

  // Caso normal: Ana, Bruno y Carla votaron 3, 5 y 13; la ronda revelada muestra el voto de cada
  // uno, la divergencia, el mínimo 3 con Ana y el máximo 13 con Carla (FR-035, FR-037, FR-039).
  it("Revelación automática", () => {
    render(
      <RoundResult
        round={revealedRound(
          [vote(ana, "3"), vote(bruno, "5"), vote(carla, "13")],
          divergence(3, [ana], 13, [carla]),
        )}
      />,
    );

    const items = shownVotes();
    expect(items).toHaveLength(3);
    expectVote(items[0], ana, "3");
    expectVote(items[1], bruno, "5");
    expectVote(items[2], carla, "13");
    expectOutcomeLabel("Divergencia");
    expectExtreme("Mínimo", 3, [ana]);
    expectExtreme("Máximo", 13, [carla]);
    expectNoAggregates();
  });

  // Caso normal: tres votaron 8 y Darío, conectado, no votó; tras la revelación manual figuran los
  // tres votos, Darío como "Sin voto" y el consenso en 8 (FR-032, FR-035, FR-036).
  it("Revelación manual con consenso", () => {
    render(
      <RoundResult
        round={revealedRound(
          [vote(ana, "8"), vote(bruno, "8"), vote(carla, "8")],
          consensus(8),
          [dario],
        )}
      />,
    );

    const items = shownVotes();
    expect(items).toHaveLength(4);
    expectVote(items[0], ana, "8");
    expectVote(items[1], bruno, "8");
    expectVote(items[2], carla, "8");
    expectVote(items[3], dario, null);
    expectOutcomeLabel("Consenso");
    expect(within(outcomeRegion()).queryByText("8")).not.toBeNull();
    expectNoExtremes();
  });

  // Caso alternativo: dos votaron 5 y Carla votó "?"; el "?" se muestra como tal y el consenso es en
  // 5, porque el "?" no participa del cálculo (FR-038).
  it("Interrogación fuera del cálculo", () => {
    render(
      <RoundResult
        round={revealedRound(
          [vote(ana, "5"), vote(bruno, "5"), vote(carla, "?")],
          consensus(5),
        )}
      />,
    );

    const items = shownVotes();
    expect(items).toHaveLength(3);
    expectVote(items[0], ana, "5");
    expectVote(items[1], bruno, "5");
    expectVote(items[2], carla, "?");
    expectOutcomeLabel("Consenso");
    expect(within(outcomeRegion()).queryByText("5")).not.toBeNull();
    expectNoExtremes();
  });

  // Caso alternativo: los tres votaron 20; el consenso es en 20 y no se señalan mínimo ni máximo
  // porque no hay diferencias que explicar (FR-040).
  it("Consenso sin extremos", () => {
    render(
      <RoundResult
        round={revealedRound(
          [vote(ana, "20"), vote(bruno, "20"), vote(carla, "20")],
          consensus(20),
        )}
      />,
    );

    expect(shownVotes()).toHaveLength(3);
    expectOutcomeLabel("Consenso");
    expect(within(outcomeRegion()).queryByText("20")).not.toBeNull();
    expectNoExtremes();
    expectNoAggregates();
  });

  // Caso límite: el único participante conectado votó 13; la ronda muestra su voto y el consenso en
  // su valor.
  it("Único participante", () => {
    render(
      <RoundResult round={revealedRound([vote(ana, "13")], consensus(13))} />,
    );

    const items = shownVotes();
    expect(items).toHaveLength(1);
    expectVote(items[0], ana, "13");
    expectOutcomeLabel("Consenso");
    expect(within(outcomeRegion()).queryByText("13")).not.toBeNull();
    expectNoExtremes();
  });

  // Caso límite: los tres votaron "?"; no hay consenso ni extremos y el resultado indica que la
  // ronda no tuvo ningún voto numérico (FR-041).
  it("Todos interrogación", () => {
    render(
      <RoundResult
        round={revealedRound(
          [vote(ana, "?"), vote(bruno, "?"), vote(carla, "?")],
          NO_NUMERIC_VOTES,
        )}
      />,
    );

    const items = shownVotes();
    expect(items).toHaveLength(3);
    expectVote(items[0], ana, "?");
    expectVote(items[1], bruno, "?");
    expectVote(items[2], carla, "?");
    expectOutcomeLabel("Sin votos numéricos");
    expectNoExtremes();
  });

  // Caso límite: Ana y Darío votaron 3 y Bruno y Carla votaron 40; se señalan los dos autores del
  // mínimo y los dos del máximo, en el orden por nombre en que llegan (FR-039).
  it("Autores múltiples", () => {
    render(
      <RoundResult
        round={revealedRound(
          [
            vote(ana, "3"),
            vote(bruno, "40"),
            vote(carla, "40"),
            vote(dario, "3"),
          ],
          divergence(3, [ana, dario], 40, [bruno, carla]),
        )}
      />,
    );

    expect(shownVotes()).toHaveLength(4);
    expectOutcomeLabel("Divergencia");
    expectExtreme("Mínimo", 3, [ana, dario]);
    expectExtreme("Máximo", 40, [bruno, carla]);
    expectNoAggregates();
  });
});
