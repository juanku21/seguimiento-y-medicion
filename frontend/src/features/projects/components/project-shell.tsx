"use client";

import { useCallback, type ReactNode } from "react";
import { Breadcrumbs } from "@/components/breadcrumbs";
import { ProjectNav } from "@/components/project-nav";
import { getProjectDetail } from "@/features/projects/api";
import { useAuthorizedQuery } from "@/hooks/use-authorized-query";

// Estructura de las pantallas de un proyecto: navegación de secciones y migas de pan. El nombre del
// proyecto solo ilustra las migas: si la consulta falla, cada pantalla informa su propio error.
export function ProjectShell({
  projectId,
  children,
}: {
  projectId: string;
  children: ReactNode;
}) {
  const load = useCallback(() => getProjectDetail(projectId), [projectId]);
  const project = useAuthorizedQuery(load);
  const projectName = project.status === "loaded" ? project.data.name : null;

  return (
    <div className="flex flex-1 flex-col gap-6 lg:flex-row lg:gap-8">
      <aside className="shrink-0 lg:sticky lg:top-22 lg:w-52 lg:self-start">
        <ProjectNav projectId={projectId} />
      </aside>
      <div className="flex min-w-0 flex-1 flex-col gap-4">
        <Breadcrumbs projectName={projectName} />
        {children}
      </div>
    </div>
  );
}
