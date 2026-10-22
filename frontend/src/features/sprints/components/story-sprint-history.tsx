import type { ReactNode } from "react";

// Propiedades del historial de sprints de una historia: la historia cuyo historial se consulta.
export type StorySprintHistoryProps = {
  storyId: string;
};

// Sprints en los que estuvo una historia y su resultado en cada uno (004/US6). Firma mínima para
// las pruebas de T075; la implementación corresponde a T080.
export function StorySprintHistory(props: StorySprintHistoryProps): ReactNode {
  void props;
  throw new Error("not implemented");
}
