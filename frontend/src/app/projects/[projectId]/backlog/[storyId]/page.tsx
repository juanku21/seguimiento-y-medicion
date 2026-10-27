import type { Metadata } from "next";
import Link from "next/link";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { StoryDefectsSection } from "@/features/defects/components/story-defects-section";
import { StoryDetailView } from "@/features/backlog/components/story-detail-view";
import { StoryEffortSection } from "@/features/effort/components/story-effort-section";
import { StoryEstimationSection } from "@/features/poker/components/story-estimation-section";
import { StorySprintHistory } from "@/features/sprints/components/story-sprint-history";

// Título propio de la pantalla de detalle de una historia.
export const metadata: Metadata = {
  title: "Historia | Software Metrics & Estimation",
};

// Pantalla protegida del detalle de una historia (003/US3). Es un Server Component que solo toma los
// identificadores de la ruta y encuadra la vista: la guardia de sesión corre en el cliente, porque
// el token vive en sessionStorage, y muestra el regreso al backlog, el detalle, la sección de
// Planning Poker para abrir o unirse a una sesión (005/US1), el registro de esfuerzo (006/US1), los defectos de la historia (007/US3) y los
// sprints en los que estuvo la historia (004/US6) únicamente cuando hay una sesión vigente.
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
          <StoryEstimationSection projectId={projectId} storyId={storyId} />
          <StoryEffortSection storyId={storyId} />
          <StoryDefectsSection projectId={projectId} storyId={storyId} />
          <StorySprintHistory storyId={storyId} />
        </div>
      </SessionGuard>
    </main>
  );
}
