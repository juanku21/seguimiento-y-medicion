interface ActiveSprintPanelProps {
  activeSprintId: string | null;
}

// Panel del sprint activo del dashboard. Firma mínima: todavía no muestra nada.
export function ActiveSprintPanel({ activeSprintId }: ActiveSprintPanelProps) {
  void activeSprintId;
  return null;
}
