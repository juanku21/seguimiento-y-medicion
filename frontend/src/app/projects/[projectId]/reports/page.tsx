import type { Metadata } from "next";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { ReportsView } from "@/features/reports/components/reports-view";

// Título propio de la pantalla de reportes.
export const metadata: Metadata = {
  title: "Reportes | Software Metrics & Estimation",
};

// Pantalla protegida de reportes del proyecto (010). Es un Server Component que solo toma el
// identificador de la ruta y encuadra la vista: la guardia de sesión corre en el cliente, porque el
// token vive en sessionStorage.
export default async function ReportsPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Reportes</h1>
      <SessionGuard>
        <ReportsView projectId={projectId} />
      </SessionGuard>
    </main>
  );
}
