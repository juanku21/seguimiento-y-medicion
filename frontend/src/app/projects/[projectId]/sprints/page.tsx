import { PageHeader } from "@/components/page-header";
import { buttonClassName } from "@/components/button";
import type { Metadata } from "next";
import Link from "next/link";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { SprintHistoryTable } from "@/features/sprints/components/sprint-history-table";
import { SprintList } from "@/features/sprints/components/sprint-list";

// Título propio de la pantalla de sprints.
export const metadata: Metadata = {
  title: "Sprints | Software Metrics & Estimation",
};

const TAB_CLASS_NAME =
  "rounded-md px-3 py-1.5 text-sm font-medium underline-offset-4 aria-[current=page]:bg-primary aria-[current=page]:text-on-primary";

// Pantalla protegida de los sprints del proyecto (004/US6). Es un Server Component que toma el
// identificador del proyecto de la ruta y la vista elegida de la consulta (?view=closed para el
// historial de cerrados, el listado completo en cualquier otro caso) y encuadra la vista: la guardia
// de sesión corre en el cliente, porque el token vive en sessionStorage, y muestra el acceso a
// "Nuevo sprint", las pestañas y la vista únicamente cuando hay una sesión vigente.
export default async function SprintsPage({
  params,
  searchParams,
}: {
  params: Promise<{ projectId: string }>;
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>;
}) {
  const { projectId } = await params;
  const { view } = await searchParams;
  const showClosed = view === "closed";
  const basePath = `/projects/${projectId}/sprints`;

  return (
    <main className="flex w-full max-w-6xl flex-1 flex-col gap-6">
      <PageHeader
        title="Sprints"
        action={
          <Link href={`${basePath}/new`} className={buttonClassName("primary")}>
            Nuevo sprint
          </Link>
        }
      />
      <SessionGuard>
        <div className="flex flex-col gap-6">
          {/* Pestañas entre el listado completo y el historial de cerrados. */}
          <nav aria-label="Vistas de sprints" className="flex gap-2">
            <Link
              href={basePath}
              aria-current={showClosed ? undefined : "page"}
              className={TAB_CLASS_NAME}
            >
              Todos
            </Link>
            <Link
              href={`${basePath}?view=closed`}
              aria-current={showClosed ? "page" : undefined}
              className={TAB_CLASS_NAME}
            >
              Cerrados
            </Link>
          </nav>

          {showClosed ? (
            <SprintHistoryTable projectId={projectId} />
          ) : (
            <SprintList projectId={projectId} />
          )}
        </div>
      </SessionGuard>
    </main>
  );
}
