import { PublicShell } from "@/components/public-shell";
import type { Metadata } from "next";
import { RegisterForm } from "@/features/auth/components/register-form";

// Título propio de la pantalla de registro.
export const metadata: Metadata = {
  title: "Crear cuenta | Software Metrics & Estimation",
};

// Pantalla de registro (US1). Es un Server Component que encuadra el formulario, que corre en el
// cliente.
export default function RegisterPage() {
  return (
    <PublicShell>
      <main className="mx-auto mb-8 flex w-[calc(100%-2rem)] max-w-md flex-col gap-6 rounded-lg border border-border bg-surface p-5 sm:my-auto sm:p-8">
        <h1 className="text-2xl font-semibold tracking-tight">Crear cuenta</h1>
        <RegisterForm />
      </main>
    </PublicShell>
  );
}
