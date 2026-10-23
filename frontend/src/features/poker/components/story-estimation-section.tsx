"use client";

import type { ReactElement } from "react";

// Propiedades de la sección "Planning Poker" del detalle de una historia: el proyecto y la historia
// tomados de la ruta.
export type StoryEstimationSectionProps = {
  projectId: string;
  storyId: string;
};

// Sección "Planning Poker" del detalle de una historia: abre una sesión o se une a la que está en
// curso (005/US1). Firma mínima de T017: la implementación es de T031.
export function StoryEstimationSection({
  projectId,
  storyId,
}: StoryEstimationSectionProps): ReactElement {
  void projectId;
  void storyId;
  throw new Error("not implemented");
}
