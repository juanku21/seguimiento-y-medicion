import type { ReactNode } from "react";
import type { StoryDetail } from "@/types/story";

// Propiedades de la tarjeta de detalle: la historia completa tal como la devuelve la API.
export type StoryDetailCardProps = {
  story: StoryDetail;
};

// Tarjeta con el detalle de una historia (003/US3). Firma mínima de T041: la implementa T046.
export function StoryDetailCard({ story }: StoryDetailCardProps): ReactNode {
  throw new Error("not implemented");
}
