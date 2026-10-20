import type { Metadata } from "next";
import Link from "next/link";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { StoryDetailView } from "@/features/backlog/components/story-detail-view";

// Título propio de la pantalla de detalle de una historia.
export const metadata: Metadata = {
  title: "Historia | Software Metrics & Estimation",
};

// Pantalla protegida del detalle de una historia (003/US3). Es un Server Component que solo toma los
// identificadores de la ruta y encuadra la vista: la guardia de sesión corre en el cliente, porque
// el token vive en sessionStorage, y muestra el regreso al backlog y el detalle únicamente cuando
// hay una sesión vigente.
export default async function StoryPage({
  params,
}: {
  params: Promise<{ projectId: string; storyId: string }>;
}) {
  const { projectId, storyId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Historia</h1>
      <SessionGuard>
        <div className="flex flex-col gap-6">
          <Link
            href={`/projects/${projectId}/backlog`}
            className="self-start text-sm font-medium underline underline-offset-4"
          >
            Volver al backlog
          </Link>
          <StoryDetailView storyId={storyId} />
        </div>
      </SessionGuard>
    </main>
  );
}
