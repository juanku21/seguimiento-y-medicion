"use client";

interface DashboardViewProps {
  projectId: string;
}

// Vista cliente del dashboard. Más adelante compone el estado del proyecto, las métricas del sprint
// activo, las métricas acumuladas y los gráficos de series; por ahora no muestra nada.
export function DashboardView({ projectId }: DashboardViewProps) {
  void projectId;
  return null;
}
