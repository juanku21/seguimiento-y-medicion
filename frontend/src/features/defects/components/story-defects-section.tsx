"use client";

import type { ReactElement } from "react";

// Propiedades de la sección: el proyecto y la historia cuyos defectos se muestran.
export type StoryDefectsSectionProps = {
  projectId: string;
  storyId: string;
};

// Firma mínima (T040): sección "Defectos" del detalle de la historia. La implementa T045.
export function StoryDefectsSection(
  props: StoryDefectsSectionProps,
): ReactElement {
  void props;
  throw new Error("not implemented");
}
