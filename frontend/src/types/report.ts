// Tipos de la respuesta de los reportes, según specs/010-project-reports/contracts/openapi.yaml.

import type { ProjectMetricsResponse, SprintMetrics } from "@/types/metrics";
import type { ProjectStatus } from "@/types/project";
import type { SprintStatus } from "@/types/sprint";
import type { StoryPriority } from "@/types/story";

// Alcance del reporte.
export type ReportScope = "sprint" | "project";

// Estado de una sección: con datos, sin datos o no disponible.
export type SectionState = "complete" | "empty" | "unavailable";

// Estado de un grupo dentro de una sección.
export type GroupState = "complete" | "empty";

// Resultado o estado de una historia en el reporte.
export type StoryResult =
  "pending" | "in_progress" | "completed" | "not_completed";

// Resultado del Planning Poker de una historia.
export type PokerState = "agreed" | "no_agreement" | "none";

// Sprint del reporte; solo puede ser Activo o Cerrado.
export type ReportSprint = {
  id: string;
  name: string;
  status: Extract<SprintStatus, "active" | "closed">;
  startDate: string;
  endDate: string;
  closedAt: string | null;
};

// Período cubierto por el reporte.
export type Period = {
  startDate: string;
  endDate: string;
};

// Encabezado del reporte; sprint es null en el alcance de proyecto y period en un proyecto sin sprints reportables.
export type ReportHeader = {
  projectId: string;
  projectName: string;
  projectStatus: ProjectStatus;
  sprint: ReportSprint | null;
  period: Period | null;
  generatedAt: string;
  generatedBy: { id: string; fullName: string };
};

// Línea de la sección de historias.
export type StoryLine = {
  storyId: string;
  title: string;
  priority: StoryPriority;
  storyPoints: number | null;
  status: StoryResult;
};

// Grupo de historias; sprint es null en el de no planificadas.
export type StoryGroup = {
  sprint: ReportSprint | null;
  state: GroupState;
  stories: StoryLine[];
};

// Sección de historias; groups es null con `unavailable`.
export type StoriesSection = {
  state: SectionState;
  groups: StoryGroup[] | null;
};

// Resumen de la última sesión de poker finalizada.
export type PokerSummary = {
  rounds: number;
  agreedStoryPoints: number;
  finishedAt: string;
};

// Línea de la sección de estimaciones; session solo viene con `agreed`.
export type EstimationLine = {
  storyId: string;
  title: string;
  storyPoints: number | null;
  poker: PokerState;
  session: PokerSummary | null;
};

// Grupo de estimaciones; sprint es null en el de no planificadas.
export type EstimationGroup = {
  sprint: ReportSprint | null;
  state: GroupState;
  items: EstimationLine[];
};

// Sección de estimaciones; groups es null con `unavailable`.
export type EstimationsSection = {
  state: SectionState;
  groups: EstimationGroup[] | null;
};

// Horas registradas en una historia.
export type StoryEffort = {
  storyId: string;
  title: string;
  hours: number;
};

// Esfuerzo de un sprint, por historia.
export type EffortGroup = {
  sprint: ReportSprint;
  state: GroupState;
  stories: StoryEffort[];
  totalHours: number;
};

// Horas registradas por integrante.
export type MemberEffort = {
  userId: string;
  fullName: string;
  hours: number;
};

// Sección de esfuerzo; groups, byMember y totalHours son null con `unavailable`.
export type EffortSection = {
  state: SectionState;
  groups: EffortGroup[] | null;
  byMember: MemberEffort[] | null;
  totalHours: number | null;
};

// Sección de métricas; solo uno de sprint o project está presente, ambos null con `unavailable`.
export type MetricsSection = {
  state: SectionState;
  sprint: SprintMetrics | null;
  project: ProjectMetricsResponse | null;
};

// Línea de la sección de defectos.
export type DefectLine = {
  defectId: string;
  description: string;
  severity: "critical" | "high" | "medium" | "low";
  status: "open" | "in_progress" | "resolved";
  story: { id: string; title: string };
  detectedSprint: { id: string; name: string };
  resolvedSprint: { id: string; name: string } | null;
};

// Sección de defectos; defects y totals son null con `unavailable`.
export type DefectsSection = {
  state: SectionState;
  defects: DefectLine[] | null;
  totals: { detected: number; resolved: number } | null;
};

// Reporte completo de un sprint o del proyecto.
export type Report = {
  scope: ReportScope;
  partial: boolean;
  header: ReportHeader;
  stories: StoriesSection;
  estimations: EstimationsSection;
  effort: EffortSection;
  metrics: MetricsSection;
  defects: DefectsSection;
};
