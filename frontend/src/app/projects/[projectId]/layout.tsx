import { ProjectShell } from "@/features/projects/components/project-shell";

// Todas las pantallas de un proyecto comparten su navegación y sus migas de pan.
export default async function ProjectLayout({
  params,
  children,
}: LayoutProps<"/projects/[projectId]">) {
  const { projectId } = await params;
  return <ProjectShell projectId={projectId}>{children}</ProjectShell>;
}
