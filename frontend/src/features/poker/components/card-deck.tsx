import type { ReactElement } from "react";
import type { Card, RoundStatus } from "@/types/poker";

// Propiedades del mazo: las cartas de la instantánea, la carta propia, el estado de la ronda vigente
// y la acción que juega una carta.
export type CardDeckProps = {
  cards: Card[];
  myVote: Card | null;
  roundStatus: RoundStatus;
  onVote: (card: Card) => void;
};

// Mazo de cartas de la ronda vigente (005/US2). Firma mínima de T037: la implementación es de T044.
export function CardDeck({
  cards,
  myVote,
  roundStatus,
  onVote,
}: CardDeckProps): ReactElement {
  void cards;
  void myVote;
  void roundStatus;
  void onVote;
  throw new Error("not implemented");
}
