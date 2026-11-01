import type { Metadata } from "next";
import { SessionGuard } from "@/features/auth/components/session-guard";
import { ProjectForm } from "@/features/projects/components/project-form";

// Título propio de la pantalla de creación de proyecto.
export const metadata: Metadata = {
  title: "Nuevo proyecto | Software Metrics & Estimation",
};

// Pantalla protegida de creación de proyecto (002/US1). Es un Server Component que solo encuadra
// el formulario: la guardia de sesión corre en el cliente, porque el token vive en sessionStorage,
// y muestra el formulario únicamente cuando hay una sesión vigente.
export default function NewProjectPage() {
  return (
    <main className="flex w-full max-w-2xl flex-1 flex-col gap-6">
      <h1 className="text-2xl font-semibold tracking-tight">Nuevo proyecto</h1>
      <SessionGuard>
        <ProjectForm />
      </SessionGuard>
    </main>
  );
}
