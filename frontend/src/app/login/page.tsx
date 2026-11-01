import { PublicShell } from "@/components/public-shell";
import type { Metadata } from "next";
import { Suspense } from "react";
import { LoginForm } from "@/features/auth/components/login-form";

// Título propio de la pantalla de inicio de sesión.
export const metadata: Metadata = {
  title: "Iniciar sesión | Software Metrics & Estimation",
};

// Pantalla de inicio de sesión (US2). Es un Server Component que encuadra el formulario, que corre
// en el cliente. El formulario lee la URL con useSearchParams para el aviso de sesión vencida, así
// que va dentro de Suspense para que el resto de la página se siga prerenderizando.
export default function LoginPage() {
  return (
    <PublicShell>
      <main className="mx-auto mb-8 flex w-[calc(100%-2rem)] max-w-md flex-col gap-6 rounded-lg border border-border bg-surface p-5 sm:my-auto sm:p-8">
        <h1 className="text-2xl font-semibold tracking-tight">
          Iniciar sesión
        </h1>
        <Suspense>
          <LoginForm />
        </Suspense>
      </main>
    </PublicShell>
  );
}
