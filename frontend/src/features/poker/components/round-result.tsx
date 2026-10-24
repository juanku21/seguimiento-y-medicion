import type { ReactElement } from "react";
import type { EstimationRound } from "@/types/poker";

// Propiedades del resultado: la ronda revelada tal como llega en la instantánea.
export type RoundResultProps = {
  round: EstimationRound;
};

// Resultado de una ronda revelada (005/US3). Firma mínima para las pruebas de T049; la
// implementación llega en T055.
export function RoundResult({ round }: RoundResultProps): ReactElement {
  void round;
  throw new Error("not implemented");
}
