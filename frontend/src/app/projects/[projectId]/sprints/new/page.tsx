import type { Metadata } from "next";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { SprintForm } from "@/features/sprints/components/sprint-form";

// Título propio de la pantalla de creación de sprints.
export const metadata: Metadata = {
  title: "Nuevo sprint | Software Metrics & Estimation",
};

// Pantalla protegida de creación de sprints (004/US1). Es un Server Component que solo toma el
// identificador del proyecto de la ruta y encuadra el formulario: la guardia de sesión corre en el
// cliente, porque el token vive en sessionStorage, y muestra el formulario únicamente cuando hay una
// sesión vigente.
export default async function NewSprintPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Nuevo sprint</h1>
      <SessionGuard>
        <SprintForm projectId={projectId} />
      </SessionGuard>
    </main>
  );
}
