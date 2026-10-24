import type { ReactElement } from "react";
import type { EstimationRound } from "@/types/poker";

// Propiedades del historial: todas las rondas de la instantánea en orden ascendente; la última es
// la ronda vigente.
export type RoundHistoryProps = {
  rounds: EstimationRound[];
};

// Historial de las rondas anteriores de la sesión (005/US4). Firma mínima para las pruebas de T057;
// la implementación llega en T060.
export function RoundHistory({
  rounds,
}: RoundHistoryProps): ReactElement | null {
  void rounds;
  throw new Error("not implemented");
}
