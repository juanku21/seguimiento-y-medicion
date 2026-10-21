"use client";

import type { ReactNode } from "react";

// Propiedades de la vista de refinamiento: el identificador de la historia tomado de la ruta.
export type StoryEditViewProps = {
  storyId: string;
};

// Vista de refinamiento de una historia (003/US5). Firma mínima para las pruebas de T060; la
// implementación corresponde a T065.
export function StoryEditView(props: StoryEditViewProps): ReactNode {
  void props;
  throw new Error("not implemented");
}
