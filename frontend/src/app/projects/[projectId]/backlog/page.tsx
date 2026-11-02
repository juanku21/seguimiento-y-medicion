import { PageHeader } from "@/components/page-header";
import { buttonClassName } from "@/components/button";
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
    <main className="flex w-full max-w-5xl flex-1 flex-col gap-6">
      <PageHeader
        title="Backlog"
        action={
          <Link
            href={`/projects/${projectId}/backlog/new`}
            className={buttonClassName("primary")}
          >
            Nueva historia
          </Link>
        }
      />
      <SessionGuard>
        <BacklogView projectId={projectId} />
      </SessionGuard>
    </main>
  );
}
