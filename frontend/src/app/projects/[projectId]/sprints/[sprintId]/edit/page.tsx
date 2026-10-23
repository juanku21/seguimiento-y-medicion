import type { Metadata } from "next";
import Link from "next/link";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { SprintEditView } from "@/features/sprints/components/sprint-edit-view";

// Título propio de la pantalla de modificación de un sprint.
export const metadata: Metadata = {
  title: "Editar sprint | Software Metrics & Estimation",
};

// Pantalla protegida de modificación de un sprint (004/US7). Es un Server Component que solo toma
// los identificadores de la ruta y encuadra la vista: la guardia de sesión corre en el cliente,
// porque el token vive en sessionStorage, y muestra el regreso al sprint y el formulario únicamente
// cuando hay una sesión vigente.
export default async function EditSprintPage({
  params,
}: {
  params: Promise<{ projectId: string; sprintId: string }>;
}) {
  const { projectId, sprintId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Editar sprint</h1>
      <SessionGuard>
        <div className="flex flex-col gap-6">
          <Link
            href={`/projects/${projectId}/sprints/${sprintId}`}
            className="self-start text-sm font-medium underline underline-offset-4"
          >
            Volver al sprint
          </Link>
          <SprintEditView sprintId={sprintId} />
        </div>
      </SessionGuard>
    </main>
  );
}
