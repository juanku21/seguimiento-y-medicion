"use client";

import type { ReactNode } from "react";
import type { EffortEntry } from "@/types/effort";

// Propiedades del formulario: la historia sobre la que se registra y el aviso del alta exitosa.
export type EffortEntryFormProps = {
  storyId: string;
  onCreated?: (entry: EffortEntry) => void;
};

// Formulario de registro de esfuerzo con fecha, actividad y horas.
// Firma mínima de T018: la implementación corresponde a T033.
export function EffortEntryForm(props: EffortEntryFormProps): ReactNode {
  throw new Error("not implemented");
}
