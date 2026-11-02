import type { ReactNode } from "react";
import { Alert } from "@/components/alert";

// Mensajes de falla por sección del dashboard (contracts/ui-contract.md), en un único lugar.
export const ACTIVE_SPRINT_ERROR = "No se pudo cargar el sprint activo.";
export const SPRINT_INDICATORS_ERROR =
  "No se pudieron cargar los indicadores del sprint activo.";
export const PROJECT_INDICATORS_ERROR =
  "No se pudieron cargar los indicadores del proyecto.";
export const VELOCITY_ERROR = "No se pudo cargar el gráfico de velocidad.";
export const EFFORT_ERROR = "No se pudo cargar el gráfico de esfuerzo.";
export const QUALITY_ERROR = "No se pudo cargar el gráfico de defectos.";
export const DISTRIBUTION_ERROR =
  "No se pudo cargar la distribución de defectos abiertos.";
export const WARNINGS_ERROR = "No se pudieron cargar las advertencias.";

// Muestra el mensaje de falla de una sección como alerta (role="alert").
export function SectionError({ message }: { message: string }): ReactNode {
  return <Alert>{message}</Alert>;
}
