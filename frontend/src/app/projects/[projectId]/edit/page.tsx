import type { Metadata } from "next";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { ProjectEditView } from "@/features/projects/components/project-edit-view";

// Título propio de la pantalla de modificación del proyecto.
export const metadata: Metadata = {
  title: "Editar proyecto | Software Metrics & Estimation",
};

// Pantalla protegida de modificación del proyecto (002/US5). Es un Server Component que solo toma
// el identificador de la ruta y encuadra la vista: la guardia de sesión corre en el cliente, porque
// el token vive en sessionStorage, y la vista pide el proyecto únicamente cuando hay una sesión
// vigente y muestra el formulario solo al propietario.
export default async function EditProjectPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Editar proyecto</h1>
      <SessionGuard>
        <ProjectEditView projectId={projectId} />
      </SessionGuard>
    </main>
  );
}
