import type { Metadata } from "next";
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
  const { sprintId } = await params;

  return (
    <main className="flex w-full max-w-2xl flex-1 flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">Editar sprint</h1>
      <SessionGuard>
        <div className="flex flex-col gap-6">
          <SprintEditView sprintId={sprintId} />
        </div>
      </SessionGuard>
    </main>
  );
}
