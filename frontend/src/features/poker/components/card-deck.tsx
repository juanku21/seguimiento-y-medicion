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

// Mazo de cartas de la ronda vigente (005/US2). Es de presentación: muestra un botón por carta en el
// orden recibido, resalta la carta propia con aria-pressed (FR-024) y juega la carta pulsada con
// onVote; con la ronda revelada todas las cartas quedan deshabilitadas (FR-026). El secreto del voto
// lo resuelve el backend: acá solo llega la carta propia en myVote.
export function CardDeck({
  cards,
  myVote,
  roundStatus,
  onVote,
}: CardDeckProps): ReactElement {
  const disabled = roundStatus === "revealed";
  return (
    <div
      role="group"
      aria-label="Mazo de cartas"
      className="flex flex-wrap gap-3"
    >
      {cards.map((card) => {
        const selected = card === myVote;
        return (
          <button
            key={card}
            type="button"
            aria-pressed={selected}
            disabled={disabled}
            onClick={() => onVote(card)}
            className={`h-24 w-16 rounded-lg border-2 text-2xl font-semibold tabular-nums transition motion-safe:hover:enabled:-translate-y-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:cursor-not-allowed disabled:opacity-50 ${
              selected
                ? "border-primary bg-primary text-on-primary"
                : "border-border bg-surface hover:border-border-strong"
            }`}
          >
            {card}
          </button>
        );
      })}
    </div>
  );
}
