import type { Metadata } from "next";
import Link from "next/link";
import { buttonClassName } from "@/components/button";
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
    <main className="flex w-full flex-1 flex-col gap-6">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <h1 className="text-2xl font-semibold tracking-tight">Mis proyectos</h1>
        <Link href="/projects/new" className={buttonClassName("primary")}>
          Nuevo proyecto
        </Link>
      </div>
      <SessionGuard>
        <ProjectList />
      </SessionGuard>
    </main>
  );
}
