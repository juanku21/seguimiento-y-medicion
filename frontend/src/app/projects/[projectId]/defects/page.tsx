import Link from "next/link";
import { PageHeader } from "@/components/page-header";
import { buttonClassName } from "@/components/button";
import type { Metadata } from "next";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { DefectsView } from "@/features/defects/components/defects-view";

// Título propio de la pantalla de defectos del proyecto.
export const metadata: Metadata = {
  title: "Defectos | Software Metrics & Estimation",
};

// Pantalla protegida de los defectos del proyecto (007/US3). Es un Server Component que solo toma el
// identificador del proyecto de la ruta y encuadra la vista: la guardia de sesión corre en el
// cliente, porque el token vive en sessionStorage, y muestra el regreso al proyecto, los filtros y
// el listado únicamente cuando hay una sesión vigente.
export default async function ProjectDefectsPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = await params;

  return (
    <main className="flex w-full max-w-6xl flex-1 flex-col gap-6">
      <PageHeader
        title="Defectos"
        action={
          <Link
            href={`/projects/${projectId}/defects/new`}
            className={buttonClassName("primary")}
          >
            Registrar defecto
          </Link>
        }
      />
      <SessionGuard>
        <div className="flex flex-col gap-6">
          <DefectsView projectId={projectId} />
        </div>
      </SessionGuard>
    </main>
  );
}
