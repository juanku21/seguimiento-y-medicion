// Tipos de las peticiones y respuestas de proyectos, según contracts/openapi.yaml (spec 002).

// Estado del proyecto en su ciclo de vida (FR-036).
export type ProjectStatus = "planned" | "in_progress" | "finished";

// Cuerpo de POST /api/v1/projects: datos editables del proyecto (ProjectInput).
export type ProjectInput = {
  name: string;
  description?: string | null;
  startDate: string;
  plannedEndDate: string;
  hoursPerStoryPoint: number;
};

// Propietario del proyecto (OwnerRef).
export type OwnerRef = {
  id: string;
  fullName: string;
};

// Integrante del proyecto; el correo solo llega cuando consulta el propietario (ProjectMemberView).
export type ProjectMemberView = {
  userId: string;
  fullName: string;
  email?: string;
  isOwner: boolean;
  joinedAt: string;
};

// Sprint activo del proyecto (ActiveSprintRef).
export type ActiveSprintRef = {
  id: string;
  name: string;
};

// Resumen del backlog del proyecto (BacklogSummary).
export type BacklogSummary = {
  countsByState: Record<string, number>;
  estimatedStoryPoints: number;
  estimatedHours: number;
};

// Estado completo del proyecto, respuesta de la creación (ProjectDetail).
export type ProjectDetail = {
  id: string;
  name: string;
  description?: string | null;
  status: ProjectStatus;
  startDate: string;
  plannedEndDate: string;
  actualEndDate?: string | null;
  hoursPerStoryPoint: number;
  readOnly: boolean;
  owner: OwnerRef;
  members: ProjectMemberView[];
  hasActiveSprint: boolean;
  activeSprint?: ActiveSprintRef | null;
  backlog: BacklogSummary;
};

// Elemento del listado de proyectos de quien consulta (ProjectListItem, FR-032).
export type ProjectListItem = {
  id: string;
  name: string;
  status: ProjectStatus;
  startDate: string;
  plannedEndDate: string;
  memberCount: number;
  isOwner: boolean;
};

// Respuesta de GET /api/v1/projects; message solo llega cuando la lista está vacía (FR-034).
export type ProjectListResponse = {
  projects: ProjectListItem[];
  message?: string | null;
};
