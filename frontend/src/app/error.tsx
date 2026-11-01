"use client";

import { Button } from "@/components/button";

// Pantalla de error inesperado: evita una página en blanco y permite reintentar.
export default function ErrorPage({ reset }: { reset: () => void }) {
  return (
    <main className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center gap-4 px-6 py-12">
      <h1 className="text-2xl font-semibold tracking-tight">
        No se pudo mostrar esta pantalla
      </h1>
      <p className="text-foreground-muted">
        Ocurrió un error inesperado. Probá de nuevo; tus datos guardados no se
        modificaron.
      </p>
      <Button onClick={reset}>Reintentar</Button>
    </main>
  );
}
