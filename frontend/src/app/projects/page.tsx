import type { Metadata } from "next";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { ProjectList } from "@/features/projects/components/project-list";

// Título propio de la pantalla del listado de proyectos.
export const metadata: Metadata = {
  title: "Mis proyectos | Software Metrics & Estimation",
};

// Pantalla protegida del listado de proyectos (002/US2). Es un Server Component que solo encuadra
// el listado: la guardia de sesión corre en el cliente, porque el token vive en sessionStorage, y
// muestra el listado únicamente cuando hay una sesión vigente, igual que la pantalla de creación.
export default function ProjectsPage() {
  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Mis proyectos</h1>
      <SessionGuard>
        <ProjectList />
      </SessionGuard>
    </main>
  );
}
