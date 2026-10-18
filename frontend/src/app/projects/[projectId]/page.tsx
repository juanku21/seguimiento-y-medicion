import type { Metadata } from "next";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { ProjectOverview } from "@/features/projects/components/project-overview";

// Título propio de la pantalla del proyecto.
export const metadata: Metadata = {
  title: "Proyecto | Software Metrics & Estimation",
};

// Pantalla protegida del proyecto (002/US4). Es un Server Component que solo toma el identificador
// de la ruta y encuadra la vista: la guardia de sesión corre en el cliente, porque el token vive en
// sessionStorage, y la vista pide el estado únicamente cuando hay una sesión vigente, igual que las
// demás pantallas de proyectos.
export default async function ProjectPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Proyecto</h1>
      <SessionGuard>
        <ProjectOverview projectId={projectId} />
      </SessionGuard>
    </main>
  );
}
