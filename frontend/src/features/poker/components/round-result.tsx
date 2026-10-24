import type { ReactElement } from "react";
import type {
  EstimationRound,
  OutcomeExtreme,
  OutcomeKind,
} from "@/types/poker";

// Propiedades del resultado: la ronda revelada tal como llega en la instantánea.
export type RoundResultProps = {
  round: EstimationRound;
};

// Etiqueta visible de cada tipo de resultado de la ronda.
const OUTCOME_LABELS: Record<OutcomeKind, string> = {
  consensus: "Consenso",
  divergence: "Divergencia",
  no_numeric_votes: "Sin votos numéricos",
};

// Resultado de una ronda revelada (005/US3). Es de presentación: lista los votos en el orden
// recibido con su autor y su carta tal cual ("?" incluido), y a continuación a los conectados que no
// votaron como "Sin voto" (FR-035, FR-038). El resultado lo calcula el backend: acá se muestra el
// consenso con su valor, la divergencia con el mínimo y el máximo y todos sus autores, o la ronda
// sin votos numéricos, sin promedio ni mediana (FR-037 a FR-042).
export function RoundResult({ round }: RoundResultProps): ReactElement {
  const votes = round.votes ?? [];
  const { outcome } = round;
  return (
    <div className="flex flex-col gap-4">
      {/* Votos en el orden recibido y, a continuación, los conectados que no votaron. */}
      <ul aria-label="Votos" className="flex flex-col gap-1 text-sm">
        {votes.map((vote) => (
          <li key={vote.user.id} className="flex justify-between gap-4">
            <span className="break-words">{vote.user.fullName}</span>
            <span className="font-semibold">{vote.card}</span>
          </li>
        ))}
        {round.notVoted.map((user) => (
          <li key={user.id} className="flex justify-between gap-4">
            <span className="break-words">{user.fullName}</span>
            <span className="text-zinc-600 dark:text-zinc-400">Sin voto</span>
          </li>
        ))}
      </ul>

      {/* Resultado calculado por el backend: el valor del consenso o los extremos de la
          divergencia; sin votos numéricos solo se muestra la etiqueta. */}
      {outcome !== null && (
        <section
          aria-label="Resultado de la ronda"
          className="flex flex-col gap-3 rounded-md border border-zinc-300 px-4 py-4 dark:border-zinc-700"
        >
          <p className="text-base font-semibold">
            {OUTCOME_LABELS[outcome.kind]}
          </p>
          {outcome.kind === "consensus" &&
            outcome.value !== null &&
            outcome.value !== undefined && (
              <p className="text-2xl font-semibold">{outcome.value}</p>
            )}
          {outcome.kind === "divergence" && (
            <div className="flex flex-wrap gap-6">
              {outcome.min && <Extreme name="Mínimo" extreme={outcome.min} />}
              {outcome.max && <Extreme name="Máximo" extreme={outcome.max} />}
            </div>
          )}
        </section>
      )}
    </div>
  );
}

// Extremo de la divergencia: su valor y la lista de todos sus autores en el orden recibido, que el
// backend ordena por nombre (FR-039).
function Extreme({
  name,
  extreme,
}: {
  name: "Mínimo" | "Máximo";
  extreme: OutcomeExtreme;
}): ReactElement {
  return (
    <div role="group" aria-label={name} className="flex flex-col gap-1">
      <span className="text-sm text-zinc-600 dark:text-zinc-400">{name}</span>
      <span className="text-2xl font-semibold">{extreme.value}</span>
      <ul className="flex flex-col gap-0.5 text-sm">
        {extreme.authors.map((author) => (
          <li key={author.id} className="break-words">
            {author.fullName}
          </li>
        ))}
      </ul>
    </div>
  );
}
