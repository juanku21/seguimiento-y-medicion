import type { Metadata } from "next";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { DashboardView } from "@/features/dashboard/components/dashboard-view";

// Título propio de la pantalla del dashboard.
export const metadata: Metadata = {
  title: "Dashboard | Software Metrics & Estimation",
};

// Pantalla protegida del dashboard del proyecto (009). Es un Server Component que solo toma el
// identificador de la ruta y encuadra la vista: la guardia de sesión corre en el cliente, porque el
// token vive en sessionStorage. El ancho es mayor que el de otras pantallas para alojar los gráficos.
export default async function DashboardPage({
  params,
}: {
  params: Promise<{ projectId: string }>;
}) {
  const { projectId } = await params;

  return (
    <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Dashboard</h1>
      <SessionGuard>
        <DashboardView projectId={projectId} />
      </SessionGuard>
    </main>
  );
}
