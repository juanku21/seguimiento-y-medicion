import type { Metadata } from "next";
import Link from "next/link";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { BacklogView } from "@/features/backlog/components/backlog-view";

// Título propio de la pantalla del backlog.
export const metadata: Metadata = {
  title: "Backlog | Software Metrics & Estimation",
};

// Pantalla protegida del backlog del proyecto (003/US2). Es un Server Component que solo toma el
// identificador del proyecto de la ruta y encuadra la vista: la guardia de sesión corre en el
// cliente, porque el token vive en sessionStorage, y muestra el acceso a "Nueva historia" y el
// listado filtrable únicamente cuando hay una sesión vigente.
export default async function BacklogPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Backlog</h1>
      <SessionGuard>
        <div className="flex flex-col gap-6">
          <Link
            href={`/projects/${projectId}/backlog/new`}
            className="self-start text-sm font-medium underline underline-offset-4"
          >
            Nueva historia
          </Link>
          <BacklogView projectId={projectId} />
        </div>
      </SessionGuard>
    </main>
  );
}
