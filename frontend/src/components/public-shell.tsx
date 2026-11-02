import type { ReactNode } from "react";
import { Brand } from "@/components/brand";
import { ThemeToggle } from "@/components/theme-toggle";

// Estructura de las pantallas públicas: marca y selector de tema sobre el contenido.
export function PublicShell({ children }: { children: ReactNode }) {
  return (
    <>
      <header className="mx-auto flex h-16 w-full max-w-6xl items-center justify-between px-6">
        <Brand href="/" />
        <ThemeToggle />
      </header>
      {children}
    </>
  );
}
