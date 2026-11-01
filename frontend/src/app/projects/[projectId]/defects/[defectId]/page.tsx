import type { Metadata } from "next";
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
  const { defectId } = await params;

  return (
    <main className="flex w-full max-w-5xl flex-1 flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">Defecto</h1>
      <SessionGuard>
        <div className="flex flex-col gap-6">
          <DefectDetail defectId={defectId} />
        </div>
      </SessionGuard>
    </main>
  );
}
