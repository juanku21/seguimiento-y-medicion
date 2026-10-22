"use client";

import type { SprintStoryItem, StoryProgressResponse } from "@/types/sprint";

// Propiedades del control de avance: la historia comprometida en el sprint Activo y onChanged, que
// recibe el avance registrado por el backend.
export type StoryProgressControlProps = {
  story: SprintStoryItem;
  onChanged: (progress: StoryProgressResponse) => void;
};

// Control de avance de una historia del sprint Activo (004/US4). Pendiente de implementar en T061.
export function StoryProgressControl({
  story,
  onChanged,
}: StoryProgressControlProps) {
  void story;
  void onChanged;
  return null;
}
