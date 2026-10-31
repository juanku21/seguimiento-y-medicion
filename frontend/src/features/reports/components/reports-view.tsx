"use client";

// Propiedades de la vista: el proyecto cuyos reportes se muestran.
interface ReportsViewProps {
  projectId: string;
}

// Vista de reportes del proyecto (010). Todavía no muestra nada: más adelante compondrá el selector
// de sprints, la generación del reporte de sprint y la del reporte consolidado del proyecto.
export function ReportsView({ projectId }: ReportsViewProps) {
  void projectId;
  return null;
}
