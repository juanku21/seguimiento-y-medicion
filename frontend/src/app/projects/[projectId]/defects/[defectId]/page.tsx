import type { Metadata } from "next";
import Link from "next/link";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { DefectDetail } from "@/features/defects/components/defect-detail";

// Título propio de la pantalla de detalle de un defecto.
export const metadata: Metadata = {
  title: "Defecto | Software Metrics & Estimation",
};

// Pantalla protegida del detalle de un defecto (007/US2). Es un Server Component que solo toma los
// identificadores de la ruta y encuadra la vista: la guardia de sesión corre en el cliente, porque
// el token vive en sessionStorage, y muestra el regreso al proyecto y el detalle únicamente cuando
// hay una sesión vigente.
export default async function DefectPage({
  params,
}: {
  params: Promise<{ projectId: string; defectId: string }>;
}) {
  const { projectId, defectId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Defecto</h1>
      <SessionGuard>
        <div className="flex flex-col gap-6">
          <Link
            href={`/projects/${projectId}`}
            className="self-start text-sm font-medium underline underline-offset-4"
          >
            Volver al proyecto
          </Link>
          <DefectDetail defectId={defectId} />
        </div>
      </SessionGuard>
    </main>
  );
}
