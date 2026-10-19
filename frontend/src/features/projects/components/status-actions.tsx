"use client";

import { useRef, useState } from "react";
import { Alert } from "@/components/alert";
import { Button } from "@/components/button";
import { changeProjectStatus } from "@/features/projects/api";
import type { ProjectDetail, ProjectStatus } from "@/types/project";

// Propiedades de las acciones de estado: el proyecto tal como lo devuelve la API y la función que
// recibe el proyecto actualizado cuando el backend acepta la transición.
export type StatusActionsProps = {
  project: ProjectDetail;
  onStatusChange: (project: ProjectDetail) => void;
};

// Transición de estado que se ofrece: el estado destino y el texto del botón.
type Transition = {
  target: ProjectStatus;
  label: string;
};

// Única transición disponible desde cada estado (FR-037): Planificado pasa a En curso, En curso
// pasa a Finalizado y Finalizado no tiene salida.
const NEXT_TRANSITION: Record<ProjectStatus, Transition | null> = {
  planned: { target: "in_progress", label: "Pasar a En curso" },
  in_progress: { target: "finished", label: "Finalizar proyecto" },
  finished: null,
};

// Acciones para avanzar el estado del proyecto (002/US6). Ofrece solo la transición disponible
// según el estado actual; las reglas las valida el backend y sus rechazos se muestran con el
// mensaje recibido, sin tocar el proyecto.
export function StatusActions({ project, onStatusChange }: StatusActionsProps) {
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const submittingRef = useRef(false);
  const transition = NEXT_TRANSITION[project.status];

  // Envía el cambio de estado. La referencia descarta un segundo envío mientras el primero sigue en
  // curso; si el backend acepta, se entrega el proyecto recibido.
  async function handleClick(target: ProjectStatus) {
    if (submittingRef.current) {
      return;
    }

    submittingRef.current = true;
    setIsSubmitting(true);
    setError(null);
    const result = await changeProjectStatus(project.id, target);
    submittingRef.current = false;
    setIsSubmitting(false);

    if (!result.ok) {
      setError(result.error.message);
      return;
    }
    onStatusChange(result.data);
  }

  if (transition === null) {
    return null;
  }

  return (
    <div className="flex flex-col gap-4">
      {error && <Alert>{error}</Alert>}
      <Button
        disabled={isSubmitting}
        onClick={() => handleClick(transition.target)}
      >
        {transition.label}
      </Button>
    </div>
  );
}
