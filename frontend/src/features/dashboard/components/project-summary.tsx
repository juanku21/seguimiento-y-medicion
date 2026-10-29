import { MemberList } from "@/features/projects/components/member-list";
import type { ProjectDetail } from "@/types/project";
import { formatDate } from "@/utils/format-date";
import { PROJECT_STATUS_LABELS } from "@/utils/project-status";

// Propiedades del resumen: el detalle del proyecto tal como lo devuelve la 002.
interface ProjectSummaryProps {
  project: ProjectDetail;
}

// Resumen del proyecto del dashboard (009/US1): nombre, estado, fechas e integrantes. La fecha de
// finalización real solo se muestra si el proyecto está finalizado y la tiene (FR-006). Los
// integrantes se listan en solo lectura y sin acciones de administración (FR-003, FR-007).
export function ProjectSummary({ project }: ProjectSummaryProps) {
  return (
    <section
      aria-labelledby="project-summary-heading"
      className="flex flex-col gap-2"
    >
      <h2 id="project-summary-heading" className="text-lg font-semibold">
        Resumen del proyecto
      </h2>
      <h3 className="text-base font-medium">{project.name}</h3>
      <p className="text-sm">Estado: {PROJECT_STATUS_LABELS[project.status]}</p>
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        Inicio: {formatDate(project.startDate)}
      </p>
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        Finalización prevista: {formatDate(project.plannedEndDate)}
      </p>
      {project.status === "finished" && project.actualEndDate ? (
        <p className="text-sm text-zinc-600 dark:text-zinc-400">
          Finalización real: {formatDate(project.actualEndDate)}
        </p>
      ) : null}
      <MemberList
        projectId={project.id}
        members={project.members}
        viewerIsOwner={false}
        readOnly
      />
    </section>
  );
}
