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
    <main className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Crear cuenta</h1>
      <RegisterForm />
    </main>
  );
}
