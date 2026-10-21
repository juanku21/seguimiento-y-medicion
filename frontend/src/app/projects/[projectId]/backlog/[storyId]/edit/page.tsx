import type { Metadata } from "next";
import Link from "next/link";
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
  const { projectId, storyId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">
        Refinar historia
      </h1>
      <SessionGuard>
        <div className="flex flex-col gap-6">
          <Link
            href={`/projects/${projectId}/backlog/${storyId}`}
            className="self-start text-sm font-medium underline underline-offset-4"
          >
            Volver a la historia
          </Link>
          <StoryEditView storyId={storyId} />
        </div>
      </SessionGuard>
    </main>
  );
}
