import Link from "next/link";
import { buttonClassName } from "@/components/button";
import { PublicShell } from "@/components/public-shell";

// Pantalla de ruta inexistente: explica qué pasó y ofrece volver a los proyectos.
export default function NotFound() {
  return (
    <PublicShell>
      <main className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-4 px-6 py-12">
        <h1 className="text-2xl font-semibold tracking-tight">
          No encontramos esta página
        </h1>
        <p className="text-foreground-muted">
          La dirección no existe o ya no está disponible.
        </p>
        <Link href="/projects" className={buttonClassName("primary")}>
          Ir a mis proyectos
        </Link>
      </main>
    </PublicShell>
  );
}
