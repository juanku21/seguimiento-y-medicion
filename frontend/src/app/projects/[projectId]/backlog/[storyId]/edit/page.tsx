import type { Metadata } from "next";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { StoryEditView } from "@/features/backlog/components/story-edit-view";

// Título propio de la pantalla de refinamiento de una historia.
export const metadata: Metadata = {
  title: "Refinar historia | Software Metrics & Estimation",
};

// Pantalla protegida de refinamiento de una historia (003/US5). Es un Server Component que solo toma
// los identificadores de la ruta y encuadra la vista: la guardia de sesión corre en el cliente,
// porque el token vive en sessionStorage, y muestra el regreso a la historia y el formulario
// únicamente cuando hay una sesión vigente.
export default async function EditStoryPage({
  params,
}: {
  params: Promise<{ projectId: string; storyId: string }>;
}) {
  const { storyId } = await params;

  return (
    <main className="flex w-full max-w-2xl flex-1 flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">
        Refinar historia
      </h1>
      <SessionGuard>
        <div className="flex flex-col gap-6">
          <StoryEditView storyId={storyId} />
        </div>
      </SessionGuard>
    </main>
  );
}
