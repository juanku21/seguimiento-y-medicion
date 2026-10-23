import type { ReactElement } from "react";
import type { EstimationParticipant } from "@/types/poker";

// Propiedades de la lista de participantes: los participantes tal como llegan en la instantánea.
export type ParticipantListProps = {
  participants: EstimationParticipant[];
};

// Lista de participantes de la sesión con su conexión y la marca de facilitador (005/US1). Firma
// mínima de T017: la implementación es de T031.
export function ParticipantList({
  participants,
}: ParticipantListProps): ReactElement {
  void participants;
  throw new Error("not implemented");
}
