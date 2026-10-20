import type { Metadata } from "next";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { StoryForm } from "@/features/backlog/components/story-form";

// Título propio de la pantalla de alta de historias.
export const metadata: Metadata = {
  title: "Nueva historia | Software Metrics & Estimation",
};

// Pantalla protegida de alta de historias en el backlog (003/US1). Es un Server Component que solo
// toma el identificador del proyecto de la ruta y encuadra el formulario: la guardia de sesión corre
// en el cliente, porque el token vive en sessionStorage, y muestra el formulario únicamente cuando
// hay una sesión vigente.
export default async function NewStoryPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Nueva historia</h1>
      <SessionGuard>
        <StoryForm projectId={projectId} />
      </SessionGuard>
    </main>
  );
}
