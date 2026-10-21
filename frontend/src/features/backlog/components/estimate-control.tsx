"use client";

import type { ReactNode } from "react";
import type { StoryDetail, StoryPoints } from "@/types/story";

// Propiedades del control de estimación: la historia que se estima, su valor vigente y el aviso a
// la pantalla de detalle con el StoryDetail que devuelve el backend tras estimar.
export type EstimateControlProps = {
  storyId: string;
  storyPoints: StoryPoints;
  onEstimated: (story: StoryDetail) => void;
};

// Control de estimación de una historia en Story Points (003/US4). Firma mínima para las pruebas de
// T050; la implementación corresponde a T055.
export function EstimateControl(props: EstimateControlProps): ReactNode {
  void props;
  throw new Error("not implemented");
}
