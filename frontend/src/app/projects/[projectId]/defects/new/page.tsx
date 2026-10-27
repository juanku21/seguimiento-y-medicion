import type { Metadata } from "next";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { NewDefectView } from "@/features/defects/components/new-defect-view";

// Título propio de la pantalla de registro de defectos.
export const metadata: Metadata = {
  title: "Nuevo defecto | Software Metrics & Estimation",
};

// Pantalla protegida de registro de un defecto (007/US1). Es un Server Component que solo toma el
// identificador del proyecto de la ruta y encuadra el formulario: la guardia de sesión corre en el
// cliente, porque el token vive en sessionStorage, y muestra el formulario únicamente cuando hay una
// sesión vigente. Al crear el defecto navega a su detalle.
export default async function NewDefectPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Nuevo defecto</h1>
      <SessionGuard>
        <NewDefectView projectId={projectId} />
      </SessionGuard>
    </main>
  );
}
