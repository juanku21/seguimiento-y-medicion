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
    <main className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-6 px-6 py-16">
      <h1 className="text-2xl font-semibold tracking-tight">Iniciar sesión</h1>
      <Suspense>
        <LoginForm />
      </Suspense>
    </main>
  );
}
