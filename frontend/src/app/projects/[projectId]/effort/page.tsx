import type { Metadata } from "next";
import Link from "next/link";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { ProjectEffortView } from "@/features/effort/components/project-effort-view";

// Título propio de la pantalla del esfuerzo del proyecto.
export const metadata: Metadata = {
  title: "Esfuerzo | Software Metrics & Estimation",
};

// Pantalla protegida del esfuerzo del proyecto (006/US4). Es un Server Component que solo toma el
// identificador del proyecto de la ruta y encuadra la vista: la guardia de sesión corre en el
// cliente, porque el token vive en sessionStorage, y muestra el regreso al proyecto y los filtros
// con la lista y el total únicamente cuando hay una sesión vigente.
export default async function ProjectEffortPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-4xl flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Esfuerzo</h1>
      <SessionGuard>
        <div className="flex flex-col gap-6">
          <Link
            href={`/projects/${projectId}`}
            className="self-start text-sm font-medium underline underline-offset-4"
          >
            Volver al proyecto
          </Link>
          <ProjectEffortView projectId={projectId} />
        </div>
      </SessionGuard>
    </main>
  );
}
