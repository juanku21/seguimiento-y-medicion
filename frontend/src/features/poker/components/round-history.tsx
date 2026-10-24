import type { ReactElement } from "react";
import { RoundResult } from "@/features/poker/components/round-result";
import type { EstimationRound } from "@/types/poker";

// Propiedades del historial: todas las rondas de la instantánea en orden ascendente; la última es
// la ronda vigente.
export type RoundHistoryProps = {
  rounds: EstimationRound[];
};

// Historial de las rondas anteriores de la sesión (005/US4). Es de presentación: descarta la última
// ronda, que es la vigente y ya se ve en la sala, y muestra cada ronda anterior en el orden recibido
// con sus votos y su resultado en el mismo formato que RoundResult (FR-044, FR-045, FR-048). Sin
// rondas anteriores no muestra nada.
export function RoundHistory({
  rounds,
}: RoundHistoryProps): ReactElement | null {
  const previous = rounds.slice(0, -1);
  if (previous.length === 0) {
    return null;
  }
  return (
    <section aria-label="Historial de rondas" className="flex flex-col gap-4">
      <h2 className="text-lg font-semibold">Historial de rondas</h2>
      {previous.map((round) => (
        <section
          key={round.number}
          aria-label={`Ronda ${round.number}`}
          className="flex flex-col gap-3"
        >
          <h3 className="text-base font-medium">Ronda {round.number}</h3>
          <RoundResult round={round} />
        </section>
      ))}
    </section>
  );
}
